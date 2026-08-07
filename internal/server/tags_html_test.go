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

// newTagHandlerConfig wires an index and renderer into the config both tag
// handlers take, including the themed error page they share with the language's
// content handler.
func newTagHandlerConfig(t *testing.T, files, themeFS fstest.MapFS) TagHandlerConfig {
	t.Helper()

	idx, err := metadata.BuildIndex(t.Context(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	themeFS["assets/themes/default/layouts/error.html.tmpl"] = &fstest.MapFile{Data: []byte(errorLayout)}

	siteConfig := config.NewSiteConfig(".")
	r := template.NewHTMLRenderer(&siteConfig, themeFS)
	tFunc := func(k string) string { return k }

	return TagHandlerConfig{
		Index:     idx,
		Renderer:  r,
		TFunc:     tFunc,
		ErrorPage: NewErrorPage(r, template.ErrorContextInput{Site: &siteConfig, TFunc: tFunc}),
	}
}

func newTagPageHandler(t *testing.T) (*TagPageHandler, *metadata.Index) {
	t.Helper()

	cfg := newTagHandlerConfig(t,
		fstest.MapFS{
			"a.md": {Data: []byte("---\ntitle: A page\ntags: [go, docs]\n---\n# A")},
			"b.md": {Data: []byte("---\ntitle: B page\ntags: [docs]\n---\n# B")},
		},
		fstest.MapFS{
			"assets/themes/default/layouts/default.html.tmpl":    {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
			"assets/themes/default/partials/tags-list.html.tmpl": {Data: []byte("{{ define \"tags-list\" }}<h1>Pages tagged {{ .Tag }}</h1><ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>{{ end }}")},
		})

	return NewTagPageHandler(cfg), cfg.Index
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
	// The theme's 404, not net/http's default: a client that can tell which
	// handler answered can tell an unknown tag from an excluded one.
	if got, want := w.Body.String(), errorBody(http.StatusNotFound); got != want {
		t.Errorf("body = %q, want the theme's error layout %q", got, want)
	}
	if got := w.Header().Get("Content-Type"); got != mimeHTML {
		t.Errorf("Content-Type = %q, want %q", got, mimeHTML)
	}
}

func TestTagPageHandler_OverlongTagRejected(t *testing.T) {
	t.Parallel()

	h, _ := newTagPageHandler(t)

	longTag := strings.Repeat("a", metadata.MaxTagLength+1)
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

	h := NewTagPageHandler(newTagHandlerConfig(t,
		fstest.MapFS{
			"a.md": {Data: []byte("---\ntitle: ML Page\ntags: [\"machine learning\"]\n---\n# A")},
		},
		fstest.MapFS{
			"assets/themes/default/layouts/default.html.tmpl":    {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
			"assets/themes/default/partials/tags-list.html.tmpl": {Data: []byte("{{ define \"tags-list\" }}<ul>{{ range .Pages }}<li>{{ .Title }}</li>{{ end }}</ul>{{ end }}")},
		}))

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

	h := NewTagsIndexHandler(newTagHandlerConfig(t,
		fstest.MapFS{
			"a.md": {Data: []byte("---\ntitle: A\ntags: [go, docs]\n---\n# A")},
			"b.md": {Data: []byte("---\ntitle: B\ntags: [docs]\n---\n# B")},
			"c.md": {Data: []byte("---\ntitle: C\ntags: [tutorial]\n---\n# C")},
		},
		fstest.MapFS{
			"assets/themes/default/layouts/default.html.tmpl":     {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
			"assets/themes/default/partials/tags-index.html.tmpl": {Data: []byte("{{ define \"tags-index\" }}<ul>{{ range .Tags }}<li>{{ .Tag }}({{ .Count }})</li>{{ end }}</ul>{{ end }}")},
		}))

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

	h := NewTagsIndexHandler(newTagHandlerConfig(t,
		fstest.MapFS{},
		fstest.MapFS{
			"assets/themes/default/layouts/default.html.tmpl":     {Data: []byte(`<html><body>{{ .Page.Content }}</body></html>`)},
			"assets/themes/default/partials/tags-index.html.tmpl": {Data: []byte("{{ define \"tags-index\" }}<p>tags index ({{ len .Tags }} tags)</p>{{ end }}")},
		}))

	req := httptest.NewRequest("GET", "/tags/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty index should still 200, got %d", w.Code)
	}
}
