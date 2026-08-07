package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/testutil/provider"
	registryutil "github.com/monolithiclab/gomddoc/internal/testutil/registry"
)

// Re-export MemoryProvider from testutil for backward compatibility in server tests
type memoryProvider = provider.MemoryProvider

func newMemoryProvider(files fstest.MapFS, defaultIndex string, dirIndex bool) *memoryProvider {
	return provider.NewMemoryProvider(files, defaultIndex, dirIndex)
}

// testPassword is the password matching the sole "admin" credential in the
// store returned by newTestAuthStore.
const testPassword = "secret"

// newTestAuthStore builds a single-user credential store. bcrypt is slow by
// design, so callers should build one store per test rather than one per case.
func newTestAuthStore(t *testing.T) *CredentialStore {
	t.Helper()
	store, err := ParseHTPasswd(strings.NewReader("admin:" + testHash(t, testPassword)))
	if err != nil {
		t.Fatalf("ParseHTPasswd: %v", err)
	}
	return store
}

// errorLayout stands in for the theme's error.html.tmpl. Every bundled theme
// ships one, so a test renderer without it would exercise ErrorPage's plain-text
// fallback instead of the themed response every error path actually produces.
// The wrapping markup is what distinguishes it from that fallback, which is the
// bare "<code> <status text>"; errorBody builds the expected string.
const errorLayout = `<html><body>Error {{ .Page.Meta.error_code }} {{ .Page.Meta.error_title }}</body></html>`

// errorBody is what errorLayout renders for a status code.
func errorBody(statusCode int) string {
	return fmt.Sprintf("<html><body>Error %d %s</body></html>", statusCode, http.StatusText(statusCode))
}

// setupTestRenderer builds the canonical in-memory renderer for this package.
// The layout carries a breadcrumb bar so tests can assert on it by passing
// tmpl.WithBreadcrumbGenerator; without a generator the trail is nil and the
// bar renders nothing, so callers that ignore it see unchanged output.
func setupTestRenderer(opts ...tmpl.RendererOption) *tmpl.HTMLRenderer {
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Site.Meta.Title}}</title></head>
<body>{{ range .Page.Breadcrumbs }}[{{ .Label }}]{{ end }}{{.Page.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(templateContent),
		},
		"assets/themes/default/layouts/error.html.tmpl": {
			Data: []byte(errorLayout),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"

	return tmpl.NewHTMLRenderer(&siteConfig, testFS, opts...)
}

// setupTestErrorPage builds the themed error writer the server shares across a
// language scope, backed by the canonical test renderer.
func setupTestErrorPage() *ErrorPage {
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"
	return NewErrorPage(setupTestRenderer(), tmpl.ErrorContextInput{Site: &siteConfig})
}

func setupTestRegistry() renderer.RendererRegistry {
	return registryutil.NewRendererRegistry()
}

func setupTestEnricherRegistry() enricher.EnricherRegistry {
	return registryutil.NewEnricherRegistry()
}

// revalidationCase describes one cacheable endpoint for assertRevalidates.
type revalidationCase struct {
	Handler   http.Handler
	Path      string
	Accept    string // optional; sent on both requests
	WantType  string
	WantCache string
}

// assertRevalidates asserts the whole contract serveWithETag owes a cacheable
// endpoint: a 200 carrying a body, a Content-Type, a Cache-Control and an ETag,
// then a conditional GET with that ETag answering 304 — same Content-Type, no
// body. Each half alone is passed by a broken handler: checking only the 304
// status passes one that still ships the payload, and checking only the headers
// passes one that ignores If-None-Match. Three call sites each used to assert a
// different subset, so which of them caught a regression was luck.
func assertRevalidates(t *testing.T, tc revalidationCase) {
	t.Helper()

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, tc.Path, nil)
		if tc.Accept != "" {
			req.Header.Set("Accept", tc.Accept)
		}
		return req
	}

	first := httptest.NewRecorder()
	tc.Handler.ServeHTTP(first, newReq())

	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", first.Code)
	}
	if first.Body.Len() == 0 {
		t.Fatal("200 carried no body")
	}
	if got := first.Header().Get("Content-Type"); got != tc.WantType {
		t.Errorf("Content-Type = %q, want %q", got, tc.WantType)
	}
	if got := first.Header().Get("Cache-Control"); got != tc.WantCache {
		t.Errorf("Cache-Control = %q, want %q", got, tc.WantCache)
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}

	req := newReq()
	req.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	tc.Handler.ServeHTTP(second, req)

	if second.Code != http.StatusNotModified {
		t.Fatalf("conditional GET: status = %d, want 304", second.Code)
	}
	if got := second.Header().Get("Content-Type"); got != tc.WantType {
		t.Errorf("304 Content-Type = %q, want %q", got, tc.WantType)
	}
	if second.Body.Len() != 0 {
		t.Errorf("304 carried a body: %q", second.Body.String())
	}
}
