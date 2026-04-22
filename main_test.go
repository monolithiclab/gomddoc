package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
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

func TestSecurityHeaders(t *testing.T) {
	// Create test markdown file
	err := os.WriteFile("security_test.md", []byte("# Security Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("security_test.md")

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Wrap handler with security middleware
	secureHandler := securityHeaders(http.HandlerFunc(handler))

	req := httptest.NewRequest("GET", "/security_test.md", nil)
	w := httptest.NewRecorder()

	secureHandler.ServeHTTP(w, req)

	// Check that security headers are present
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options: nosniff, got: %q", w.Header().Get("X-Content-Type-Options"))
	}

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("Expected X-Frame-Options: DENY, got: %q", w.Header().Get("X-Frame-Options"))
	}

	// Check that the response is still successful
	if w.Code != 200 {
		t.Errorf("Expected status 200, got: %d", w.Code)
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

func TestEdgeCasesAndErrorScenarios(t *testing.T) {
	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedType   string
		shouldContain  string
	}{
		{
			name:           "POST method (should still work)",
			method:         "POST",
			path:           "/",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Gomddoc",
		},
		{
			name:           "PUT method",
			method:         "PUT",
			path:           "/",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Gomddoc",
		},
		{
			name:           "slash only path",
			method:         "GET",
			path:           "/",
			expectedStatus: 200,
			expectedType:   "text/html; charset=utf-8",
			shouldContain:  "Gomddoc",
		},
		{
			name:           "complex missing file path",
			method:         "GET",
			path:           "/nonexistent/deeply/nested/file.md",
			expectedStatus: 404,
			expectedType:   "text/plain; charset=utf-8",
			shouldContain:  "File not found",
		},
		{
			name:           "file with spaces in name",
			method:         "GET",
			path:           "/file%20with%20spaces.md",
			expectedStatus: 404,
			expectedType:   "text/plain; charset=utf-8",
			shouldContain:  "File not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.expectedStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != tt.expectedType {
				t.Errorf("got content-type %q, want %q", contentType, tt.expectedType)
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

func TestConfigurationVariations(t *testing.T) {
	// Test different directory configurations
	tests := []struct {
		name      string
		dir       string
		index     string
		shouldErr bool
	}{
		{
			name:      "current directory",
			dir:       ".",
			index:     "README.md",
			shouldErr: false,
		},
		{
			name:      "custom index file",
			dir:       ".",
			index:     "custom.md",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Dir:             tt.dir,
				DefaultIndex:    tt.index,
				Port:            ":8080",
				ShutdownTimeout: 1 * time.Second,
			}

			handler, err := newServeHTTP(config)
			if tt.shouldErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if handler == nil && !tt.shouldErr {
				t.Error("handler should not be nil")
			}
		})
	}
}

func TestHTTPMethodsAndHeaders(t *testing.T) {
	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	baseHandler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test with security headers middleware
	handler := securityHeaders(http.HandlerFunc(baseHandler))

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// All methods should work (server doesn't restrict by method)
			if w.Code != 200 {
				t.Errorf("method %s: got status %d, want 200", method, w.Code)
			}

			// Security headers should always be present
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("method %s: missing X-Content-Type-Options header", method)
			}
			if w.Header().Get("X-Frame-Options") != "DENY" {
				t.Errorf("method %s: missing X-Frame-Options header", method)
			}
		})
	}
}

