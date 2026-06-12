package enricher

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

func TestMarkdownEnricher_SupportedMimeTypes(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})
	types := e.SupportedMimeTypes()
	if len(types) != 1 || types[0] != "text/markdown" {
		t.Errorf("SupportedMimeTypes() = %v, want [text/markdown]", types)
	}
}

func TestMarkdownEnricher_Enrich_Metadata(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	content := []byte("---\ntitle: Test Page\ndescription: A test\ntags:\n  - go\n  - docs\n---\n# Hello\nWorld")

	result, err := e.Enrich(context.Background(), content, "/test.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Metadata == nil {
		t.Fatal("Enrich() Metadata is nil")
	}

	if title, ok := result.Metadata["title"].(string); !ok || title != "Test Page" {
		t.Errorf("Metadata[title] = %v, want 'Test Page'", result.Metadata["title"])
	}

	if desc, ok := result.Metadata["description"].(string); !ok || desc != "A test" {
		t.Errorf("Metadata[description] = %v, want 'A test'", result.Metadata["description"])
	}
}

func TestMarkdownEnricher_Enrich_TOC(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	content := []byte("# First\n## Second\n### Third\n## Another Second\n")

	result, err := e.Enrich(context.Background(), content, "/test.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.TOC == nil {
		t.Fatal("Enrich() TOC is nil")
	}

	if result.TOC.Level != 0 {
		t.Errorf("TOC root level = %d, want 0", result.TOC.Level)
	}

	if len(result.TOC.Children) != 1 {
		t.Fatalf("TOC root has %d children, want 1 (h1)", len(result.TOC.Children))
	}

	h1 := result.TOC.Children[0]
	if h1.Text != "First" {
		t.Errorf("h1.Text = %q, want 'First'", h1.Text)
	}
	if h1.ID != "first" {
		t.Errorf("h1.ID = %q, want 'first'", h1.ID)
	}

	if len(h1.Children) != 2 {
		t.Fatalf("h1 has %d children, want 2 (two h2s)", len(h1.Children))
	}

	if h1.Children[0].Text != "Second" {
		t.Errorf("first h2 text = %q, want 'Second'", h1.Children[0].Text)
	}
	if h1.Children[1].Text != "Another Second" {
		t.Errorf("second h2 text = %q, want 'Another Second'", h1.Children[1].Text)
	}
}

func TestMarkdownEnricher_Enrich_NoFrontmatter(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	content := []byte("# Hello\nPlain markdown without frontmatter.")

	result, err := e.Enrich(context.Background(), content, "/test.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if len(result.Metadata) > 0 {
		t.Errorf("Metadata = %v, want nil or empty", result.Metadata)
	}

	if result.TOC == nil {
		t.Error("TOC should not be nil for content with headings")
	}
}

func TestMarkdownEnricher_Enrich_EmptyContent(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	result, err := e.Enrich(context.Background(), nil, "/empty.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result == nil {
		t.Fatal("Enrich() returned nil result")
	}
}

func TestMarkdownEnricher_Enrich_Navigation(t *testing.T) {
	t.Parallel()
	navItems := []NavItem{
		{Title: "Guide", Path: "/guide/", IsDir: true, Children: []NavItem{
			{Title: "Getting Started", Path: "/guide/start.md"},
		}},
		{Title: "API", Path: "/api.md", Active: true},
	}

	e := NewMarkdownEnricher(MarkdownEnricherOptions{
		NavBuilder: func(_ string) []NavItem {
			return navItems
		},
	})

	result, err := e.Enrich(context.Background(), []byte("# Test"), "/api.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Navigation == nil {
		t.Fatal("Navigation is nil")
	}

	if len(result.Navigation.Items) != 2 {
		t.Fatalf("Navigation has %d items, want 2", len(result.Navigation.Items))
	}

	if result.Navigation.Items[1].Active != true {
		t.Error("API nav item should be active")
	}
}

func TestMarkdownEnricher_Enrich_NavigationNil(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	result, err := e.Enrich(context.Background(), []byte("# Test"), "/test.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Navigation != nil {
		t.Error("Navigation should be nil when no NavBuilder")
	}
}

func TestMarkdownEnricher_Enrich_NavigationEmpty(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{
		NavBuilder: func(_ string) []NavItem { return nil },
	})

	result, err := e.Enrich(context.Background(), []byte("# Test"), "/test.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Navigation != nil {
		t.Error("Navigation should be nil when NavBuilder returns empty")
	}
}

func TestMarkdownEnricher_Enrich_RelatedDocs(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": {Data: []byte("---\ntitle: Guide\ntags:\n  - tutorial\n---\n# Guide")},
		"api.md":   {Data: []byte("---\ntitle: API Reference\ntags:\n  - tutorial\n  - api\n---\n# API")},
		"other.md": {Data: []byte("---\ntitle: Other\ntags:\n  - unrelated\n---\n# Other")},
	}

	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})

	// Content that shares the "tutorial" tag
	content := []byte("---\ntitle: Current Page\ntags:\n  - tutorial\n---\n# Current")

	result, err := e.Enrich(context.Background(), content, "/current.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if len(result.RelatedDocs) < 1 {
		t.Fatalf("RelatedDocs has %d docs, want at least 1", len(result.RelatedDocs))
	}

	for _, doc := range result.RelatedDocs {
		if doc.Path == "/current.md" {
			t.Error("RelatedDocs should not include the current page")
		}
	}
}

