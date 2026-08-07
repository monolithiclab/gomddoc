package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

func setupTagRenderer(t *testing.T, opts ...tmpl.RendererOption) *tmpl.HTMLRenderer {
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
		"assets/themes/default/layouts/error.html.tmpl": {
			Data: []byte(errorLayout),
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
	return tmpl.NewHTMLRenderer(&siteConfig, testFS, opts...)
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

	// French files. "b.md" exists only here, so the default-language resolver
	// cannot map it — linking it correctly requires the fr pipeline's own.
	frFiles := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: French A\ntags: [tutoriel]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: French B\ntags: [tutoriel]\n---\n# B")},
	}
	frIdx, err := metadata.BuildIndex(context.Background(), frFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex fr: %v", err)
	}
	frResolver := resolve.Build(frFiles, resolve.BuildOptions{
		StripExtensions: []string{".md"},
		HasRenderer:     func(string) bool { return true },
	})

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
				MetaIndex:        frIdx,
				Provider:         newMemoryProvider(frFiles, "README.md", false),
				Resolver:         frResolver,
				TemplateRenderer: setupTagRenderer(t, tmpl.WithResolver(frResolver), tmpl.WithLangPrefix("fr")),
			},
		},
	})

	tests := []struct {
		path string
		want int
		// body, when set, must appear in the response. Tag listings hold real
		// file paths, so a wrong-pipeline renderer emits "/b.md" here.
		body string
	}{
		{path: "/tags/docs", want: http.StatusOK},
		{path: "/fr/tags/tutoriel", want: http.StatusOK, body: `href="/fr/b"`},
		// The themed 404, not net/http's default: a tag route must answer an
		// unknown tag exactly as the language's content handler answers an
		// unknown page.
		{path: "/fr/tags/missing", want: http.StatusNotFound, body: errorBody(http.StatusNotFound)},
		{path: "/fr/tags/", want: http.StatusOK},
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
			if tt.body != "" && !strings.Contains(w.Body.String(), tt.body) {
				t.Errorf("%s: body missing %s:\n%s", tt.path, tt.body, w.Body.String())
			}
		})
	}
}
