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

// NavNode represents a node in the navigation tree.
type NavNode struct {
	Label    string     // Display label for this node
	Path     string     // URL path for this node
	IsDir    bool       // Whether this node represents a directory
	IsActive bool       // Whether this node is the current page
	IsOpen   bool       // Whether this node or a descendant is active
	Children []*NavNode // Child nodes
}

// Generator builds navigation trees from an fs.FS.
// The base tree (without active/open markings) is built once on first call
// and cached. Each Generate() call clones the cached tree and applies
// active-path marking to the clone.
type Generator struct {
	rootFS          fs.FS
	defaultIndex    string
	excludePatterns []string
	resolver        *resolve.PathResolver
	cachedTree      *NavNode
	cacheOnce       sync.Once
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

// Generate builds a navigation tree with the given path marked as active.
// The base tree is built once (on first call) by walking rootFS, then cached.
// Subsequent calls clone the cached tree and apply active-path marking.
func (g *Generator) Generate(currentPath string) *NavNode {
	g.cacheOnce.Do(func() {
		root := &NavNode{
			Label: "Root",
			Path:  "/",
			IsDir: true,
		}
		g.buildTree(root, ".")
		if len(root.Children) > 0 {
			g.cachedTree = root
		}
	})

	if g.cachedTree == nil {
		return nil
	}

	tree := cloneTree(g.cachedTree)
	g.markActive(tree, cleanPath(currentPath))
	return tree
}

// cloneTree creates a deep copy of a NavNode tree.
// IsActive and IsOpen are reset to false in the clone.
func cloneTree(node *NavNode) *NavNode {
	clone := &NavNode{
		Label: node.Label,
		Path:  node.Path,
		IsDir: node.IsDir,
	}
	if len(node.Children) > 0 {
		clone.Children = make([]*NavNode, len(node.Children))
		for i, child := range node.Children {
			clone.Children[i] = cloneTree(child)
		}
	}
	return clone
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
				Label: text.TitleCase(name),
				Path:  urlPath,
				IsDir: true,
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
			label := g.extractTitle(entryPath)
			if label == "" {
				// Fall back to title-cased filename without extension,
				// replacing hyphens and underscores with spaces
				base := strings.TrimSuffix(name, path.Ext(name))
				base = strings.ReplaceAll(base, "-", " ")
				base = strings.ReplaceAll(base, "_", " ")
				label = text.TitleCase(base)
			}
			node := &NavNode{
				Label: label,
				Path:  urlPath,
				IsDir: false,
			}
			parent.Children = append(parent.Children, node)
		}
	}
}

// markActive marks the active node and opens all ancestor directories.
// Returns true if this node or any descendant is active.
func (g *Generator) markActive(node *NavNode, currentPath string) bool {
	if !node.IsDir && cleanPath(node.Path) == currentPath {
		node.IsActive = true
		node.IsOpen = true
		return true
	}

	for _, child := range node.Children {
		if g.markActive(child, currentPath) {
			node.IsOpen = true
			return true
		}
	}

	return false
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

// cleanPath normalizes a URL path for comparison by removing trailing slashes
// and leading slashes, then cleaning it.
func cleanPath(p string) string {
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
