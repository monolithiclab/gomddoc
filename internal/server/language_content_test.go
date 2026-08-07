package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
)

// TestNewHTTPServer_NilLocaleBundleWithLangPipelines proves NewHTTPServer does
// not panic when LangPipelines is non-empty but LocaleBundle is nil. Regression
// test for REVIEW.md §9.4: the per-language loop dereferenced LocaleBundle
// unconditionally while the default-language path guarded it.
func TestNewHTTPServer_NilLocaleBundleWithLangPipelines(t *testing.T) {
	t.Parallel()

	frFiles := fstest.MapFS{"guide.md": {Data: []byte("# Guide")}}
	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   config.NewSiteConfig("."),
	}

	// Must not panic.
	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(fstest.MapFS{}, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		DefaultLang:      "en-US",
		AllLanguages:     []string{"fr"},
		LocaleBundle:     nil,
		LangPipelines: map[string]LangPipelineConfig{
			"fr": {
				TemplateRenderer: setupTestRenderer(),
				Provider:         newMemoryProvider(frFiles, "README.md", false),
				EnricherRegistry: setupTestEnricherRegistry(),
			},
		},
	})
	if srv == nil {
		t.Fatal("NewHTTPServer returned nil")
	}
}

// localeFSForLangTest returns a minimal locale FS covering en-US and fr so the
// per-language content handler can build its TFunc.
func localeFSForLangTest() fstest.MapFS {
	return fstest.MapFS{
		"locales/en-US.yml": {Data: []byte("tags_index_title: \"All tags\"\n")},
		"locales/fr.yml":    {Data: []byte("tags_index_title: \"Toutes les étiquettes\"\n")},
	}
}

// TestServer_ContentRoutes_PerLanguage proves that per-language content pages
// are served (not 404'd) from the language-rooted provider. Regression test for
// REVIEW.md §9.1: the /{lang} prefix was not stripped before the language
// provider, so localized content pages always 404'd. The provider is rooted at
// the language subdirectory, so the handler must serve "/fr/guide.md" by reading
// "guide.md" from the French provider.
func TestServer_ContentRoutes_PerLanguage(t *testing.T) {
	t.Parallel()

	enFiles := fstest.MapFS{
		"README.md": {Data: []byte("# Home\n\nenglish-index-marker-1c4\n")},
		"guide.md":  {Data: []byte("# Guide\n\nenglish-only-marker-9a7\n")},
	}
	frFiles := fstest.MapFS{
		"README.md": {Data: []byte("# Accueil\n\nfrench-index-marker-5d8\n")},
		"guide.md":  {Data: []byte("# Guide\n\nfrench-only-marker-3b2\n")},
	}
	enIdx, err := metadata.BuildIndex(context.Background(), enFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex en: %v", err)
	}
	frIdx, err := metadata.BuildIndex(context.Background(), frFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex fr: %v", err)
	}

	bundle, err := locale.LoadBundle("en-US", localeFSForLangTest(), "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   config.NewSiteConfig("."),
	}

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(enFiles, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		MetaIndex:        enIdx,
		LocaleBundle:     bundle,
		DefaultLang:      "en-US",
		AllLanguages:     []string{"fr"},
		LangPipelines: map[string]LangPipelineConfig{
			"fr": {
				TemplateRenderer: setupTestRenderer(),
				MetaIndex:        frIdx,
				Provider:         newMemoryProvider(frFiles, "README.md", false),
				EnricherRegistry: setupTestEnricherRegistry(),
			},
		},
	})

	tests := []struct {
		name     string
		path     string
		want     int
		contains string
		excludes string
	}{
		{name: "default language content", path: "/guide.md", want: http.StatusOK, contains: "english-only-marker-9a7", excludes: "french-only-marker-3b2"},
		{name: "french content served from language provider", path: "/fr/guide.md", want: http.StatusOK, contains: "french-only-marker-3b2", excludes: "english-only-marker-9a7"},
		{name: "french index served", path: "/fr/", want: http.StatusOK, contains: "french-index-marker-5d8", excludes: "english-index-marker-1c4"},
		{name: "missing french page 404s", path: "/fr/missing.md", want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept", "text/html")
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Fatalf("%s: status = %d, want %d", tt.path, w.Code, tt.want)
			}
			body := w.Body.String()
			if tt.contains != "" && !strings.Contains(body, tt.contains) {
				t.Errorf("%s: body missing %q\nbody: %s", tt.path, tt.contains, body)
			}
			if tt.excludes != "" && strings.Contains(body, tt.excludes) {
				t.Errorf("%s: body unexpectedly contains %q (wrong language served)", tt.path, tt.excludes)
			}
		})
	}
}

