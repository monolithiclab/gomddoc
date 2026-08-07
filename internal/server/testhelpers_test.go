package server

import (
	"fmt"
	"net/http"
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
