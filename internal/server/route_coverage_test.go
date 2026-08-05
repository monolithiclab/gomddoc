package server

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/search"
)

// newFullRouteServer builds a server with every optional route group enabled —
// tags, sitemap, feed, search API, static assets — over a corpus large enough
// that each of their responses clears minCompressionSize.
func newFullRouteServer(t *testing.T) *HTTPServer {
	t.Helper()

	// Each page carries a shared tag (so /tags/docs lists all 60) and a unique
	// one (so /tags/ and /api/tags are themselves over minCompressionSize), and
	// a body long enough that the rendered page clears the threshold too.
	body := strings.Repeat("lorem ipsum dolor sit amet consectetur adipiscing elit. ", 30)
	files := fstest.MapFS{}
	for i := range 60 {
		files[fmt.Sprintf("page-%02d.md", i)] = &fstest.MapFile{
			Data: fmt.Appendf(nil,
				"---\ntitle: Page %02d Lorem Ipsum Dolor\ntags: [docs, lorem-ipsum-topic-%02d]\n---\n# Page %02d\n\n%s\n",
				i, i, i, body),
		}
	}

	metaIdx, err := metadata.BuildIndex(context.Background(), files, nil)
	if err != nil {
		t.Fatalf("metadata.BuildIndex: %v", err)
	}
	searchIdx, err := search.BuildIndex(context.Background(), files, metaIdx, nil)
	if err != nil {
		t.Fatalf("search.BuildIndex: %v", err)
	}

	localeFS := fstest.MapFS{
		"locales/en-US.yml": {Data: []byte(
			"tags_tagged_as: \"Pages tagged %s\"\ntags_empty: \"No pages tagged %s\"\ntags_index_title: \"All tags\"\n")},
	}
	bundle, err := locale.LoadBundle("en-US", localeFS, "locales")
	if err != nil {
		t.Fatalf("locale.LoadBundle: %v", err)
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Domain = "https://docs.example.com"
	cfg := &config.Config{
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   siteConfig,
	}

	// A stylesheet comfortably over minCompressionSize, and highly compressible.
	staticFS := fstest.MapFS{
		"style.css": {Data: []byte(strings.Repeat(".gmd-block { margin: 0 }\n", 200))},
	}

	return NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(files, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTagRenderer(t),
		MetaIndex:        metaIdx,
		SearchIndex:      searchIdx,
		StaticFS:         staticFS,
		LocaleBundle:     bundle,
		DefaultLang:      "en-US",
	})
}

// TestRouteGroups_CompressedAndMeasured pins the fix for "compression and
// metrics cover only 2 of 8 route groups": everything below used to bypass both
// because Compression and Metrics were attached to the content subgroups rather
// than to the group they all descend from. Moving either middleware back down
// to `content` turns every case here red.
//
// No t.Parallel: httpRequestsTotal is a package-level Prometheus counter.
func TestRouteGroups_CompressedAndMeasured(t *testing.T) {
	srv := newFullRouteServer(t)

	paths := []string{
		"/page-00.md",         // content (already covered before the fix)
		"/tags/docs",          // tag page
		"/tags/",              // tag index
		"/api/tags",           // metadata API
		"/api/search?q=lorem", // search API
		"/sitemap.xml",        // SEO
		"/feed.xml",           // SEO
		"/_assets/style.css",  // static assets
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			before := getCounterValue(t, httpRequestsTotal, "GET", "200")

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %.200s)", w.Code, w.Body.String())
			}
			if got := getCounterValue(t, httpRequestsTotal, "GET", "200"); got != before+1 {
				t.Errorf("httpRequestsTotal{GET,200} = %v, want %v", got, before+1)
			}
			if vary := w.Header().Values("Vary"); !slices.Contains(vary, "Accept-Encoding") {
				t.Errorf("Vary = %v, want it to contain Accept-Encoding", vary)
			}
			if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
				t.Fatalf("Content-Encoding = %q, want gzip (body %d bytes, want >= %d)",
					ce, w.Body.Len(), minCompressionSize)
			}

			// The gzip stream must decode, and to more than it compressed to —
			// a Content-Encoding header over a plain body is worse than none.
			zr, err := gzip.NewReader(w.Body)
			if err != nil {
				t.Fatalf("gzip.NewReader: %v", err)
			}
			plain, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("read gzip body: %v", err)
			}
			if len(plain) < minCompressionSize {
				t.Errorf("decoded body = %d bytes, want >= %d", len(plain), minCompressionSize)
			}
		})
	}
}

// TestRobotsTxt_VaryWithoutCompression covers the small-response half of the
// contract: /robots.txt is below minCompressionSize so it is served plain, but
// it still passes through Compression and so must advertise Vary.
func TestRobotsTxt_VaryWithoutCompression(t *testing.T) {
	srv := newFullRouteServer(t)

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if vary := w.Header().Values("Vary"); !slices.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %v, want it to contain Accept-Encoding", vary)
	}
	if ce := w.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want none for a %d-byte body", ce, w.Body.Len())
	}
}

// TestMetricsEndpoint_NotSelfCounted keeps /metrics off the base group: a scrape
// that increments the counter it is reporting feeds its own numbers back, and
// promhttp negotiates its own content encoding.
func TestMetricsEndpoint_NotSelfCounted(t *testing.T) {
	siteConfig := config.NewSiteConfig(".")
	cfg := &config.Config{
		// Empty AdminPort means AdminOnMain(): /metrics is registered on this mux.
		Server: config.ServerConfig{Port: ":8080", Dir: "."},
		Site:   siteConfig,
	}
	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(fstest.MapFS{}, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
	})

	before := getCounterValue(t, httpRequestsTotal, "GET", "200")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := getCounterValue(t, httpRequestsTotal, "GET", "200"); got != before {
		t.Errorf("httpRequestsTotal{GET,200} = %v, want it unchanged at %v", got, before)
	}
	if vary := w.Header().Values("Vary"); slices.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %v, want no Accept-Encoding: /metrics must not go through Compression", vary)
	}
}