// TestMarkdownEnricher_Enrich_RelatedDocs_SortedStable asserts related docs are
// returned in a deterministic order (title case-insensitive, path tiebreaker)
// so serve and build render the see-also section identically (REVIEW §9.4).
func TestMarkdownEnricher_Enrich_RelatedDocs_SortedStable(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"zebra.md": {Data: []byte("---\ntitle: Alpha\ntags: [x]\n---\n# z")},
		"apple.md": {Data: []byte("---\ntitle: Beta\ntags: [x]\n---\n# a")},
		"dup2.md":  {Data: []byte("---\ntitle: Same\ntags: [x]\n---\n# d2")},
		"dup1.md":  {Data: []byte("---\ntitle: Same\ntags: [x]\n---\n# d1")},
	}
	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}
	e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})

	result, err := e.Enrich(context.Background(), []byte("---\ntitle: Cur\ntags: [x]\n---\n# c"), "/current.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	gotTitles := make([]string, len(result.RelatedDocs))
	for i, d := range result.RelatedDocs {
		gotTitles[i] = d.Title
	}
	wantTitles := []string{"Alpha", "Beta", "Same", "Same"}
	if !slices.Equal(gotTitles, wantTitles) {
		t.Fatalf("related titles = %v, want %v", gotTitles, wantTitles)
	}
	// Equal titles ("Same") must be ordered by path: /dup1.md before /dup2.md.
	if result.RelatedDocs[2].Path != "/dup1.md" || result.RelatedDocs[3].Path != "/dup2.md" {
		t.Errorf("equal-title docs not path-ordered: %q then %q",
			result.RelatedDocs[2].Path, result.RelatedDocs[3].Path)
	}
}

// TestMarkdownEnricher_Enrich_RelatedDocs_Capped asserts the see-also list is
// bounded so a very common tag cannot bloat every page (REVIEW §9.4).
func TestMarkdownEnricher_Enrich_RelatedDocs_Capped(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{}
	for i := range 25 {
		name := fmt.Sprintf("p%02d.md", i)
		fsys[name] = &fstest.MapFile{Data: []byte("---\ntitle: P" + fmt.Sprintf("%02d", i) + "\ntags: [common]\n---\n# p")}
	}
	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}
	e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})

	result, err := e.Enrich(context.Background(), []byte("---\ntitle: Cur\ntags: [common]\n---\n# c"), "/current.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}
	if len(result.RelatedDocs) != maxRelatedDocs {
		t.Errorf("RelatedDocs len = %d, want capped at %d", len(result.RelatedDocs), maxRelatedDocs)
	}
}

// Regression: when URL extension stripping is in effect the handler passes
// a stripped path (/guide) but the metadata index stores raw .md paths
// (/guide.md). Self-exclusion must work across that mismatch.
func TestMarkdownEnricher_Enrich_RelatedDocs_StripsExtensionForSelfExclusion(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": {Data: []byte("---\ntitle: Guide\ntags:\n  - tutorial\n---\n# Guide")},
		"api.md":   {Data: []byte("---\ntitle: API\ntags:\n  - tutorial\n---\n# API")},
	}

	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})
	content := []byte("---\ntitle: Guide\ntags:\n  - tutorial\n---\n# Guide")

	// Caller passes the URL-stripped form, no .md suffix.
	result, err := e.Enrich(context.Background(), content, "/guide")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	for _, doc := range result.RelatedDocs {
		if doc.Path == "/guide.md" {
			t.Errorf("RelatedDocs should exclude the current page even when caller passes the stripped path; got %v", result.RelatedDocs)
		}
	}
}

func TestMarkdownEnricher_Enrich_NoRelatedDocsWithoutIndex(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	content := []byte("---\ntags:\n  - go\n---\n# Test")
	result, err := e.Enrich(context.Background(), content, "/test.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if len(result.RelatedDocs) != 0 {
		t.Errorf("RelatedDocs = %v, want empty without MetaIndex", result.RelatedDocs)
	}
}

func TestMarkdownEnricher_Enrich_NoRelatedDocsWithoutTags(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": {Data: []byte("---\ntitle: Guide\ntags:\n  - tutorial\n---\n# Guide")},
	}

	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})

	// Content without tags
	content := []byte("---\ntitle: No Tags\n---\n# No Tags")
	result, err := e.Enrich(context.Background(), content, "/notagged.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if len(result.RelatedDocs) != 0 {
		t.Errorf("RelatedDocs = %v, want empty for page without tags", result.RelatedDocs)
	}
}

func TestMarkdownEnricher_Enrich_MalformedTags(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"guide.md": {Data: []byte("---\ntitle: Guide\ntags:\n  - tutorial\n---\n# Guide")},
	}

	idx, err := metadata.BuildIndex(context.Background(), fsys, nil)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	e := NewMarkdownEnricher(MarkdownEnricherOptions{MetaIndex: idx})

	// Content with tags as a string instead of a list — should not panic
	content := []byte("---\ntitle: Bad Tags\ntags: single-string\n---\n# Bad Tags")
	result, err := e.Enrich(context.Background(), content, "/bad.md")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if len(result.RelatedDocs) != 0 {
		t.Errorf("RelatedDocs = %v, want empty for malformed tags", result.RelatedDocs)
	}
}

func TestMarkdownEnricher_Enrich_ContextCancelled(t *testing.T) {
	t.Parallel()
	e := NewMarkdownEnricher(MarkdownEnricherOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.Enrich(ctx, []byte("# Test"), "/test.md")
	if err == nil {
		t.Error("Enrich() should return error on cancelled context")
	}
}
