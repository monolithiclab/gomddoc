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
		"assets/themes/default/layouts/default.html.tmpl":      {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-list.html.tmpl":   {Data: []byte("{{ define \"tags-list\" }}<h1>Pages tagged {{ .Tag }}</h1><ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>{{ end }}")},
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
		"assets/themes/default/layouts/default.html.tmpl":      {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
		"assets/themes/default/partials/tags-list.html.tmpl":   {Data: []byte("{{ define \"tags-list\" }}<ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>{{ end }}")},
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
