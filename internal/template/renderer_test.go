package template

import (
	"html/template"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

// testFSProvider wraps an fs.FS to implement provider.Provider for testing
type testFSProvider struct {
	fsys         fs.FS
	defaultIndex string
}

// Ensure testFSProvider implements provider.Provider interface
var _ provider.Provider = (*testFSProvider)(nil)

// newTestFSProvider creates a new test provider from a filesystem
func newTestFSProvider(fsys fs.FS, defaultIndex string) *testFSProvider {
	return &testFSProvider{
		fsys:         fsys,
		defaultIndex: defaultIndex,
	}
}

func (t *testFSProvider) ReadFile(path string) ([]byte, error) {
	// Remove leading slash to make it relative for fs.FS
	path = strings.TrimPrefix(path, "/")
	return fs.ReadFile(t.fsys, path)
}

func (t *testFSProvider) Stat(path string) (fs.FileInfo, error) {
	// Remove leading slash to make it relative for fs.FS
	if path == "/" {
		path = "."
	} else {
		path = strings.TrimPrefix(path, "/")
	}
	return fs.Stat(t.fsys, path)
}

func (t *testFSProvider) DefaultIndex() string {
	return t.defaultIndex
}

func TestNewHTMLRenderer(t *testing.T) {
	renderer := NewHTMLRenderer(nil)
	if renderer == nil {
		t.Fatal("Renderer should not be nil")
	}
}

func TestTemplateContext(t *testing.T) {
	ctx := &TemplateContext{
		Title:       "Test Title",
		Description: "Test Description",
		Content:     template.HTML("<p>Test content</p>"),
		Theme: ThemeOptions{
			BaseURL: "/assets/themes/default",
		},
		Breadcrumbs: map[string]string{
			"/":     "Home",
			"/test": "Test",
		},
	}

	if ctx.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got %q", ctx.Title)
	}

	if ctx.Description != "Test Description" {
		t.Errorf("Expected description 'Test Description', got %q", ctx.Description)
	}

	if string(ctx.Content) != "<p>Test content</p>" {
		t.Errorf("Expected content '<p>Test content</p>', got %q", string(ctx.Content))
	}

	if ctx.Theme.BaseURL != "/assets/themes/default" {
		t.Errorf("Expected theme base URL '/assets/themes/default', got %q", ctx.Theme.BaseURL)
	}

	if len(ctx.Breadcrumbs) != 2 {
		t.Errorf("Expected 2 breadcrumbs, got %d", len(ctx.Breadcrumbs))
	}

	if ctx.Breadcrumbs["/"] != "Home" {
		t.Errorf("Expected breadcrumb '/' to be 'Home', got %q", ctx.Breadcrumbs["/"])
	}

	if ctx.Breadcrumbs["/test"] != "Test" {
		t.Errorf("Expected breadcrumb '/test' to be 'Test', got %q", ctx.Breadcrumbs["/test"])
	}
}

