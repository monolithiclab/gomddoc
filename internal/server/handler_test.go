package server

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/processor"
	"github.com/monolithiclab/gomddoc/internal/provider"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

func setupTestRenderer() *tmpl.HTMLRenderer {
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Site.Meta.Title}}</title></head>
<body>{{.Page.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layout.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")

	return tmpl.NewHTMLRenderer(siteConfig, testFS)
}

func TestNewHandler(t *testing.T) {
	// Create mock dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	proc := processor.NewMarkdownProcessor()
	rend := setupTestRenderer()

	handler := NewHandler(siteConfig, prov, proc, rend)
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}
}

func TestHandlerServeMarkdown(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test_handler.md", []byte("# Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_handler.md")

	// Setup dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.test.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	proc := processor.NewMarkdownProcessor()
	rend := setupTestRenderer()
	handler := NewHandler(siteConfig, prov, proc, rend)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedType   string
	}{
		{"valid markdown", "/test_handler.md", 200, "text/html; charset=utf-8"},
		{"missing file", "/missing.md", 404, "text/plain; charset=utf-8"},
		{"root path", "/", 200, "text/html; charset=utf-8"}, // Should serve README.test.md
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeMarkdown(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expectedType {
				t.Errorf("got content type %q, want %q", contentType, tt.expectedType)
			}
		})
	}
}

func TestHandlerErrorResponses(t *testing.T) {
	// Setup dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.test.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	proc := processor.NewMarkdownProcessor()
	rend := setupTestRenderer()
	handler := NewHandler(siteConfig, prov, proc, rend)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		shouldContain  string
	}{
		{
			name:           "404 error",
			path:           "/nonexistent.md",
			expectedStatus: 404,
			shouldContain:  "File not found",
		},
		{
			name:           "complex missing file path",
			path:           "/nonexistent/deeply/nested/file.md",
			expectedStatus: 404,
			shouldContain:  "File not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeMarkdown(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.shouldContain != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.shouldContain) {
					t.Errorf("response should contain %q, got: %q", tt.shouldContain, body)
				}
			}
		})
	}
}

func TestHandlerCacheHeaders(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test_cache_handler.md", []byte("# Cache Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_cache_handler.md")

	// Setup dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.test.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	proc := processor.NewMarkdownProcessor()
	rend := setupTestRenderer()
	handler := NewHandler(siteConfig, prov, proc, rend)

	req := httptest.NewRequest("GET", "/test_cache_handler.md", nil)
	w := httptest.NewRecorder()

	handler.ServeMarkdown(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	cacheControl := w.Header().Get("Cache-Control")
	expectedCache := "public, max-age=300"
	if cacheControl != expectedCache {
		t.Errorf("got cache control %q, want %q", cacheControl, expectedCache)
	}
}