// TestServer_PerLanguageSitemaps_ButNoIndex pins the serve/build asymmetry the
// guides now describe: serve routes one sitemap per language and *no*
// sitemap-index.xml, while build writes the index. Both live sitemaps are
// asserted first because the routes are gated on meta.domain — without a domain
// the 404 below would pass no matter what NewHTTPServer registered.
func TestServer_PerLanguageSitemaps_ButNoIndex(t *testing.T) {
	t.Parallel()

	enFiles := fstest.MapFS{"guide.md": {Data: []byte("# Guide\n")}}
	frFiles := fstest.MapFS{"guide.md": {Data: []byte("# Guide\n")}}

	enIdx, err := metadata.BuildIndex(context.Background(), enFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex en: %v", err)
	}
	frIdx, err := metadata.BuildIndex(context.Background(), frFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex fr: %v", err)
	}

	site := config.NewSiteConfig(".")
	site.Meta.Domain = "https://example.com"
	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   site,
	}

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(enFiles, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		MetaIndex:        enIdx,
		DefaultLang:      "en-US",
		AllLanguages:     []string{"fr"},
		LangPipelines: map[string]LangPipelineConfig{
			"fr": {
				TemplateRenderer: setupTestRenderer(),
				MetaIndex:        frIdx,
				Provider:         newMemoryProvider(frFiles, "README.md", false),
				EnricherRegistry: setupTestEnricherRegistry(),
			},
		},
	})

	tests := []struct {
		path string
		want int
	}{
		{"/sitemap.xml", http.StatusOK},
		{"/fr/sitemap.xml", http.StatusOK},
		{"/sitemap-index.xml", http.StatusNotFound},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		w := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(w, req)
		if w.Code != tt.want {
			t.Errorf("GET %s = %d, want %d", tt.path, w.Code, tt.want)
		}
	}
}

// TestServer_ContentRoutes_PerLanguage_Redirects proves both redirect kinds work
// inside a language subtree and target the language's own pages:
//
//   - extension stripping — "/fr/guide.md" 301s to the language-prefixed clean URL
//     "/fr/guide" (not the unprefixed "/guide"), which then resolves against the
//     per-language resolver. Regression test for REVIEW.md §9.1.
//   - redirect_from — only the default handler received URLRedirects before, so a
//     French page's redirect_from was honoured only because the default pipeline
//     indexed the fr/ directory, and it then pointed at the wrong language's page.
func TestServer_ContentRoutes_PerLanguage_Redirects(t *testing.T) {
	t.Parallel()

	enFiles := fstest.MapFS{"guide.md": {Data: []byte("# Guide\n\nenglish\n")}}
	frFiles := fstest.MapFS{"guide.md": {Data: []byte(
		"---\ntitle: Guide\nredirect_from:\n  - /ancien-guide\n---\n# Guide\n\nfrench-only-marker-3b2\n")}}

	enIdx, err := metadata.BuildIndex(context.Background(), enFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex en: %v", err)
	}
	frIdx, err := metadata.BuildIndex(context.Background(), frFiles, nil)
	if err != nil {
		t.Fatalf("BuildIndex fr: %v", err)
	}

	isRenderable := func(string) bool { return true }
	enResolver := resolve.Build(enFiles, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: isRenderable})
	frResolver := resolve.Build(frFiles, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: isRenderable})

	bundle, err := locale.LoadBundle("en-US", localeFSForLangTest(), "locales")
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   config.NewSiteConfig("."),
	}
	cfg.Site.StripExtensions = []string{".md"}

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(enFiles, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		MetaIndex:        enIdx,
		Resolver:         enResolver,
		LocaleBundle:     bundle,
		DefaultLang:      "en-US",
		AllLanguages:     []string{"fr"},
		LangPipelines: map[string]LangPipelineConfig{
			"fr": {
				TemplateRenderer: setupTestRenderer(),
				MetaIndex:        frIdx,
				Provider:         newMemoryProvider(frFiles, "README.md", false),
				Resolver:         frResolver,
				EnricherRegistry: setupTestEnricherRegistry(),
				URLRedirects:     BuildRedirectMap(frIdx, frResolver, "/fr"),
			},
		},
	})

	// /fr/guide.md must 301 to the language-prefixed clean URL.
	req := httptest.NewRequest(http.MethodGet, "/fr/guide.md", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("/fr/guide.md: status = %d, want 301", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/fr/guide" {
		t.Fatalf("/fr/guide.md: Location = %q, want %q", loc, "/fr/guide")
	}

	// The clean URL resolves against the per-language resolver and serves French.
	req = httptest.NewRequest(http.MethodGet, "/fr/guide", nil)
	req.Header.Set("Accept", "text/html")
	w = httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/fr/guide: status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "french-only-marker-3b2") {
		t.Errorf("/fr/guide: body missing French marker\nbody: %s", body)
	}

	// redirect_from on the French page must resolve to the French page.
	req = httptest.NewRequest(http.MethodGet, "/fr/ancien-guide", nil)
	w = httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("/fr/ancien-guide: status = %d, want 301", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/fr/guide" {
		t.Errorf("/fr/ancien-guide: Location = %q, want %q", loc, "/fr/guide")
	}
}
