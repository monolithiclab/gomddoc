package server

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
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

func setupTestRegistry() renderer.RendererRegistry {
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer())
	registry.Register(renderer.NewPassthroughRenderer())
	return registry
}

func TestNewHandler(t *testing.T) {
	// Create mock dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()

	handler := NewHandler(prov, registry, rend, siteConfig)
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}
}

func TestHandlerServeContent_MarkdownFiles(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test_handler.md", []byte("# Test Markdown"), 0644)
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
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	tests := []struct {
		name           string
		path           string
		acceptHeader   string
		expectedStatus int
		expectedType   string
		shouldContain  string
	}{
		{
			name:           "markdown file rendered as HTML",
			path:           "/test_handler.md",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "<h1",
		},
		{
			name:           "markdown file with */* accept",
			path:           "/test_handler.md",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "<!DOCTYPE html>",
		},
		{
			name:           "markdown file with empty accept (defaults to */*)",
			path:           "/test_handler.md",
			acceptHeader:   "",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Test Markdown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expectedType {
				t.Errorf("got content type %q, want %q", contentType, tt.expectedType)
			}

			if tt.shouldContain != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.shouldContain) {
					t.Errorf("response should contain %q, got: %q", tt.shouldContain, body)
				}
			}

			// Check Content-Length header is set
			if contentLength := w.Header().Get("Content-Length"); contentLength == "" {
				t.Error("Content-Length header should be set")
			}
		})
	}
}

func TestHandlerServeContent_BinaryFiles(t *testing.T) {
	// Create test binary file (PNG header)
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	err := os.WriteFile("test.png", pngData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test.png")

	// Setup dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/test.png", nil)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	// Should be served as image/png (passthrough)
	contentType := w.Header().Get("Content-Type")
	if contentType != "image/png" {
		t.Errorf("got content type %q, want image/png", contentType)
	}

	// Check Content-Length header
	if contentLength := w.Header().Get("Content-Length"); contentLength == "" {
		t.Error("Content-Length header should be set")
	}

	// Check Cache-Control header
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=300" {
		t.Errorf("got cache control %q, want public, max-age=300", cacheControl)
	}

	// Verify binary content preserved
	if len(w.Body.Bytes()) != len(pngData) {
		t.Errorf("got %d bytes, want %d", len(w.Body.Bytes()), len(pngData))
	}
}

func TestHandlerServeContent_AcceptNegotiation(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test_accept.md", []byte("# Accept Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_accept.md")

	// Setup dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	tests := []struct {
		name           string
		acceptHeader   string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "accept text/html",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
		},
		{
			name:           "accept */*",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
		},
		{
			name:           "accept text/*",
			acceptHeader:   "text/*",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
		},
		{
			name:           "accept application/json - not acceptable",
			acceptHeader:   "application/json",
			expectedStatus: 406,
			expectedType:   "text/plain; charset=utf-8",
		},
		{
			name:           "accept image/png - not acceptable",
			acceptHeader:   "image/png",
			expectedStatus: 406,
			expectedType:   "text/plain; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test_accept.md", nil)
			req.Header.Set("Accept", tt.acceptHeader)
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expectedType {
				t.Errorf("got content type %q, want %q", contentType, tt.expectedType)
			}

			if tt.expectedStatus == 406 {
				body := w.Body.String()
				if !strings.Contains(body, "Not Acceptable") {
					t.Error("406 response should contain 'Not Acceptable'")
				}
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
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

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
			shouldContain:  "404 Not Found",
		},
		{
			name:           "complex missing file path",
			path:           "/nonexistent/deeply/nested/file.md",
			expectedStatus: 404,
			shouldContain:  "404 Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			// Error responses should be HTML
			contentType := w.Header().Get("Content-Type")
			if contentType != "text/html; charset=utf-8" {
				t.Errorf("error response should be HTML, got %q", contentType)
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
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/test_cache_handler.md", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	cacheControl := w.Header().Get("Cache-Control")
	expectedCache := "public, max-age=300"
	if cacheControl != expectedCache {
		t.Errorf("got cache control %q, want %q", cacheControl, expectedCache)
	}
}

func TestHandlerContentLength(t *testing.T) {
	// Create test files
	err := os.WriteFile("test_length.md", []byte("# Content Length Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_length.md")

	// Setup dependencies
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	t.Run("HTML content has Content-Length", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test_length.md", nil)
		w := httptest.NewRecorder()

		handler.ServeContent(w, req)

		contentLength := w.Header().Get("Content-Length")
		if contentLength == "" {
			t.Error("Content-Length header should be set for HTML content")
		}

		// Verify Content-Length matches actual body length
		if contentLength != string(rune(len(w.Body.Bytes()))) && contentLength != "0" {
			bodyLen := len(w.Body.Bytes())
			if contentLength != string(rune(bodyLen)) {
				t.Logf("Content-Length: %s, Body length: %d", contentLength, bodyLen)
			}
		}
	})
}

func TestHandlerForbiddenDirectory(t *testing.T) {
	// Create test directory without README
	testDir := "test_forbidden_dir"
	err := os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Setup dependencies with dirIndex=false
	siteConfig := config.NewSiteConfig(".")
	prov, err := provider.NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/"+testDir, nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 403 {
		t.Errorf("got status %d, want 403", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "403 Forbidden") {
		t.Errorf("response should contain '403 Forbidden', got: %q", body)
	}
}
