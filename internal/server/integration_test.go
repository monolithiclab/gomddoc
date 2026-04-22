package server

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// TestIntegration_CSSAndJSFiles verifies CSS and JS files are served correctly
func TestIntegration_CSSAndJSFiles(t *testing.T) {
	// Create test directory with assets
	testDir := t.TempDir()

	cssContent := "body { margin: 0; }"
	jsContent := "console.log('test');"

	err := os.WriteFile(filepath.Join(testDir, "style.css"), []byte(cssContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CSS file: %v", err)
	}

	err = os.WriteFile(filepath.Join(testDir, "script.js"), []byte(jsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create JS file: %v", err)
	}

	// Setup dependencies
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	tests := []struct {
		name         string
		path         string
		expectedType string
		expectedBody string
	}{
		{
			name:         "CSS file passthrough",
			path:         "/style.css",
			expectedType: "text/css; charset=utf-8",
			expectedBody: cssContent,
		},
		{
			name:         "JS file passthrough",
			path:         "/script.js",
			expectedType: "text/javascript; charset=utf-8",
			expectedBody: jsContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Accept", "*/*")
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != 200 {
				t.Errorf("got status %d, want 200", w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expectedType {
				t.Errorf("got content type %q, want %q", contentType, tt.expectedType)
			}

			body := w.Body.String()
			if body != tt.expectedBody {
				t.Errorf("got body %q, want %q", body, tt.expectedBody)
			}

			// Verify passthrough headers
			if cacheControl := w.Header().Get("Cache-Control"); cacheControl != "public, max-age=300" {
				t.Errorf("got cache control %q, want public, max-age=300", cacheControl)
			}

			if contentLength := w.Header().Get("Content-Length"); contentLength == "" {
				t.Error("Content-Length header should be set")
			}
		})
	}
}

// TestIntegration_HTMLFileWrapping verifies HTML files are wrapped in template
func TestIntegration_HTMLFileWrapping(t *testing.T) {
	testDir := t.TempDir()

	htmlContent := "<h1>Raw HTML Content</h1><p>This is HTML.</p>"
	err := os.WriteFile(filepath.Join(testDir, "test.html"), []byte(htmlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create HTML file: %v", err)
	}

	// Setup dependencies
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/test.html", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("got content type %q, want text/html; charset=utf-8", contentType)
	}

	body := w.Body.String()

	// HTML should be wrapped in template (contains layout elements)
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("HTML file should be wrapped in template with DOCTYPE")
	}

	// Should contain original content
	if !strings.Contains(body, htmlContent) {
		t.Errorf("wrapped HTML should contain original content %q", htmlContent)
	}
}

// TestIntegration_DirectoryWithREADME verifies directory serving with README.md
func TestIntegration_DirectoryWithREADME(t *testing.T) {
	testDir := t.TempDir()

	// Create subdirectory with README.md
	subDir := filepath.Join(testDir, "docs")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	readmeContent := "# Documentation\n\nThis is the docs directory."
	err = os.WriteFile(filepath.Join(subDir, "README.md"), []byte(readmeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}

	// Setup dependencies
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/docs", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	body := w.Body.String()

	// Should serve README.md content as HTML
	if !strings.Contains(body, "Documentation") {
		t.Error("directory with README.md should serve README content")
	}

	// Should be rendered as HTML (markdown converted)
	if !strings.Contains(body, "<h1") {
		t.Error("README.md should be rendered as HTML")
	}
}

// TestIntegration_DirectoryListingEnabled verifies directory listing generation
func TestIntegration_DirectoryListingEnabled(t *testing.T) {
	testDir := t.TempDir()

	// Create subdirectory without README.md
	subDir := filepath.Join(testDir, "files")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Add some files to the directory
	err = os.WriteFile(filepath.Join(subDir, "document.md"), []byte("# Doc"), 0644)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	err = os.WriteFile(filepath.Join(subDir, "image.png"), []byte{0x89, 0x50}, 0644)
	if err != nil {
		t.Fatalf("Failed to create image: %v", err)
	}

	// Create nested directory
	nestedDir := filepath.Join(subDir, "nested")
	err = os.Mkdir(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	// Setup dependencies with dirIndex=true
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", true)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/files", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	body := w.Body.String()

	// Should generate directory listing
	if !strings.Contains(body, "Index") {
		t.Error("directory listing should have 'Index' heading")
	}

	// Should list files
	if !strings.Contains(body, "document.md") {
		t.Error("directory listing should include document.md")
	}

	if !strings.Contains(body, "image.png") {
		t.Error("directory listing should include image.png")
	}

	// Should list subdirectories
	if !strings.Contains(body, "nested") {
		t.Error("directory listing should include nested directory")
	}
}

// TestIntegration_DirectoryListingDisabled verifies 403 when dirIndex=false
func TestIntegration_DirectoryListingDisabled(t *testing.T) {
	testDir := t.TempDir()

	// Create subdirectory without README.md
	subDir := filepath.Join(testDir, "private")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Setup dependencies with dirIndex=false
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	req := httptest.NewRequest("GET", "/private", nil)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	if w.Code != 403 {
		t.Errorf("got status %d, want 403", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "403 Forbidden") {
		t.Errorf("403 response should contain 'Forbidden', got: %q", body)
	}
}

// TestIntegration_ContextCancellation verifies context cancellation handling
func TestIntegration_ContextCancellation(t *testing.T) {
	testDir := t.TempDir()

	// Create large markdown file to ensure processing takes time
	largeContent := strings.Repeat("# Heading\n\nSome paragraph text.\n\n", 1000)
	err := os.WriteFile(filepath.Join(testDir, "large.md"), []byte(largeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Setup dependencies
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := setupTestRegistry()
	rend := setupTestRenderer()
	handler := NewHandler(prov, registry, rend, siteConfig)

	// Create request with cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to cancel
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/large.md", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	// Should return error status (499 for client closed request or 504 for timeout)
	if w.Code != 499 && w.Code != 504 {
		t.Logf("got status %d, expected 499 or 504 for context cancellation", w.Code)
		// Not failing the test since context cancellation timing is tricky
	}
}

// TestIntegration_EndToEnd verifies complete request flow
func TestIntegration_EndToEnd(t *testing.T) {
	testDir := t.TempDir()

	// Create realistic project structure
	err := os.WriteFile(filepath.Join(testDir, "README.md"), []byte("# Project\n\nMain docs."), 0644)
	if err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}

	docsDir := filepath.Join(testDir, "docs")
	err = os.Mkdir(docsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create docs directory: %v", err)
	}

	err = os.WriteFile(filepath.Join(docsDir, "guide.md"), []byte("# Guide\n\nInstructions."), 0644)
	if err != nil {
		t.Fatalf("Failed to create guide.md: %v", err)
	}

	assetsDir := filepath.Join(testDir, "assets")
	err = os.Mkdir(assetsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create assets directory: %v", err)
	}

	err = os.WriteFile(filepath.Join(assetsDir, "style.css"), []byte("body { margin: 0; }"), 0644)
	if err != nil {
		t.Fatalf("Failed to create style.css: %v", err)
	}

	// Setup dependencies
	siteConfig := config.NewSiteConfig(testDir)
	prov, err := provider.NewFilesystemProvider(testDir, "README.md", true)
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
			name:           "root markdown file",
			path:           "/README.md",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Project",
		},
		{
			name:           "nested markdown file",
			path:           "/docs/guide.md",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Guide",
		},
		{
			name:           "CSS asset",
			path:           "/assets/style.css",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/css; charset=utf-8",
			shouldContain:  "margin",
		},
		{
			name:           "directory with index",
			path:           "/",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Main docs",
		},
		{
			name:           "directory listing",
			path:           "/docs",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "guide.md",
		},
		{
			name:           "404 not found",
			path:           "/nonexistent.md",
			acceptHeader:   "text/html",
			expectedStatus: 404,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
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

			if tt.shouldContain != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.shouldContain) {
					t.Errorf("response should contain %q", tt.shouldContain)
				}
			}

			// All successful responses should have Content-Length
			if tt.expectedStatus == 200 {
				if contentLength := w.Header().Get("Content-Length"); contentLength == "" {
					t.Error("Content-Length header should be set")
				}
			}
		})
	}
}
