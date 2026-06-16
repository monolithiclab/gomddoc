package navigation

import (
	"bufio"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// NavNode represents a node in the navigation tree. The cached tree returned
// by Generator.Tree is immutable — callers must not mutate any node.
type NavNode struct {
	Label    string     // Display label for this node
	Path     string     // URL path for this node
	IsDir    bool       // Whether this node represents a directory
	Children []*NavNode // Child nodes

	// cleanPath is the normalized form of Path used for active-path comparison
	// (no leading/trailing slash, lexically cleaned). Pre-computed at cache build
	// time so per-request walks don't reallocate.
	cleanPath string
}

// CompareClean reports whether the node's path matches a pre-cleaned request
// path. Generator-built nodes use the pre-computed clean path; manually-built
// nodes (in tests) fall back to normalizing on the fly.
func (n *NavNode) CompareClean(cleanRequestPath string) bool {
	cp := n.cleanPath
	if cp == "" && n.Path != "" {
		cp = NormalizeRequestPath(n.Path)
	}
	return cp == cleanRequestPath
}

// PageEntry is a leaf page in the navigation tree.
type PageEntry struct {
	Path  string // URL path (e.g. "/guide/setup")
	Label string // Display label
}

// Generator builds and caches a navigation tree from an fs.FS.
type Generator struct {
	rootFS          fs.FS
	defaultIndex    string
	excludePatterns []string
	resolver        *resolve.PathResolver

	// titleLookup, when set, returns a page's title for an fs-relative path
	// (e.g. "guide/setup.md") from the metadata index, avoiding a file open.
	// It returns "" when the page has no indexed title, in which case the tree
	// builder falls back to scanning the file for its first heading.
	titleLookup func(filePath string) string

	cacheOnce   sync.Once
	cachedTree  *NavNode
	cachedPages []PageEntry
	cachedIndex map[string]int // cleanPath → index in cachedPages
}

// SetTitleLookup installs a title lookup (backed by the metadata index) used to
// label leaf pages without opening each file. Must be called before the tree is
// first built (i.e. during setup, before serving requests).
func (g *Generator) SetTitleLookup(fn func(filePath string) string) {
	g.titleLookup = fn
}

// NewGenerator creates a new navigation generator.
func NewGenerator(rootFS fs.FS, defaultIndex string, excludePatterns []string, resolver *resolve.PathResolver) *Generator {
	return &Generator{
		rootFS:          rootFS,
		defaultIndex:    defaultIndex,
		excludePatterns: excludePatterns,
		resolver:        resolver,
	}
}

// Tree returns the cached navigation tree, building it on first call.
// Returns nil when no renderable pages exist.
func (g *Generator) Tree() *NavNode {
	g.ensureCache()
	return g.cachedTree
}

// PrevNext returns the previous and next leaf pages relative to currentPath
// in depth-first navigation order. Either may be nil at the boundaries or
// when currentPath is not a leaf in the tree. Lookup is O(1).
func (g *Generator) PrevNext(currentPath string) (prev, next *PageEntry) {
	g.ensureCache()
	if g.cachedIndex == nil {
		return nil, nil
	}
	idx, ok := g.cachedIndex[NormalizeRequestPath(currentPath)]
	if !ok {
		return nil, nil
	}
	if idx > 0 {
		prev = &g.cachedPages[idx-1]
	}
	if idx+1 < len(g.cachedPages) {
		next = &g.cachedPages[idx+1]
	}
	return prev, next
}

func (g *Generator) ensureCache() {
	g.cacheOnce.Do(func() {
		root := &NavNode{Label: "Root", Path: "/", IsDir: true}
		g.buildTree(root, ".")
		if len(root.Children) == 0 {
			return
		}
		g.cachedTree = root
		g.cachedPages = root.appendLeaves(nil)
		g.cachedIndex = make(map[string]int, len(g.cachedPages))
		for i, p := range g.cachedPages {
			g.cachedIndex[NormalizeRequestPath(p.Path)] = i
		}
	})
}

func (n *NavNode) appendLeaves(out []PageEntry) []PageEntry {
	for _, child := range n.Children {
		if child.IsDir {
			out = child.appendLeaves(out)
		} else {
			out = append(out, PageEntry{Path: child.Path, Label: child.Label})
		}
	}
	return out
}

// buildTree recursively walks the filesystem and populates the tree.
func (g *Generator) buildTree(parent *NavNode, dir string) {
	entries, err := fs.ReadDir(g.rootFS, dir)
	if err != nil {
		return
	}

	// Filter to visible entries (skip hidden, excluded, non-markdown, default index)
	var visible []fs.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		entryPath := path.Join(dir, name)
		if provider.IsRestrictedPath(entryPath, g.excludePatterns) {
			continue
		}
		if entry.IsDir() || (strings.HasSuffix(name, ".md") && name != g.defaultIndex) {
			visible = append(visible, entry)
		}
	}

	// Sort all entries alphabetically — directories and files interleaved
	slices.SortFunc(visible, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	for _, entry := range visible {
		name := entry.Name()
		entryPath := joinPath(dir, name)

		if entry.IsDir() {
			urlPath := "/" + entryPath + "/"
			node := &NavNode{
				Label:     text.TitleCase(name),
				Path:      urlPath,
				IsDir:     true,
				cleanPath: NormalizeRequestPath(urlPath),
			}
			g.buildTree(node, entryPath)
			// Only include directories with renderable children
			if len(node.Children) > 0 {
				parent.Children = append(parent.Children, node)
			}
		} else {
			urlPath := "/" + entryPath
			if g.resolver != nil {
				if clean, found := g.resolver.CleanPath(entryPath); found {
					urlPath = "/" + clean
				}
			}
			// Prefer the indexed title (no file open); fall back to scanning the
			// file's first heading, then to a title-cased filename.
			var label string
			if g.titleLookup != nil {
				label = g.titleLookup(entryPath)
			}
			if label == "" {
				label = g.extractTitle(entryPath)
			}
			if label == "" {
				// Fall back to title-cased filename without extension,
				// replacing hyphens and underscores with spaces
				base := strings.TrimSuffix(name, path.Ext(name))
				base = strings.ReplaceAll(base, "-", " ")
				base = strings.ReplaceAll(base, "_", " ")
				label = text.TitleCase(base)
			}
			parent.Children = append(parent.Children, &NavNode{
				Label:     label,
				Path:      urlPath,
				IsDir:     false,
				cleanPath: NormalizeRequestPath(urlPath),
			})
		}
	}
}

// extractTitle reads the first # heading from a markdown file, skipping
// any YAML front matter (delimited by ---).
func (g *Generator) extractTitle(filePath string) string {
	f, err := g.rootFS.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFrontMatter := false
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()

		// Handle front matter
		if firstLine && strings.TrimSpace(line) == "---" {
			inFrontMatter = true
			firstLine = false
			continue
		}
		firstLine = false

		if inFrontMatter {
			if strings.TrimSpace(line) == "---" {
				inFrontMatter = false
			}
			continue
		}

		// Look for the first # heading
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "# "); ok {
			return strings.TrimSpace(after)
		}
	}

	return ""
}

// FindFirstPage walks the navigation tree depth-first and returns the path
// of the first non-directory (leaf) node. Returns "" if no page is found.
func FindFirstPage(root *NavNode) string {
	if root == nil {
		return ""
	}
	for _, child := range root.Children {
		if !child.IsDir {
			return child.Path
		}
		if p := FindFirstPage(child); p != "" {
			return p
		}
	}
	return ""
}

// NormalizeRequestPath canonicalizes an HTTP request path for comparison
// against navigation node paths (use NavNode.CompareClean to perform the
// match). Strips leading/trailing slashes and lexically cleans the path.
func NormalizeRequestPath(p string) string {
	p = path.Clean(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "." || p == "" {
		return ""
	}
	return p
}

// joinPath joins directory and filename, handling the root "." case.
func joinPath(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}