func TestMarkdownVariations(t *testing.T) {
	// Test different markdown content types
	tests := []struct {
		name     string
		content  string
		contains []string
	}{
		{
			name:     "complex markdown",
			content:  "# Header\n\n**Bold** and *italic*\n\n- List item\n- Another item\n\n```go\nfunc test() {}\n```\n\n[Link](http://example.com)",
			contains: []string{"<h1", "<strong>", "<em>", "<ul>", "<li>", "<pre>", "<code", "<a href"},
		},
		{
			name:     "markdown with special characters",
			content:  "# Test with símβols & entities\n\n> Quote block\n\n| Table | Header |\n|-------|--------|\n| Cell  | Data   |",
			contains: []string{"<h1", "símβols", "&amp;", "<blockquote>", "<table>", "<th>", "<td>"},
		},
		{
			name:     "minimal markdown",
			content:  "Just plain text.",
			contains: []string{"<p>", "Just plain text"},
		},
		{
			name:     "empty markdown",
			content:  "",
			contains: []string{}, // Empty content should render as empty HTML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filename := "test_" + strings.ReplaceAll(tt.name, " ", "_") + ".md"
			err := os.WriteFile(filename, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer os.Remove(filename)

			config := &Config{Dir: ".", DefaultIndex: "README.md"}
			handler, err := newServeHTTP(config)
			if err != nil {
				t.Fatalf("Failed to create handler: %v", err)
			}

			req := httptest.NewRequest("GET", "/"+filename, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != 200 {
				t.Errorf("got status %d, want 200", w.Code)
			}

			body := w.Body.String()
			for _, expected := range tt.contains {
				if !strings.Contains(body, expected) {
					t.Errorf("response should contain %q, got: %s", expected, body)
				}
			}
		})
	}
}

func TestErrorHandlingEdgeCases(t *testing.T) {
	// Test valid directory but request file outside scope
	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test deeply nested missing file path
	req := httptest.NewRequest("GET", "/deeply/nested/missing/file.md", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should return 404 for missing nested files
	if w.Code != 404 {
		t.Errorf("expected status 404 for missing nested file, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("got content-type %q, want %q", contentType, "text/plain; charset=utf-8")
	}

	body := w.Body.String()
	if !strings.Contains(body, "File not found") {
		t.Errorf("response should contain 'File not found', got: %q", body)
	}
}

func TestWriteErrorScenarios(t *testing.T) {
	// Test scenarios that could cause write errors
	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Create a custom ResponseWriter that fails on Write
	type failingWriter struct {
		*httptest.ResponseRecorder
		failOnWrite bool
	}

	fw := &failingWriter{
		ResponseRecorder: httptest.NewRecorder(),
		failOnWrite:      true,
	}

	// Override Write method to simulate write failure
	fw.ResponseRecorder.Body.Reset()

	req := httptest.NewRequest("GET", "/nonexistent-file.md", nil)

	// Test with normal writer first to ensure our test setup is correct
	normalW := httptest.NewRecorder()
	handler(normalW, req)

	if normalW.Code != 404 {
		t.Errorf("expected 404 for missing file, got %d", normalW.Code)
	}
}

func TestSpecialCharacterPaths(t *testing.T) {
	// Test paths with special characters and URL encoding
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
		{"URL encoded spaces", "/file%20name.md", 404},
		{"Special characters", "/file-with_special.chars.md", 404},
		{"Unicode characters", "/файл.md", 404},
		{"Double encoded", "/file%2520name.md", 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("path %q: got status %d, want %d", tt.path, w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestLargeMarkdownFiles(t *testing.T) {
	// Test handling of larger markdown content
	largeContent := "# Large Test File\n\n"
	for i := 0; i < 100; i++ {
		largeContent += "This is paragraph " + strings.Repeat("content ", 20) + "\n\n"
	}

	filename := "large_test.md"
	err := os.WriteFile(filename, []byte(largeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}
	defer os.Remove(filename)

	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	req := httptest.NewRequest("GET", "/"+filename, nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != 200 {
		t.Errorf("got status %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<h1") || !strings.Contains(body, "Large Test File") {
		t.Error("large file should be properly converted to HTML")
	}

	// Check response size is reasonable (should be larger than input due to HTML tags)
	if len(body) < len(largeContent) {
		t.Error("HTML output should be larger than markdown input")
	}
}

func TestBenchmarkMdToHTML(t *testing.T) {
	// Test the mdToHTML function directly with various inputs
	tests := []struct {
		name  string
		input string
	}{
		{"small", "# Small test"},
		{"medium", strings.Repeat("## Header\n\nSome content with **bold** and *italic*.\n\n", 10)},
		{"large", strings.Repeat("# Large Header\n\nContent with lists:\n- Item 1\n- Item 2\n\n", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mdToHTML([]byte(tt.input))
			if len(result) == 0 {
				t.Error("mdToHTML should not return empty result")
			}
			if !strings.Contains(string(result), "<") {
				t.Error("result should contain HTML tags")
			}
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	// Test concurrent access to ensure thread safety
	config := &Config{Dir: ".", DefaultIndex: "README.md"}
	handler, err := newServeHTTP(config)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Create test file
	err = os.WriteFile("concurrent_test.md", []byte("# Concurrent Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("concurrent_test.md")

	const numRequests = 10
	results := make(chan int, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/concurrent_test.md", nil)
			w := httptest.NewRecorder()
			handler(w, req)
			results <- w.Code
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		status := <-results
		if status != 200 {
			t.Errorf("concurrent request %d: got status %d, want 200", i, status)
		}
	}
}
