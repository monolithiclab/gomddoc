package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// redirectFinderFromFS creates a RedirectFinder using a navigation.Generator,
// mirroring the adapter pattern used in cmd/gomddoc/serve.go.
func redirectFinderFromFS(files fstest.MapFS, defaultIndex string) RedirectFinder {
	navGen := navigation.NewGenerator(files, defaultIndex, nil)
	return func(dirPath string) string {
		tree := navGen.Generate(dirPath)
		return navigation.FindFirstPage(tree)
	}
}

func TestHandlerServeContent_ContentNegotiation(t *testing.T) {
	t.Parallel()

	// Create in-memory filesystem - NO disk I/O!
	files := fstest.MapFS{
		"test.md":  &fstest.MapFile{Data: []byte("# Test Markdown")},
		"test.png": &fstest.MapFile{Data: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
		SiteConfig:       &siteConfig,
	})

	tests := []struct {
		name           string
		path           string
		acceptHeader   string
		expectedStatus int
		expectedType   string
		shouldContain  string
		expectedLength int
	}{
		{
			name:           "markdown rendered as HTML with explicit accept",
			path:           "/test.md",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "<h1",
		},
		{
			name:           "markdown rendered with wildcard accept",
			path:           "/test.md",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "<!DOCTYPE html>",
		},
		{
			name:           "markdown rendered with text/* accept",
			path:           "/test.md",
			acceptHeader:   "text/*",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Test Markdown",
		},
		{
			name:           "markdown with empty accept defaults to */*",
			path:           "/test.md",
			acceptHeader:   "",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Test Markdown",
		},
		{
			name:           "markdown passthrough with text/markdown accept",
			path:           "/test.md",
			acceptHeader:   "text/markdown",
			expectedStatus: 200,
			expectedType:   "text/markdown; charset=utf-8",
			shouldContain:  "# Test Markdown",
		},
		{
			name:           "binary file (png) with wildcard accept",
			path:           "/test.png",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "image/png",
			expectedLength: 8,
		},
		{
			name:           "accept application/json - not acceptable",
			path:           "/test.md",
			acceptHeader:   "application/json",
			expectedStatus: 406,
			expectedType:   "text/plain; charset=utf-8",
			shouldContain:  "Not Acceptable",
		},
		{
			name:           "accept image/png for markdown - not acceptable",
			path:           "/test.md",
			acceptHeader:   "image/png",
			expectedStatus: 406,
			expectedType:   "text/plain; charset=utf-8",
			shouldContain:  "Not Acceptable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if contentType := w.Header().Get("Content-Type"); contentType != tt.expectedType {
				t.Errorf("content-type = %q, want %q", contentType, tt.expectedType)
			}

			if tt.shouldContain != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.shouldContain) {
					t.Errorf("response should contain %q", tt.shouldContain)
				}
			}

			if tt.expectedLength > 0 && w.Body.Len() != tt.expectedLength {
				t.Errorf("body length = %d, want %d", w.Body.Len(), tt.expectedLength)
			}

			if tt.expectedStatus == 200 && w.Header().Get("Content-Length") == "" {
				t.Error("Content-Length header should be set for successful responses")
			}
		})
	}
}

func TestHandlerErrorResponses(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"exists.md": &fstest.MapFile{Data: []byte("# Exists")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
		SiteConfig:       &siteConfig,
	})

	req := httptest.NewRequest("GET", "/nonexistent/deeply/nested/file.md", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Errorf("error response content-type = %q, want text/html; charset=utf-8", contentType)
	}

	if !strings.Contains(w.Body.String(), "404 Not Found") {
		t.Error("response should contain '404 Not Found'")
	}
}

func TestHandlerCacheHeaders(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"test.md": &fstest.MapFile{Data: []byte("# Cache Test")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
		SiteConfig:       &siteConfig,
	})

	req := httptest.NewRequest("GET", "/test.md", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	if cacheControl := w.Header().Get("Cache-Control"); cacheControl != "public, max-age=300" {
		t.Errorf("cache-control = %q, want public, max-age=300", cacheControl)
	}
}

func TestHandler304IncludesContentType(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"test.md":   &fstest.MapFile{Data: []byte("# Hello")},
		"style.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		SiteConfig:       &siteConfig,
	})

	tests := []struct {
		name     string
		path     string
		accept   string
		wantType string
	}{
		{"html content", "/test.md", "text/html", "text/html; charset=utf-8"},
		{"raw content", "/style.css", "*/*", "text/css; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// First request to get the ETag
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Accept", tt.accept)
			w := httptest.NewRecorder()
			handler.ServeContent(w, req)

			etag := w.Header().Get("ETag")
			if etag == "" {
				t.Fatal("expected ETag on first request")
			}

			// Conditional request should return 304 with Content-Type
			req2 := httptest.NewRequest("GET", tt.path, nil)
			req2.Header.Set("Accept", tt.accept)
			req2.Header.Set("If-None-Match", etag)
			w2 := httptest.NewRecorder()
			handler.ServeContent(w2, req2)

			if w2.Code != http.StatusNotModified {
				t.Fatalf("status = %d, want 304", w2.Code)
			}
			if ct := w2.Header().Get("Content-Type"); ct != tt.wantType {
				t.Errorf("304 Content-Type = %q, want %q", ct, tt.wantType)
			}
		})
	}
}

