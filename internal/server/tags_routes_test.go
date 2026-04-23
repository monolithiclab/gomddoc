package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

func setupTagRenderer(t *testing.T) *tmpl.HTMLRenderer {
	t.Helper()

	tagsListPartial, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl")
	if err != nil {
		t.Fatalf("read tags-list partial: %v", err)
	}
	tagsIndexPartial, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl")
	if err != nil {
		t.Fatalf("read tags-index partial: %v", err)
	}

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<!DOCTYPE html><html><body>{{ .Page.Content }}</body></html>`),
		},
		"assets/themes/default/partials/tags-list.html.tmpl": {
			Data: tagsListPartial,
		},
		"assets/themes/default/partials/tags-index.html.tmpl": {
			Data: tagsIndexPartial,
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"
	return tmpl.NewHTMLRenderer(&siteConfig, testFS)
}

func TestServer_TagRoutes_DefaultLang(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ntags: [docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B\ntags: [tutorial]\n---\n# B")},
	}
	idx, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}

	localeFS := fstest.MapFS{
		"locales/en-US.yml": {Data: []byte(`
tags_tagged_as: "Pages tagged %s"
tags_empty: "No pages tagged %s"
tags_index_title: "All tags"
`)},
	}
	bundle, err := locale.LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: ":8080",
			Dir:  ".",
		},
		Site: config.NewSiteConfig("."),
	}

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(files, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTagRenderer(t),
		MetaIndex:        idx,
		LocaleBundle:     bundle,
		DefaultLang:      "en-US",
	})

	tests := []struct {
		path string
		want int
	}{
		{"/tags/docs", http.StatusOK},
		{"/tags/tutorial", http.StatusOK},
		{"/tags/missing", http.StatusNotFound},
		{"/tags/", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("%s: status = %d, want %d", tt.path, w.Code, tt.want)
			}
		})
	}
}

func TestServer_TagRoutes_PerLanguage(t *testing.T) {
	t.Parallel()

	// English files
	enFiles := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: English A\ntags: [docs]\n---\n# A")},
	}
	enIdx, err := metadata.BuildIndex(context.Background(), enFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex en: %v", err)
	}

	// French files
	frFiles := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: French A\ntags: [tutoriel]\n---\n# A")},
	}
	frIdx, err := metadata.BuildIndex(context.Background(), frFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex fr: %v", err)
	}

	localeFS := fstest.MapFS{
		"locales/en-US.yml": {Data: []byte(`
tags_tagged_as: "Pages tagged %s"
tags_empty: "No pages tagged %s"
tags_index_title: "All tags"
`)},
		"locales/fr.yml": {Data: []byte(`
tags_tagged_as: "Pages étiquetées %s"
tags_empty: "Aucune page étiquetée %s"
tags_index_title: "Toutes les étiquettes"
`)},
	}
	bundle, err := locale.LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: ":8080",
			Dir:  ".",
		},
		Site: config.NewSiteConfig("."),
	}

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(enFiles, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTagRenderer(t),
		MetaIndex:        enIdx,
		LocaleBundle:     bundle,
		DefaultLang:      "en-US",
		AllLanguages:     []string{"fr"},
		LangPipelines: map[string]LangPipelineConfig{
			"fr": {
				MetaIndex: frIdx,
				Provider:  newMemoryProvider(frFiles, "README.md", false),
			},
		},
	})

	tests := []struct {
		path string
		want int
	}{
		{"/tags/docs", http.StatusOK},
		{"/fr/tags/tutoriel", http.StatusOK},
		{"/fr/tags/missing", http.StatusNotFound},
		{"/fr/tags/", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("%s: status = %d, want %d", tt.path, w.Code, tt.want)
			}
		})
	}
}
