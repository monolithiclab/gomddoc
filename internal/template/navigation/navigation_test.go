package navigation

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestGenerate_BasicTree(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"README.md":       {Data: []byte("# Home")},
		"guide.md":        {Data: []byte("# Getting Started")},
		"reference.md":    {Data: []byte("# API Reference")},
		"docs/install.md": {Data: []byte("# Installation")},
		"docs/usage.md":   {Data: []byte("# Usage Guide")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/guide.md")

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	// Should have: docs/ dir, guide.md, reference.md (README.md excluded)
	if len(root.Children) != 3 {
		t.Fatalf("expected 3 root children, got %d", len(root.Children))
	}

	// Directories come first
	if root.Children[0].Label != "Docs" || !root.Children[0].IsDir {
		t.Errorf("first child should be Docs dir, got %q (isDir=%v)", root.Children[0].Label, root.Children[0].IsDir)
	}

	// Then files alphabetically
	if root.Children[1].Label != "Getting Started" {
		t.Errorf("second child label should be 'Getting Started', got %q", root.Children[1].Label)
	}
	if root.Children[2].Label != "API Reference" {
		t.Errorf("third child label should be 'API Reference', got %q", root.Children[2].Label)
	}
}

func TestGenerate_ActiveMarking(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"guide.md":        {Data: []byte("# Guide")},
		"docs/install.md": {Data: []byte("# Install")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/docs/install.md")

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	// Root should be open (descendant is active)
	if !root.IsOpen {
		t.Error("root should be open when descendant is active")
	}

	// Find docs dir
	docsDir := root.Children[0]
	if !docsDir.IsOpen {
		t.Error("docs dir should be open when child is active")
	}

	// Find install.md
	installNode := docsDir.Children[0]
	if !installNode.IsActive {
		t.Error("install.md should be marked active")
	}
}

func TestGenerate_HiddenFilesSkipped(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"visible.md":         {Data: []byte("# Visible")},
		".hidden.md":         {Data: []byte("# Hidden")},
		".gomddoc/config.md": {Data: []byte("# Config")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

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

func TestGenerate_DefaultIndexSkipped(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"README.md":      {Data: []byte("# Home")},
		"guide.md":       {Data: []byte("# Guide")},
		"docs/README.md": {Data: []byte("# Docs Index")},
		"docs/setup.md":  {Data: []byte("# Setup")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	// Check that README.md files are excluded everywhere
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

func TestGenerate_EmptyDirsExcluded(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"guide.md":        {Data: []byte("# Guide")},
		"empty/README.md": {Data: []byte("# Empty")}, // Only has default index
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	// Should only have guide.md, not the empty dir
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (empty dir excluded), got %d", len(root.Children))
	}
}

func TestGenerate_NonMDFilesSkipped(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"guide.md":    {Data: []byte("# Guide")},
		"image.png":   {Data: []byte("PNG")},
		"style.css":   {Data: []byte("body{}")},
		"data.json":   {Data: []byte("{}")},
		"other.txt":   {Data: []byte("text")},
		"script.html": {Data: []byte("<html>")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root == nil {
		t.Fatal("expected non-nil root node")
	}

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (only .md files), got %d", len(root.Children))
	}
}

func TestGenerate_TitleExtraction(t *testing.T) {
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

			gen := NewGenerator(fs, "README.md")
			root := gen.Generate("/")

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

func TestGenerate_SortOrder(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"zebra.md":     {Data: []byte("# Zebra")},
		"alpha.md":     {Data: []byte("# Alpha")},
		"beta/one.md":  {Data: []byte("# One")},
		"alpha/two.md": {Data: []byte("# Two")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	// Dirs first (alpha, beta), then files (alpha.md, zebra.md)
	expected := []string{"Alpha", "Beta", "Alpha", "Zebra"}
	if len(root.Children) != 4 {
		t.Fatalf("expected 4 children, got %d", len(root.Children))
	}

	for i, exp := range expected {
		if root.Children[i].Label != exp {
			t.Errorf("child[%d] label = %q, want %q", i, root.Children[i].Label, exp)
		}
	}

	// First two should be dirs
	if !root.Children[0].IsDir || !root.Children[1].IsDir {
		t.Error("first two children should be directories")
	}
	if root.Children[2].IsDir || root.Children[3].IsDir {
		t.Error("last two children should be files")
	}
}

func TestGenerate_EmptyFS(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root != nil {
		t.Error("expected nil root for empty filesystem")
	}
}

func TestGenerate_OnlyDefaultIndex(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"README.md": {Data: []byte("# Home")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root != nil {
		t.Error("expected nil root when only default index exists")
	}
}

func TestRenderNavTree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		root     *NavNode
		contains []string
		excludes []string
	}{
		{
			name: "nil root",
			root: nil,
		},
		{
			name: "empty children",
			root: &NavNode{Children: nil},
		},
		{
			name: "simple file list",
			root: &NavNode{
				Children: []*NavNode{
					{Label: "Guide", Path: "/guide.md", IsActive: true},
					{Label: "Reference", Path: "/reference.md"},
				},
			},
			contains: []string{
				`<ul>`,
				`<a href="/guide.md" class="active">Guide</a>`,
				`<a href="/reference.md">Reference</a>`,
				`</ul>`,
			},
		},
		{
			name: "directory with details/summary",
			root: &NavNode{
				Children: []*NavNode{
					{
						Label:  "Docs",
						Path:   "/docs/",
						IsDir:  true,
						IsOpen: true,
						Children: []*NavNode{
							{Label: "Setup", Path: "/docs/setup.md"},
						},
					},
				},
			},
			contains: []string{
				`<details open>`,
				`<summary>Docs</summary>`,
				`<a href="/docs/setup.md">Setup</a>`,
				`</details>`,
			},
		},
		{
			name: "closed directory",
			root: &NavNode{
				Children: []*NavNode{
					{
						Label: "Other",
						Path:  "/other/",
						IsDir: true,
						Children: []*NavNode{
							{Label: "Page", Path: "/other/page.md"},
						},
					},
				},
			},
			contains: []string{`<details>`},
			excludes: []string{`<details open>`},
		},
		{
			name: "html escaping",
			root: &NavNode{
				Children: []*NavNode{
					{Label: "Q&A <Guide>", Path: "/q&a.md"},
				},
			},
			contains: []string{
				`Q&amp;A &lt;Guide&gt;`,
				`/q&amp;a.md`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := RenderNavTree(tt.root)

			if tt.root == nil || len(tt.root.Children) == 0 {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got:\n%s", want, result)
				}
			}

			for _, unwant := range tt.excludes {
				if strings.Contains(result, unwant) {
					t.Errorf("result should not contain %q, got:\n%s", unwant, result)
				}
			}
		})
	}
}

func TestCleanPath(t *testing.T) {
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
			got := cleanPath(tt.input)
			if got != tt.want {
				t.Errorf("cleanPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractTitle_FrontMatter(t *testing.T) {
	t.Parallel()

	// Ensure front matter is properly skipped and the heading after it is found
	fs := fstest.MapFS{
		"page.md": {Data: []byte("---\ntitle: FM Title\nauthor: Test\n---\n\nSome intro text.\n\n# Real Heading\n\nContent.")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/")

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	if root.Children[0].Label != "Real Heading" {
		t.Errorf("expected 'Real Heading', got %q", root.Children[0].Label)
	}
}

func TestGenerate_DeepNesting(t *testing.T) {
	t.Parallel()

	fs := fstest.MapFS{
		"a/b/c/deep.md": {Data: []byte("# Deep Page")},
	}

	gen := NewGenerator(fs, "README.md")
	root := gen.Generate("/a/b/c/deep.md")

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	// Navigate: root -> a -> b -> c -> deep.md
	aNode := root.Children[0]
	if aNode.Label != "A" || !aNode.IsDir || !aNode.IsOpen {
		t.Errorf("expected open dir A, got label=%q isDir=%v isOpen=%v", aNode.Label, aNode.IsDir, aNode.IsOpen)
	}

	bNode := aNode.Children[0]
	if bNode.Label != "B" || !bNode.IsDir || !bNode.IsOpen {
		t.Errorf("expected open dir B, got label=%q isDir=%v isOpen=%v", bNode.Label, bNode.IsDir, bNode.IsOpen)
	}

	cNode := bNode.Children[0]
	if cNode.Label != "C" || !cNode.IsDir || !cNode.IsOpen {
		t.Errorf("expected open dir C, got label=%q isDir=%v isOpen=%v", cNode.Label, cNode.IsDir, cNode.IsOpen)
	}

	deepNode := cNode.Children[0]
	if deepNode.Label != "Deep Page" || !deepNode.IsActive {
		t.Errorf("expected active 'Deep Page', got label=%q isActive=%v", deepNode.Label, deepNode.IsActive)
	}
}
