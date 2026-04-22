package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestHandlerServeContent_ContentNegotiation(t *testing.T) {
	t.Parallel()

	// Create in-memory filesystem - NO disk I/O!
	files := fstest.MapFS{
		"test.md": &fstest.MapFile{Data: []byte("# Test Markdown")},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, &siteConfig)

	tests := []struct {
		name           string
		path           string
		acceptHeader   string
		expectedStatus int
		expectedType   string
		shouldContain  string
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
			name:           "accept application/json - not acceptable",
			path:           "/test.md",
			acceptHeader:   "application/json",
			expectedStatus: 406,
			expectedType:   "text/plain; charset=utf-8",
			shouldContain:  "Not Acceptable",
		},
		{
			name:           "accept image/png - not acceptable",
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

			if tt.expectedStatus == 200 && w.Header().Get("Content-Length") == "" {
				t.Error("Content-Length header should be set for successful responses")
			}
		})
	}
}

func TestHandlerServeContent_BinaryFiles(t *testing.T) {
	t.Parallel()

	// In-memory PNG file - NO disk I/O!
	files := fstest.MapFS{
		"test.png": &fstest.MapFile{Data: []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}},
	}

	siteConfig := config.NewSiteConfig(".")
	prov := newMemoryProvider(files, "README.md", false)
	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, &siteConfig)

	req := httptest.NewRequest("GET", "/test.png", nil)
	req.Header.Set("Accept", "*/*")
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "image/png" {
		t.Errorf("content-type = %q, want image/png", contentType)
	}

	if cacheControl := w.Header().Get("Cache-Control"); cacheControl != "public, max-age=300" {
		t.Errorf("cache-control = %q, want public, max-age=300", cacheControl)
	}

	if len(w.Body.Bytes()) != 8 {
		t.Errorf("body length = %d, want 8", len(w.Body.Bytes()))
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
	handler := NewHandler(prov, registry, rend, &siteConfig)

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
	handler := NewHandler(prov, registry, rend, &siteConfig)

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
	handler := NewHandler(prov, registry, rend, &siteConfig)

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