func TestHTMLRendererRender(t *testing.T) {
	// Create an in-memory filesystem with the template
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>{{.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layout.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	renderer := NewHTMLRenderer(testFS)

	ctx := &TemplateContext{
		Title:       "Test Page",
		Description: "A test page",
		Content:     template.HTML("<h1>Hello World</h1>"),
		Theme: ThemeOptions{
			BaseURL: "/assets/themes/default",
		},
		Breadcrumbs: map[string]string{
			"/": "Home",
		},
	}

	result, err := renderer.Render("layout.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	resultStr := string(result)

	// Check that the template was rendered with our data
	if !strings.Contains(resultStr, "Test Page") {
		t.Error("Expected rendered template to contain title 'Test Page'")
	}

	if !strings.Contains(resultStr, "<h1>Hello World</h1>") {
		t.Error("Expected rendered template to contain content '<h1>Hello World</h1>'")
	}

	if !strings.Contains(resultStr, "<!DOCTYPE html>") {
		t.Error("Expected rendered template to be valid HTML")
	}
}

func TestTemplateCache(t *testing.T) {
	// Create test filesystem
	templateContent := `<h1>{{.Title}}</h1>`
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	renderer := NewHTMLRenderer(testFS)

	ctx := &TemplateContext{
		Title: "Test",
		Breadcrumbs: map[string]string{
			"/": "Home",
		},
	}

	// First render - should parse and cache
	result1, err := renderer.Render("test.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	// Second render - should use cache
	result2, err := renderer.Render("test.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Second render failed: %v", err)
	}

	// Results should be identical
	if string(result1) != string(result2) {
		t.Errorf("Cached template result differs: %q vs %q", string(result1), string(result2))
	}

	if !strings.Contains(string(result1), "Test") {
		t.Error("Expected rendered template to contain 'Test'")
	}
}

func TestGenerateBreadcrumbs(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		fsys     fs.FS
		expected map[string]string
	}{
		{
			name:     "root file without leading slash",
			filePath: "README.md",
			fsys: fstest.MapFS{
				"README.md": {Data: []byte("# Readme")},
			},
			expected: map[string]string{
				"/":          "Home",
				"/README.md": "Readme",
			},
		},
		{
			name:     "multi levels deep file",
			filePath: "references/subscription/overview.md",
			fsys: fstest.MapFS{
				"references/subscription/overview.md": {Data: []byte("# Overview")},
			},
			expected: map[string]string{
				"/":                                    "Home",
				"/references/":                         "References",
				"/references/subscription/":            "Subscription",
				"/references/subscription/overview.md": "Overview",
			},
		},
		{
			name:     "path with leading slash for file",
			filePath: "/docs/guide/setup.md",
			fsys: fstest.MapFS{
				"docs/guide/setup.md": {Data: []byte("# Setup Guide")},
			},
			expected: map[string]string{
				"/":                    "Home",
				"/docs/":               "Docs",
				"/docs/guide/":         "Guide",
				"/docs/guide/setup.md": "Setup",
			},
		},
		{
			name:     "user example: file path",
			filePath: "/howtos/core/test.md",
			fsys: fstest.MapFS{
				"howtos/core/test.md": {Data: []byte("# Test Instructions")},
			},
			expected: map[string]string{
				"/":                    "Home",
				"/howtos/":             "Howtos",
				"/howtos/core/":        "Core",
				"/howtos/core/test.md": "Test",
			},
		},
		{
			name:     "user example: directory path without trailing slash",
			filePath: "/howtos/core",
			fsys: fstest.MapFS{
				"howtos/core/README.md": {Data: []byte("# Core Documentation")},
			},
			expected: map[string]string{
				"/":             "Home",
				"/howtos/":      "Howtos",
				"/howtos/core/": "Core",
			},
		},
		{
			name:     "user example: directory path with trailing slash",
			filePath: "/howtos/core/",
			fsys: fstest.MapFS{
				"howtos/core/index.md": {Data: []byte("# Core Index")},
			},
			expected: map[string]string{
				"/":             "Home",
				"/howtos/":      "Howtos",
				"/howtos/core/": "Core",
			},
		},
		{
			name:     "empty path edge case",
			filePath: "",
			fsys:     fstest.MapFS{},
			expected: map[string]string{
				"/": "Home",
			},
		},
		{
			name:     "root directory with trailing slash",
			filePath: "/",
			fsys:     fstest.MapFS{},
			expected: map[string]string{
				"/": "Home",
			},
		},
		{
			name:     "excessively long path (DoS protection)",
			filePath: "/" + strings.Repeat("a/", 1100) + "file.md", // ~3300 chars before clean, exceeds 2048 limit
			fsys:     fstest.MapFS{},
			expected: map[string]string{
				"/": "Home", // Should only return root breadcrumb
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newTestFSProvider(tt.fsys, "README.md")
			result := GenerateBreadcrumbs(provider, tt.filePath)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d breadcrumbs, got %d", len(tt.expected), len(result))
				t.Errorf("Expected: %v", tt.expected)
				t.Errorf("Got: %v", result)
				return
			}

			for expectedPath, expectedName := range tt.expected {
				if actualName, exists := result[expectedPath]; !exists {
					t.Errorf("Expected breadcrumb path %q not found", expectedPath)
				} else if actualName != expectedName {
					t.Errorf("For path %q, expected name %q, got %q", expectedPath, expectedName, actualName)
				}
			}
		})
	}
}

func BenchmarkGenerateBreadcrumbs(b *testing.B) {
	testPaths := []string{
		"README.md",
		"docs/overview.md",
		"references/subscription/billing/overview.md",
		"documentation/tutorials/advanced/configuration/settings.md",
		"api/v1/users/create.md",
	}

	// Create a filesystem with all test paths as files
	fsys := fstest.MapFS{
		"README.md":        {Data: []byte("# Readme")},
		"docs/overview.md": {Data: []byte("# Overview")},
		"references/subscription/billing/overview.md":                {Data: []byte("# Billing Overview")},
		"documentation/tutorials/advanced/configuration/settings.md": {Data: []byte("# Settings")},
		"api/v1/users/create.md":                                     {Data: []byte("# Create User")},
	}

	provider := newTestFSProvider(fsys, "README.md")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			GenerateBreadcrumbs(provider, path)
		}
	}
}
