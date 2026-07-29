package navigation

import (
	"io/fs"
	"mime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/resolve"
)

func init() {
	_ = mime.AddExtensionType(".md", "text/markdown")
}

// fnFS wraps an fstest.MapFS to record which files are opened, so tests can
// assert the title lookup avoids opening files.
type fnFS struct {
	fstest.MapFS
	onOpen func(string)
}

func (f fnFS) Open(name string) (fs.File, error) {
	if f.onOpen != nil {
		f.onOpen(name)
	}
	return f.MapFS.Open(name)
}

func TestTree_BasicShape(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"README.md":       {Data: []byte("# Home")},
		"guide.md":        {Data: []byte("# Getting Started")},
		"reference.md":    {Data: []byte("# API Reference")},
		"docs/install.md": {Data: []byte("# Installation")},
		"docs/usage.md":   {Data: []byte("# Usage Guide")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	// Should have: docs/ dir, guide.md, reference.md (README.md excluded)
	if len(root.Children) != 3 {
		t.Fatalf("expected 3 root children, got %d", len(root.Children))
	}

	// Sorted alphabetically: docs/, guide.md, reference.md
	if root.Children[0].Label != "Docs" || !root.Children[0].IsDir {
		t.Errorf("first child should be Docs dir, got %q (isDir=%v)", root.Children[0].Label, root.Children[0].IsDir)
	}
	if root.Children[1].Label != "Getting Started" {
		t.Errorf("second child label should be 'Getting Started', got %q", root.Children[1].Label)
	}
	if root.Children[2].Label != "API Reference" {
		t.Errorf("third child label should be 'API Reference', got %q", root.Children[2].Label)
	}
}

// TestTree_TitleLookup verifies leaf labels come from the indexed title lookup
// (no file open) when available, and fall back to the file's heading otherwise.
// Regression test for REVIEW.md §9.8.
func TestTree_TitleLookup(t *testing.T) {
	t.Parallel()

	opened := map[string]bool{}
	fsys := fnFS{
		MapFS: fstest.MapFS{
			"indexed.md":  {Data: []byte("# Heading Title")},
			"fallback.md": {Data: []byte("# Scanned Heading")},
		},
		onOpen: func(name string) { opened[name] = true },
	}

	gen := NewGenerator(fsys, "README.md", nil, nil)
	gen.SetTitleLookup(func(filePath string) string {
		if filePath == "indexed.md" {
			return "Indexed Title"
		}
		return "" // fallback.md has no indexed title
	})
	root := gen.Tree()

	labels := map[string]string{}
	for _, c := range root.Children {
		labels[c.Path] = c.Label
	}
	if labels["/indexed"] != "Indexed Title" && labels["/indexed.md"] != "Indexed Title" {
		t.Errorf("indexed page label = %v, want %q (from lookup)", labels, "Indexed Title")
	}
	// The indexed page must NOT have been opened (the lookup short-circuits).
	if opened["indexed.md"] {
		t.Error("indexed.md was opened despite an indexed title being available")
	}
	// The page without an indexed title falls back to a file scan of its heading.
	if labels["/fallback"] != "Scanned Heading" && labels["/fallback.md"] != "Scanned Heading" {
		t.Errorf("fallback page label = %v, want %q (from file scan)", labels, "Scanned Heading")
	}
	if !opened["fallback.md"] {
		t.Error("fallback.md should have been opened for its heading")
	}
}

func TestTree_CachedAcrossCalls(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"guide.md": {Data: []byte("# Guide")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	first := gen.Tree()
	second := gen.Tree()

	if first != second {
		t.Errorf("Tree() should return the same cached pointer; got %p then %p", first, second)
	}
}

func TestTree_HiddenFilesSkipped(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"visible.md":         {Data: []byte("# Visible")},
		".hidden.md":         {Data: []byte("# Hidden")},
		".gomddoc/config.md": {Data: []byte("# Config")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (hidden files/dirs skipped), got %d", len(root.Children))
	}

	if root.Children[0].Label != "Visible" {
		t.Errorf("expected 'Visible', got %q", root.Children[0].Label)
	}
}

func TestTree_DefaultIndexSkipped(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"README.md":      {Data: []byte("# Home")},
		"guide.md":       {Data: []byte("# Guide")},
		"docs/README.md": {Data: []byte("# Docs Index")},
		"docs/setup.md":  {Data: []byte("# Setup")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	for _, child := range root.Children {
		if strings.Contains(child.Path, "README") {
			t.Errorf("README.md should be excluded, found %q", child.Path)
		}
		for _, grandchild := range child.Children {
			if strings.Contains(grandchild.Path, "README") {
				t.Errorf("README.md should be excluded in subdirs, found %q", grandchild.Path)
			}
		}
	}
}

func TestTree_EmptyDirsExcluded(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"guide.md":        {Data: []byte("# Guide")},
		"empty/README.md": {Data: []byte("# Empty")}, // Only has default index
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (empty dir excluded), got %d", len(root.Children))
	}
}

func TestTree_NonMDFilesSkipped(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"guide.md":    {Data: []byte("# Guide")},
		"image.png":   {Data: []byte("PNG")},
		"style.css":   {Data: []byte("body{}")},
		"data.json":   {Data: []byte("{}")},
		"other.txt":   {Data: []byte("text")},
		"script.html": {Data: []byte("<html>")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (only .md files), got %d", len(root.Children))
	}
}

func TestTree_TitleExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple heading",
			content:  "# My Title\n\nContent here",
			expected: "My Title",
		},
		{
			name:     "heading after front matter",
			content:  "---\ntitle: FM Title\n---\n# Heading Title\n\nContent",
			expected: "Heading Title",
		},
		{
			name:     "no heading falls back to filename",
			content:  "Just some content\nwithout headings",
			expected: "Test File",
		},
		{
			name:     "heading with extra spaces",
			content:  "#   Spaced Title  \n\nContent",
			expected: "Spaced Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := fstest.MapFS{
				"test-file.md": {Data: []byte(tt.content)},
			}

			gen := NewGenerator(fs, "README.md", nil, nil)
			root := gen.Tree()

			if root == nil {
				t.Fatal("expected non-nil root")
			}

			if len(root.Children) != 1 {
				t.Fatalf("expected 1 child, got %d", len(root.Children))
			}

			if root.Children[0].Label != tt.expected {
				t.Errorf("expected label %q, got %q", tt.expected, root.Children[0].Label)
			}
		})
	}
}