func TestHandlerForbiddenDirectory(t *testing.T) {
	t.Parallel()

	// In-memory directory structure - NO disk I/O!
	files := fstest.MapFS{
		"testdir/.keep": &fstest.MapFile{Data: []byte("")}, // Empty dir with no README
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false) // dirIndex=false
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
		SiteConfig:       &siteConfig,
	})

	req := httptest.NewRequest("GET", "/testdir", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}

	if !strings.Contains(w.Body.String(), "403 Forbidden") {
		t.Errorf("response should contain '403 Forbidden'")
	}
}

func TestHandlerDirectoryRedirect(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"docs/guide.md":   &fstest.MapFile{Data: []byte("# Guide")},
		"docs/install.md": &fstest.MapFile{Data: []byte("# Install")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	redirectFinder := redirectFinderFromFS(files, "README.md")
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
		SiteConfig:       &siteConfig,
		RedirectFinder:   redirectFinder,
	})

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 302 {
		t.Errorf("status = %d, want 302", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/docs/guide.md" {
		t.Errorf("Location = %q, want /docs/guide.md", location)
	}
}

func TestHandlerLayoutSelection(t *testing.T) {
	t.Parallel()

	// Create a template renderer with multiple layouts
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`DEFAULT:{{.Page.Content}}`),
		},
		"assets/themes/default/layouts/page.html.tmpl": {
			Data: []byte(`PAGE:{{.Page.Content}}`),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rend := tmpl.NewHTMLRenderer(&siteConfig, testFS)
	registry := setupTestRegistry()

	tests := []struct {
		name         string
		markdown     string
		wantContains string
		wantAbsent   string
	}{
		{
			name:         "page with layout:page uses page template",
			markdown:     "---\nlayout: page\n---\n# Page Layout",
			wantContains: "PAGE:",
			wantAbsent:   "DEFAULT:",
		},
		{
			name:         "page with no layout uses default template",
			markdown:     "---\ntitle: Hello\n---\n# Default Layout",
			wantContains: "DEFAULT:",
			wantAbsent:   "PAGE:",
		},
		{
			name:         "page with nonexistent layout falls back to default",
			markdown:     "---\nlayout: nonexistent\n---\n# Fallback",
			wantContains: "DEFAULT:",
			wantAbsent:   "PAGE:",
		},
		{
			name:         "page with no frontmatter uses default template",
			markdown:     "# No Frontmatter",
			wantContains: "DEFAULT:",
			wantAbsent:   "PAGE:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := fstest.MapFS{
				"test.md": &fstest.MapFile{Data: []byte(tt.markdown)},
			}

			prov := newMemoryProvider(files, "README.md", false)
			handler := NewHandler(HandlerConfig{
				Provider:         prov,
				Registry:         registry,
				EnricherRegistry: setupTestEnricherRegistry(),
				TemplateRenderer: rend,
				SiteConfig:       &siteConfig,
			})

			req := httptest.NewRequest("GET", "/test.md", nil)
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != 200 {
				t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.wantContains) {
				t.Errorf("response should contain %q, got %q", tt.wantContains, body)
			}
			if tt.wantAbsent != "" && strings.Contains(body, tt.wantAbsent) {
				t.Errorf("response should not contain %q, got %q", tt.wantAbsent, body)
			}
		})
	}
}

func TestHandlerURLRedirect(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"test.md": &fstest.MapFile{Data: []byte("# Test")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		SiteConfig:       &siteConfig,
		URLRedirects:     URLRedirectMap{"/old-page": "/test.md"},
	})

	req := httptest.NewRequest("GET", "/old-page", nil)
	w := httptest.NewRecorder()
	handler.ServeContent(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if loc := w.Header().Get("Location"); loc != "/test.md" {
		t.Errorf("Location = %q, want %q", loc, "/test.md")
	}
}

func TestHandlerResolverFallback(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"guide.md":      &fstest.MapFile{Data: []byte("# Guide")},
		"docs/intro.md": &fstest.MapFile{Data: []byte("# Intro")},
	}

	resolver := resolve.Build(files, []string{".md"}, func(mimeType string) bool {
		return mimeType == "text/markdown"
	})

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		SiteConfig:       &siteConfig,
		Resolver:         resolver,
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		shouldContain  string
	}{
		{
			name:           "extensionless path resolves via resolver",
			path:           "/guide",
			expectedStatus: 200,
			shouldContain:  "Guide",
		},
		{
			name:           "nested extensionless path resolves",
			path:           "/docs/intro",
			expectedStatus: 200,
			shouldContain:  "Intro",
		},
		{
			name:           "unknown extensionless path returns 404",
			path:           "/nonexistent",
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}
			if tt.shouldContain != "" && !strings.Contains(w.Body.String(), tt.shouldContain) {
				t.Errorf("response should contain %q, got %q", tt.shouldContain, w.Body.String())
			}
		})
	}
}

func TestHandlerDirectoryRedirect_EmptyDir(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"empty/.keep": &fstest.MapFile{Data: []byte("")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	redirectFinder := redirectFinderFromFS(files, "README.md")
	handler := NewHandler(HandlerConfig{
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
		SiteConfig:       &siteConfig,
		RedirectFinder:   redirectFinder,
	})

	req := httptest.NewRequest("GET", "/empty", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	// Empty dir with no markdown files → still 403
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}
