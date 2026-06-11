package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/resolve"
)

// TestIntegration_ContentServing verifies all content types are served correctly
func TestIntegration_ContentServing(t *testing.T) {
	t.Parallel()

	// Create comprehensive in-memory filesystem - NO disk I/O!
	files := fstest.MapFS{
		"README.md":          &fstest.MapFile{Data: []byte("# Project\n\nMain docs.")},
		"test.html":          &fstest.MapFile{Data: []byte("<h1>Raw HTML</h1><p>Content</p>")},
		"style.css":          &fstest.MapFile{Data: []byte("body { margin: 0; }")},
		"script.js":          &fstest.MapFile{Data: []byte("console.log('test');")},
		"docs/guide.md":      &fstest.MapFile{Data: []byte("# Guide\n\nInstructions.")},
		"docs/README.md":     &fstest.MapFile{Data: []byte("# Docs\n\nDocumentation.")},
		"assets/style.css":   &fstest.MapFile{Data: []byte("body { margin: 0; }")},
		"files/document.md":  &fstest.MapFile{Data: []byte("# Doc")},
		"files/image.png":    &fstest.MapFile{Data: []byte{0x89, 0x50}},
		"files/nested/.keep": &fstest.MapFile{Data: []byte("")},
	}

	tests := []struct {
		name             string
		path             string
		acceptHeader     string
		dirIndex         bool
		expectedStatus   int
		expectedType     string
		shouldContain    []string
		shouldNotContain []string
	}{
		// CSS and JS passthrough
		{
			name:           "CSS file passthrough",
			path:           "/style.css",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/css; charset=utf-8",
			shouldContain:  []string{"body { margin: 0; }"},
		},
		{
			name:           "JS file passthrough",
			path:           "/script.js",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/javascript; charset=utf-8",
			shouldContain:  []string{"console.log('test');"},
		},
		{
			name:           "nested CSS file",
			path:           "/assets/style.css",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			expectedType:   "text/css; charset=utf-8",
			shouldContain:  []string{"margin"},
		},

		// HTML wrapping
		{
			name:           "HTML file wrapped in template",
			path:           "/test.html",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  []string{"<!DOCTYPE html>", "<h1>Raw HTML</h1>"},
		},

		// Markdown rendering
		{
			name:           "root markdown file",
			path:           "/README.md",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  []string{"Project", "<h1"},
		},
		{
			name:           "nested markdown file",
			path:           "/docs/guide.md",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  []string{"Guide", "Instructions"},
		},

		// Directory with README
		{
			name:           "directory serves README",
			path:           "/docs",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  []string{"Docs", "Documentation"},
		},
		{
			name:           "root directory serves README",
			path:           "/",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  []string{"Main docs"},
		},

		// 404 errors
		{
			name:           "404 not found",
			path:           "/nonexistent.md",
			acceptHeader:   "text/html",
			expectedStatus: 404,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  []string{"404"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			prov := newMemoryProvider(files, "README.md", tt.dirIndex)
			registry := setupTestRegistry()
			rend := setupTestRenderer()
			handler := NewHandler(HandlerConfig{
				Provider:         prov,
				Registry:         registry,
				EnricherRegistry: setupTestEnricherRegistry(),
				TemplateRenderer: rend,
				SiteConfig:       &siteConfig,
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Accept", tt.acceptHeader)
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if contentType := w.Header().Get("Content-Type"); contentType != tt.expectedType {
				t.Errorf("content-type = %q, want %q", contentType, tt.expectedType)
			}

			body := w.Body.String()
			for _, want := range tt.shouldContain {
				if !strings.Contains(body, want) {
					t.Errorf("response should contain %q", want)
				}
			}

			for _, notWant := range tt.shouldNotContain {
				if strings.Contains(body, notWant) {
					t.Errorf("response should not contain %q", notWant)
				}
			}

			// Successful responses should have required headers
			if tt.expectedStatus == 200 {
				if cacheControl := w.Header().Get("Cache-Control"); cacheControl == "" {
					t.Error("Cache-Control header should be set")
				}
				if contentLength := w.Header().Get("Content-Length"); contentLength == "" {
					t.Error("Content-Length header should be set")
				}
				// Content-negotiated responses must include Vary: Accept
				varyValues := w.Header().Values("Vary")
				hasVaryAccept := slices.Contains(varyValues, "Accept")
				if !hasVaryAccept {
					t.Errorf("Vary header %v should contain Accept", varyValues)
				}
			}
		})
	}
}

// TestIntegration_DirectoryListing verifies directory listing behavior
func TestIntegration_DirectoryListing(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"public/file1.md":     &fstest.MapFile{Data: []byte("# File 1")},
		"public/file2.txt":    &fstest.MapFile{Data: []byte("text")},
		"public/nested/.keep": &fstest.MapFile{Data: []byte("")},
		"private/.keep":       &fstest.MapFile{Data: []byte("")},
	}

	tests := []struct {
		name           string
		path           string
		dirIndex       bool
		expectedStatus int
		shouldContain  []string
	}{
		{
			name:           "directory listing enabled",
			path:           "/public",
			dirIndex:       true,
			expectedStatus: 200,
			shouldContain:  []string{"Index", "file1.md", "file2.txt", "nested"},
		},
		{
			name:           "directory listing disabled - 403",
			path:           "/private",
			dirIndex:       false,
			expectedStatus: 403,
			shouldContain:  []string{"403 Forbidden"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			prov := newMemoryProvider(files, "README.md", tt.dirIndex)
			registry := setupTestRegistry()
			rend := setupTestRenderer()
			handler := NewHandler(HandlerConfig{
				Provider:         prov,
				Registry:         registry,
				EnricherRegistry: setupTestEnricherRegistry(),
				TemplateRenderer: rend,
				SiteConfig:       &siteConfig,
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Accept", "text/html")
			w := httptest.NewRecorder()

			handler.ServeContent(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			body := w.Body.String()
			for _, want := range tt.shouldContain {
				if !strings.Contains(body, want) {
					t.Errorf("response should contain %q", want)
				}
			}
		})
	}
}

// TestIntegration_ContextCancellation verifies context cancellation handling
func TestIntegration_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Large content to ensure processing takes time
	largeContent := strings.Repeat("# Heading\n\nParagraph.\n\n", 1000)
	files := fstest.MapFS{
		"large.md": &fstest.MapFile{Data: []byte(largeContent)},
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

	// Create request with already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest("GET", "/large.md", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeContent(w, req)

	// Should return error status (499 for client closed request or 504 for timeout)
	if w.Code != 499 && w.Code != 504 {
		t.Errorf("got status %d, expected 499 or 504 for context cancellation", w.Code)
	}
}

// TestIntegration_ExtensionStripping verifies end-to-end URL extension stripping behavior
func TestIntegration_ExtensionStripping(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"guide.md":      &fstest.MapFile{Data: []byte("# Guide\n\nUser guide content.")},
		"docs/intro.md": &fstest.MapFile{Data: []byte("# Introduction\n\nGetting started.")},
		"image.jpg":     &fstest.MapFile{Data: []byte{0xFF, 0xD8, 0xFF}}, // JPEG header
		"style.css":     &fstest.MapFile{Data: []byte("body { margin: 0; }")},
	}

	// Build resolver for extension stripping
	resolver := resolve.Build(files, []string{".md"}, func(mimeType string) bool {
		return mimeType == "text/markdown"
	})

	// Set up handler with resolver
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

	// Wrap handler with extension redirect middleware
	middleware := ExtensionRedirect(resolver, []string{".md"}, "")
	wrappedHandler := middleware(http.HandlerFunc(handler.ServeContent))

	tests := []struct {
		name           string
		path           string
		acceptHeader   string
		expectedStatus int
		expectedLoc    string
		shouldContain  string
	}{
		{
			name:           "extensionless path resolves and renders",
			path:           "/guide",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			shouldContain:  "User guide content",
		},
		{
			name:           "nested extensionless path resolves",
			path:           "/docs/intro",
			acceptHeader:   "text/html",
			expectedStatus: 200,
			shouldContain:  "Getting started",
		},
		{
			name:           "md extension redirects to extensionless",
			path:           "/guide.md",
			acceptHeader:   "text/html",
			expectedStatus: 301,
			expectedLoc:    "/guide",
		},
		{
			name:           "nested md extension redirects",
			path:           "/docs/intro.md",
			acceptHeader:   "text/html",
			expectedStatus: 301,
			expectedLoc:    "/docs/intro",
		},
		{
			name:           "non-stripped extension serves normally",
			path:           "/image.jpg",
			acceptHeader:   "*/*",
			expectedStatus: 200,
		},
		{
			name:           "css file serves normally",
			path:           "/style.css",
			acceptHeader:   "*/*",
			expectedStatus: 200,
			shouldContain:  "margin",
		},
		{
			name:           "nonexistent extensionless path returns 404",
			path:           "/nonexistent",
			acceptHeader:   "text/html",
			expectedStatus: 404,
		},
		{
			name:           "nonexistent md file returns 404",
			path:           "/nonexistent.md",
			acceptHeader:   "text/html",
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Accept", tt.acceptHeader)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.expectedLoc != "" {
				if loc := w.Header().Get("Location"); loc != tt.expectedLoc {
					t.Errorf("Location = %q, want %q", loc, tt.expectedLoc)
				}
			}

			if tt.shouldContain != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.shouldContain) {
					t.Errorf("response should contain %q", tt.shouldContain)
				}
			}
		})
	}
}
