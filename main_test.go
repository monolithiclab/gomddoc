package main

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMarkdownHandler(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test.md", []byte("# Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test.md")

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"valid markdown", "/test.md", 200},
		{"missing file", "/missing.md", 404},
		{"root path", "/", 200}, // Should serve README.md
	}

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestMarkdownHandlerContentType(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test_content.md", []byte("# Test Content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_content.md")

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	tests := []struct {
		name                string
		path                string
		expectedStatus      int
		expectedContentType string
	}{
		{"valid markdown html response", "/test_content.md", 200, "text/html; charset=utf-8"},
		{"404 error text response", "/missing.md", 404, "text/plain; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expectedContentType {
				t.Errorf("got content type %q, want %q", contentType, tt.expectedContentType)
			}
		})
	}
}

func TestMarkdownHTMLConversion(t *testing.T) {
	// Create test markdown file with specific content
	markdownContent := "# Test Header\n\nThis is a test paragraph."
	err := os.WriteFile("test_html.md", []byte(markdownContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_html.md")

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest("GET", "/test_html.md", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want %d", w.Code, 200)
	}

	body := w.Body.String()

	// Check that markdown was converted to HTML
	if !strings.Contains(body, "<h1") {
		t.Error("Expected HTML h1 tag in response")
	}

	if !strings.Contains(body, "<p>") {
		t.Error("Expected HTML p tag in response")
	}

	// Check that raw markdown syntax is not present
	if strings.Contains(body, "# Test Header") {
		t.Error("Expected markdown syntax to be converted, found raw markdown")
	}
}

func TestMarkdownHandlerCacheHeaders(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("test_cache.md", []byte("# Cache Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_cache.md")

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest("GET", "/test_cache.md", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want %d", w.Code, 200)
	}

	cacheControl := w.Header().Get("Cache-Control")
	expectedCache := "public, max-age=300"
	if cacheControl != expectedCache {
		t.Errorf("got cache control %q, want %q", cacheControl, expectedCache)
	}
}

func TestMdToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "basic header",
			input:    "# Test Header",
			contains: []string{"<h1", "Test Header", "</h1>"},
		},
		{
			name:     "paragraph",
			input:    "This is a paragraph.",
			contains: []string{"<p>", "This is a paragraph.", "</p>"},
		},
		{
			name:     "code block",
			input:    "```go\nfunc main() {}\n```",
			contains: []string{"<pre>", "<code", "func main()"},
		},
		{
			name:     "link",
			input:    "[link text](http://example.com)",
			contains: []string{"<a", "href=\"http://example.com\"", "link text", "</a>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mdToHTML([]byte(tt.input))
			resultStr := string(result)

			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected HTML to contain %q, got: %s", expected, resultStr)
				}
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	// Save original args to restore later
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test default values
	os.Args = []string{"cmd"}
	config := newConfig()

	if config.Dir != "." {
		t.Errorf("Expected default dir '.', got %q", config.Dir)
	}

	if config.Port != ":8080" {
		t.Errorf("Expected default port ':8080', got %q", config.Port)
	}

	if config.DefaultIndex != "README.md" {
		t.Errorf("Expected default index 'README.md', got %q", config.DefaultIndex)
	}

	if config.ShutdownTimeout.Seconds() != 1 {
		t.Errorf("Expected shutdown timeout 1s, got %v", config.ShutdownTimeout)
	}
}

func TestPathCleaning(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("path_test.md", []byte("# Path Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("path_test.md")

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"normal path", "/path_test.md", 200},
		{"path with dots", "/./path_test.md", 200},
		{"path with double slash", "//path_test.md", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d for path %q", w.Code, tt.expectedStatus, tt.path)
			}
		})
	}
}
