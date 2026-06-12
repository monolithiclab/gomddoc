package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
)

func newTagPageHandler(t *testing.T) (*TagPageHandler, *metadata.Index) {
	t.Helper()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A page\ntags: [go, docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B page\ntags: [docs]\n---\n# B")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":    {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-list.html.tmpl": {Data: []byte("{{ define \"tags-list\" }}<h1>Pages tagged {{ .Tag }}</h1><ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>{{ end }}")},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }

	return NewTagPageHandler(idx, r, tFunc, "" /* default lang */), idx
}

func TestTagPageHandler_Found(t *testing.T) {
	t.Parallel()

	h, _ := newTagPageHandler(t)

	req := httptest.NewRequest("GET", "/tags/docs", nil)
	req.SetPathValue("tag", "docs")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "A page") || !strings.Contains(body, "B page") {
		t.Errorf("body missing both pages\n%s", body)
	}
	if !strings.Contains(body, "Pages tagged docs") {
		t.Errorf("body missing title\n%s", body)
	}
}

func TestTagPageHandler_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newTagPageHandler(t)

	req := httptest.NewRequest("GET", "/tags/missing", nil)
	req.SetPathValue("tag", "missing")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTagPageHandler_OverlongTagRejected(t *testing.T) {
	t.Parallel()

	h, _ := newTagPageHandler(t)

	longTag := strings.Repeat("a", maxTagLength+1)
	req := httptest.NewRequest("GET", "/tags/x", nil)
	req.SetPathValue("tag", longTag)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("over-long tag: status = %d, want 404", w.Code)
	}
}

func TestTagPageHandler_DecodesPathValue(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: ML Page\ntags: [\"machine learning\"]\n---\n# A")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":    {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-list.html.tmpl": {Data: []byte("{{ define \"tags-list\" }}<ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>{{ end }}")},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }
	h := NewTagPageHandler(idx, r, tFunc, "")

	req := httptest.NewRequest("GET", "/tags/machine%20learning", nil)
	req.SetPathValue("tag", "machine learning")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ML Page") {
		t.Errorf("expected ML Page, got %s", w.Body.String())
	}
}

func TestTagsIndexHandler(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ntags: [go, docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B\ntags: [docs]\n---\n# B")},
		"c.md": {Data: []byte("---\ntitle: C\ntags: [tutorial]\n---\n# C")},
	}
	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":     {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-index.html.tmpl": {Data: []byte("{{ define \"tags-index\" }}<ul>{{ range .Tags }}<li>{{ .Tag }}({{ .Count }})</li>{{ end }}</ul>{{ end }}")},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }
	h := NewTagsIndexHandler(idx, r, tFunc, "")

	req := httptest.NewRequest("GET", "/tags/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"docs(2)", "go(1)", "tutorial(1)"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in %s", want, body)
		}
	}
	if i, j, k := strings.Index(body, "docs"), strings.Index(body, "go"), strings.Index(body, "tutorial"); !(i < j && j < k) {
		t.Errorf("not alphabetical: docs=%d go=%d tutorial=%d", i, j, k)
	}
}

func TestTagsIndexHandler_Empty(t *testing.T) {
	t.Parallel()

	idx, err := metadata.BuildIndex(t.Context(), fstest.MapFS{}, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl":     {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-index.html.tmpl": {Data: []byte("{{ define \"tags-index\" }}<p>tags index ({{ len .Tags }} tags)</p>{{ end }}")},
	}
	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }
	h := NewTagsIndexHandler(idx, r, tFunc, "")

	req := httptest.NewRequest("GET", "/tags/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty index should still 200, got %d", w.Code)
	}
}
