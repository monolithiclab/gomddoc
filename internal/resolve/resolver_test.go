package resolve

import (
	"mime"
	"testing"
	"testing/fstest"
)

func init() {
	// Register markdown MIME type for tests (normally done by renderer package init).
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
}

// mockRenderer returns a RendererCheck that returns true for the given MIME types.
func mockRenderer(types ...string) RendererCheck {
	set := make(map[string]bool, len(types))
	for _, t := range types {
		set[t] = true
	}
	return func(mimeType string) bool {
		return set[mimeType]
	}
}

func TestResolver_BasicResolution(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"index.md":        {},
		"guide.md":        {},
		"images/logo.jpg": {},
	}

	r := Build(fsys, []string{".md"}, mockRenderer("text/markdown"))

	tests := []struct {
		name      string
		clean     string
		wantReal  string
		wantFound bool
	}{
		{"resolve index", "index", "index.md", true},
		{"resolve guide", "guide", "guide.md", true},
		{"jpg not stripped", "images/logo", "", false},
		{"non-existent", "missing", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := r.Resolve(tt.clean)
			if found != tt.wantFound {
				t.Fatalf("Resolve(%q) found=%v, want %v", tt.clean, found, tt.wantFound)
			}
			if got != tt.wantReal {
				t.Errorf("Resolve(%q) = %q, want %q", tt.clean, got, tt.wantReal)
			}
		})
	}

	// Verify CleanPath works for the same files.
	cleanPath, found := r.CleanPath("guide.md")
	if !found || cleanPath != "guide" {
		t.Errorf("CleanPath(guide.md) = %q, %v; want %q, true", cleanPath, found, "guide")
	}
}

func TestResolver_MultiExtensionCollision(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"guide.md":   {},
		"guide.html": {},
	}

	// .md is first, so it wins the "guide" clean path.
	r := Build(fsys, []string{".md", ".html"}, mockRenderer("text/markdown", "text/html"))

	real, found := r.Resolve("guide")
	if !found || real != "guide.md" {
		t.Errorf("Resolve(guide) = %q, %v; want guide.md, true", real, found)
	}

	// guide.html should NOT have a clean path since guide.md claimed it.
	_, found = r.CleanPath("guide.html")
	if found {
		t.Error("CleanPath(guide.html) should not be found, guide.md won the collision")
	}
}

func TestResolver_DirectoryCollision(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"guide.md":          {},
		"guide/install.md":  {},
		"guide/overview.md": {},
	}

	r := Build(fsys, []string{".md"}, mockRenderer("text/markdown"))

	// File wins over directory.
	real, found := r.Resolve("guide")
	if !found || real != "guide.md" {
		t.Errorf("Resolve(guide) = %q, %v; want guide.md, true", real, found)
	}

	// Directory contents still have their own mappings.
	real, found = r.Resolve("guide/install")
	if !found || real != "guide/install.md" {
		t.Errorf("Resolve(guide/install) = %q, %v; want guide/install.md, true", real, found)
	}

	real, found = r.Resolve("guide/overview")
	if !found || real != "guide/overview.md" {
		t.Errorf("Resolve(guide/overview) = %q, %v; want guide/overview.md, true", real, found)
	}
}

func TestResolver_MultipleDots(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"my.config.md": {},
	}

	r := Build(fsys, []string{".md"}, mockRenderer("text/markdown"))

	real, found := r.Resolve("my.config")
	if !found || real != "my.config.md" {
		t.Errorf("Resolve(my.config) = %q, %v; want my.config.md, true", real, found)
	}

	clean, found := r.CleanPath("my.config.md")
	if !found || clean != "my.config" {
		t.Errorf("CleanPath(my.config.md) = %q, %v; want my.config, true", clean, found)
	}
}

func TestResolver_EmptyConfig(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"index.md": {},
	}

	r := Build(fsys, nil, mockRenderer("text/markdown"))

	if !r.IsEmpty() {
		t.Error("expected empty resolver with nil stripExts")
	}

	_, found := r.Resolve("index")
	if found {
		t.Error("empty resolver should not resolve anything")
	}

	r2 := Build(fsys, []string{}, mockRenderer("text/markdown"))
	if !r2.IsEmpty() {
		t.Error("expected empty resolver with empty stripExts")
	}
}

func TestResolver_CleanPath(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"docs/guide.md":    {},
		"docs/install.md":  {},
		"assets/style.css": {},
	}

	r := Build(fsys, []string{".md"}, mockRenderer("text/markdown"))

	tests := []struct {
		name      string
		realPath  string
		wantClean string
		wantFound bool
	}{
		{"mapped file", "docs/guide.md", "docs/guide", true},
		{"mapped file 2", "docs/install.md", "docs/install", true},
		{"unmapped file", "assets/style.css", "", false},
		{"non-existent", "missing.md", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := r.CleanPath(tt.realPath)
			if found != tt.wantFound {
				t.Fatalf("CleanPath(%q) found=%v, want %v", tt.realPath, found, tt.wantFound)
			}
			if got != tt.wantClean {
				t.Errorf("CleanPath(%q) = %q, want %q", tt.realPath, got, tt.wantClean)
			}
		})
	}
}

func TestResolver_NonRenderedFileSkipped(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"data.csv": {},
		"guide.md": {},
	}

	// .csv is in strip list but hasRenderer returns false for text/csv.
	r := Build(fsys, []string{".csv", ".md"}, mockRenderer("text/markdown"))

	// .csv should not be stripped since it has no renderer.
	_, found := r.Resolve("data")
	if found {
		t.Error("data.csv should not be resolved; its MIME type has no renderer")
	}

	// .md should still work.
	real, found := r.Resolve("guide")
	if !found || real != "guide.md" {
		t.Errorf("Resolve(guide) = %q, %v; want guide.md, true", real, found)
	}
}

func TestResolver_AllMappings(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"a.md": {},
		"b.md": {},
	}

	r := Build(fsys, []string{".md"}, mockRenderer("text/markdown"))

	m := r.AllMappings()
	if len(m) != 2 {
		t.Fatalf("AllMappings() returned %d entries, want 2", len(m))
	}
	if m["a.md"] != "a" {
		t.Errorf("AllMappings()[a.md] = %q, want a", m["a.md"])
	}
	if m["b.md"] != "b" {
		t.Errorf("AllMappings()[b.md] = %q, want b", m["b.md"])
	}

	// The returned map is a defensive copy: mutating it must not affect the
	// resolver's internal state (REVIEW §9.8).
	m["a.md"] = "mutated"
	delete(m, "b.md")
	m["injected"] = "x"
	if clean, _ := r.CleanPath("a.md"); clean != "a" {
		t.Errorf("after mutating returned map, CleanPath(a.md) = %q, want a", clean)
	}
	if clean, ok := r.CleanPath("b.md"); !ok || clean != "b" {
		t.Errorf("after deleting from returned map, CleanPath(b.md) = %q (ok=%v), want b", clean, ok)
	}
	if m2 := r.AllMappings(); len(m2) != 2 {
		t.Errorf("resolver mutated via returned map: AllMappings() now has %d entries, want 2", len(m2))
	}
}
