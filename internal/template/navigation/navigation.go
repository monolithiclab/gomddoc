package navigation

import (
	"bufio"
	"io/fs"
	"path"
	"slices"
	"strings"

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
type Generator struct {
	rootFS       fs.FS
	defaultIndex string
}

// NewGenerator creates a new navigation generator.
func NewGenerator(rootFS fs.FS, defaultIndex string) *Generator {
	return &Generator{
		rootFS:       rootFS,
		defaultIndex: defaultIndex,
	}
}

// Generate builds a navigation tree with the given path marked as active.
// It walks the rootFS to discover all renderable markdown files and
// organizes them into a hierarchical tree structure.
func (g *Generator) Generate(currentPath string) *NavNode {
	root := &NavNode{
		Label: "Root",
		Path:  "/",
		IsDir: true,
	}

	g.buildTree(root, ".")
	g.markActive(root, cleanPath(currentPath))

	// If root has no children, return nil to signal no navigation
	if len(root.Children) == 0 {
		return nil
	}

	return root
}

// buildTree recursively walks the filesystem and populates the tree.
func (g *Generator) buildTree(parent *NavNode, dir string) {
	entries, err := fs.ReadDir(g.rootFS, dir)
	if err != nil {
		return
	}

	// Separate directories and files, filtering as we go
	var dirs []fs.DirEntry
	var files []fs.DirEntry

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else if strings.HasSuffix(name, ".md") && name != g.defaultIndex {
			files = append(files, entry)
		}
	}

	// Sort directories first, then files, both alphabetically
	slices.SortFunc(dirs, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	slices.SortFunc(files, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})

	// Add directories
	for _, d := range dirs {
		name := d.Name()
		entryPath := joinPath(dir, name)
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
	}

	// Add files
	for _, f := range files {
		name := f.Name()
		entryPath := joinPath(dir, name)
		urlPath := "/" + entryPath

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