func TestTree_SortOrder(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"zebra.md":     {Data: []byte("# Zebra")},
		"alpha.md":     {Data: []byte("# Alpha")},
		"beta/one.md":  {Data: []byte("# One")},
		"alpha/two.md": {Data: []byte("# Two")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	// Sorted alphabetically: alpha/ (dir), alpha.md (file), beta/ (dir), zebra.md (file)
	expected := []struct {
		label string
		isDir bool
	}{
		{"Alpha", true},
		{"Alpha", false},
		{"Beta", true},
		{"Zebra", false},
	}
	if len(root.Children) != 4 {
		t.Fatalf("expected 4 children, got %d", len(root.Children))
	}

	for i, exp := range expected {
		if root.Children[i].Label != exp.label {
			t.Errorf("child[%d] label = %q, want %q", i, root.Children[i].Label, exp.label)
		}
		if root.Children[i].IsDir != exp.isDir {
			t.Errorf("child[%d] isDir = %v, want %v", i, root.Children[i].IsDir, exp.isDir)
		}
	}
}

func TestTree_EmptyFS(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root != nil {
		t.Error("expected nil root for empty filesystem")
	}
}

func TestTree_OnlyDefaultIndex(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"README.md": {Data: []byte("# Home")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root != nil {
		t.Error("expected nil root when only default index exists")
	}
}

func TestNormalizeRequestPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"/", ""},
		{"/guide.md", "guide.md"},
		{"/docs/install.md", "docs/install.md"},
		{"docs/install.md", "docs/install.md"},
		{"/docs/", "docs"},
		{".", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := NormalizeRequestPath(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeRequestPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTitle_FrontMatter(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"page.md": {Data: []byte("---\ntitle: FM Title\nauthor: Test\n---\n\nSome intro text.\n\n# Real Heading\n\nContent.")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	if root.Children[0].Label != "Real Heading" {
		t.Errorf("expected 'Real Heading', got %q", root.Children[0].Label)
	}
}

func TestTree_DeepNesting(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"a/b/c/deep.md": {Data: []byte("# Deep Page")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	root := gen.Tree()

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	// Navigate: root -> a -> b -> c -> deep.md
	aNode := root.Children[0]
	if aNode.Label != "A" || !aNode.IsDir {
		t.Errorf("expected dir A, got label=%q isDir=%v", aNode.Label, aNode.IsDir)
	}

	bNode := aNode.Children[0]
	if bNode.Label != "B" || !bNode.IsDir {
		t.Errorf("expected dir B, got label=%q isDir=%v", bNode.Label, bNode.IsDir)
	}

	cNode := bNode.Children[0]
	if cNode.Label != "C" || !cNode.IsDir {
		t.Errorf("expected dir C, got label=%q isDir=%v", cNode.Label, cNode.IsDir)
	}

	deepNode := cNode.Children[0]
	if deepNode.Label != "Deep Page" || deepNode.IsDir {
		t.Errorf("expected leaf 'Deep Page', got label=%q isDir=%v", deepNode.Label, deepNode.IsDir)
	}
}

func TestFindFirstPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root *NavNode
		want string
	}{
		{
			name: "nil root",
			root: nil,
			want: "",
		},
		{
			name: "empty children",
			root: &NavNode{Children: nil},
			want: "",
		},
		{
			name: "single file",
			root: &NavNode{
				Children: []*NavNode{
					{Label: "Guide", Path: "/guide.md"},
				},
			},
			want: "/guide.md",
		},
		{
			name: "dir then file",
			root: &NavNode{
				Children: []*NavNode{
					{Label: "Docs", Path: "/docs/", IsDir: true, Children: []*NavNode{
						{Label: "Setup", Path: "/docs/setup.md"},
					}},
					{Label: "README", Path: "/readme.md"},
				},
			},
			want: "/docs/setup.md",
		},
		{
			name: "only empty dirs",
			root: &NavNode{
				Children: []*NavNode{
					{Label: "Empty", Path: "/empty/", IsDir: true},
				},
			},
			want: "",
		},
		{
			name: "nested dirs to file",
			root: &NavNode{
				Children: []*NavNode{
					{Label: "A", Path: "/a/", IsDir: true, Children: []*NavNode{
						{Label: "B", Path: "/a/b/", IsDir: true, Children: []*NavNode{
							{Label: "Deep", Path: "/a/b/deep.md"},
						}},
					}},
				},
			},
			want: "/a/b/deep.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FindFirstPage(tt.root)
			if got != tt.want {
				t.Errorf("FindFirstPage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTree_CleanPaths(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": &fstest.MapFile{Data: []byte("# Guide")},
		"about.md": &fstest.MapFile{Data: []byte("# About")},
	}

	resolver := resolve.Build(fsys, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: func(mime string) bool {
		return mime == "text/markdown"
	}})

	gen := NewGenerator(fsys, "README.md", nil, resolver)
	root := gen.Tree()

	for _, child := range root.Children {
		if child.Label == "Guide" {
			if child.Path != "/guide" {
				t.Errorf("guide Path = %q, want /guide", child.Path)
			}
		}
		if child.Label == "About" {
			if child.Path != "/about" {
				t.Errorf("about Path = %q, want /about", child.Path)
			}
		}
	}
}

func TestTree_NilResolver(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": &fstest.MapFile{Data: []byte("# Guide")},
	}
	gen := NewGenerator(fsys, "README.md", nil, nil)
	root := gen.Tree()
	for _, child := range root.Children {
		if child.Label == "Guide" {
			if child.Path != "/guide.md" {
				t.Errorf("Path = %q, want /guide.md (no resolver)", child.Path)
			}
		}
	}
}

func TestPrevNext(t *testing.T) {
	t.Parallel()

	// Files/dirs are sorted alphabetically and interleaved at each level,
	// so depth-first leaf order is: faq.md, guide/setup.md, guide/usage.md, intro.md.
	fs := fstest.MapFS{
		"intro.md":       {Data: []byte("# Intro")},
		"guide/setup.md": {Data: []byte("# Setup")},
		"guide/usage.md": {Data: []byte("# Usage")},
		"faq.md":         {Data: []byte("# FAQ")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)

	tests := []struct {
		name      string
		current   string
		wantPrev  string
		wantNext  string
		wantNoPrv bool
		wantNoNxt bool
	}{
		{
			name:     "middle page",
			current:  "/guide/setup.md",
			wantPrev: "/faq.md",
			wantNext: "/guide/usage.md",
		},
		{
			name:      "first page",
			current:   "/faq.md",
			wantNoPrv: true,
			wantNext:  "/guide/setup.md",
		},
		{
			name:      "last page",
			current:   "/intro.md",
			wantPrev:  "/guide/usage.md",
			wantNoNxt: true,
		},
		{
			name:      "not in tree",
			current:   "/missing.md",
			wantNoPrv: true,
			wantNoNxt: true,
		},
		{
			name:     "with leading-slash variation",
			current:  "guide/setup.md",
			wantPrev: "/faq.md",
			wantNext: "/guide/usage.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prev, next := gen.PrevNext(tt.current)

			if tt.wantNoPrv {
				if prev != nil {
					t.Errorf("prev = %+v, want nil", prev)
				}
			} else {
				if prev == nil {
					t.Fatalf("prev = nil, want %q", tt.wantPrev)
				}
				if prev.Path != tt.wantPrev {
					t.Errorf("prev.Path = %q, want %q", prev.Path, tt.wantPrev)
				}
				if prev.Label == "" {
					t.Errorf("prev.Label is empty")
				}
			}

			if tt.wantNoNxt {
				if next != nil {
					t.Errorf("next = %+v, want nil", next)
				}
			} else {
				if next == nil {
					t.Fatalf("next = nil, want %q", tt.wantNext)
				}
				if next.Path != tt.wantNext {
					t.Errorf("next.Path = %q, want %q", next.Path, tt.wantNext)
				}
				if next.Label == "" {
					t.Errorf("next.Label is empty")
				}
			}
		})
	}
}

func TestPrevNext_SinglePage(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"only.md": {Data: []byte("# Only")},
	}

	gen := NewGenerator(fs, "README.md", nil, nil)
	prev, next := gen.PrevNext("/only.md")

	if prev != nil || next != nil {
		t.Errorf("PrevNext on single page: prev=%+v next=%+v, want both nil", prev, next)
	}
}

func TestPrevNext_EmptyTree(t *testing.T) {
	t.Parallel()

	gen := NewGenerator(fstest.MapFS{}, "README.md", nil, nil)
	prev, next := gen.PrevNext("/anything.md")

	if prev != nil || next != nil {
		t.Errorf("PrevNext on empty tree: prev=%+v next=%+v, want both nil", prev, next)
	}
}
