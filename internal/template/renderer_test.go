package template

import (
	"context"
	"html/template"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
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

func (t *testFSProvider) ReadFile(path string) ([]byte, string, error) {
	// Remove leading slash to make it relative for fs.FS
	path = strings.TrimPrefix(path, "/")
	content, err := fs.ReadFile(t.fsys, path)
	if err != nil {
		return nil, "", err
	}
	// Return generic text/plain MIME type for tests
	return content, "text/plain", nil
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

func (t *testFSProvider) Close() error {
	return nil
}

func TestNewHTMLRenderer(t *testing.T) {
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte("<html></html>"),
		},
	}
	siteConfig := config.NewSiteConfig(".")

	// Test with explicit cache
	cache := &CachedTemplateStore{}
	renderer := NewHTMLRenderer(siteConfig, testFS, WithCache(cache))
	if renderer == nil {
		t.Fatal("Renderer should not be nil")
	}

	// Test with default cache (no options)
	renderer2 := NewHTMLRenderer(siteConfig, testFS)
	if renderer2 == nil {
		t.Fatal("Renderer should not be nil")
	}
	if renderer2.cache == nil {
		t.Fatal("Cache should default to PassthroughTemplateStore")
	}
}

func TestTemplateContext(t *testing.T) {
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Title"
	siteConfig.Meta.Description = "Test Description"

	ctx := &TemplateContext{
		Site: siteConfig,
		Page: PageContext{
			Content: template.HTML("<p>Test content</p>"),
			Breadcrumbs: []Breadcrumb{
				{"/", "Home"},
				{"/test", "Test"},
			},
		},
	}

	if ctx.Site.Meta.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got %q", ctx.Site.Meta.Title)
	}

	if ctx.Site.Meta.Description != "Test Description" {
		t.Errorf("Expected description 'Test Description', got %q", ctx.Site.Meta.Description)
	}

	if string(ctx.Page.Content) != "<p>Test content</p>" {
		t.Errorf("Expected content '<p>Test content</p>', got %q", string(ctx.Page.Content))
	}

	if len(ctx.Page.Breadcrumbs) != 2 {
		t.Errorf("Expected 2 breadcrumbs, got %d", len(ctx.Page.Breadcrumbs))
	}

	if ctx.Page.Breadcrumbs[0].Path != "/" || ctx.Page.Breadcrumbs[0].Label != "Home" {
		t.Errorf("Expected first breadcrumb to be {'/','Home'}, got {%q,%q}", ctx.Page.Breadcrumbs[0].Path, ctx.Page.Breadcrumbs[0].Label)
	}

	if ctx.Page.Breadcrumbs[1].Path != "/test" || ctx.Page.Breadcrumbs[1].Label != "Test" {
		t.Errorf("Expected second breadcrumb to be {'/test','Test'}, got {%q,%q}", ctx.Page.Breadcrumbs[1].Path, ctx.Page.Breadcrumbs[1].Label)
	}
}

func TestHTMLRendererRender(t *testing.T) {
	// Create an in-memory filesystem with the template
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
	siteConfig.Meta.Title = "Test Page"
	cache := &CachedTemplateStore{}
	renderer := NewHTMLRenderer(siteConfig, testFS, WithCache(cache))

	ctx := &TemplateContext{
		Site: siteConfig,
		Page: PageContext{
			Content: template.HTML("<h1>Hello World</h1>"),
			Breadcrumbs: []Breadcrumb{
				{"/", "Home"},
			},
		},
	}

	result, err := renderer.Render(context.Background(), "layout.html.tmpl", ctx)
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
	templateContent := `<h1>{{.Site.Meta.Title}}</h1>`
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test"
	cache := &CachedTemplateStore{} // Production mode with caching
	renderer := NewHTMLRenderer(siteConfig, testFS, WithCache(cache))

	ctx := &TemplateContext{
		Site: siteConfig,
		Page: PageContext{
			Breadcrumbs: []Breadcrumb{
				{"/", "Home"},
			},
		},
	}

	// First render - should parse and cache
	result1, err := renderer.Render(context.Background(), "test.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	// Second render - should use cache
	result2, err := renderer.Render(context.Background(), "test.html.tmpl", ctx)
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

func TestRenderWithContextCancellation(t *testing.T) {
	// Create test filesystem
	templateContent := `<h1>{{.Site.Meta.Title}}</h1>`
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test"
	// Dev mode - no caching, always parses
	renderer := NewHTMLRenderer(siteConfig, testFS)

	ctx := &TemplateContext{
		Site: siteConfig,
		Page: PageContext{
			Breadcrumbs: []Breadcrumb{
				{"/", "Home"},
			},
		},
	}

	// Create a cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	// Attempt to render with cancelled context
	_, err := renderer.Render(cancelledCtx, "test.html.tmpl", ctx)
	if err == nil {
		t.Fatal("Expected error when rendering with cancelled context")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestGenerateBreadcrumbs(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		fsys     fs.FS
		expected []Breadcrumb
	}{
		{
			name:     "root file without leading slash",
			filePath: "README.md",
			fsys: fstest.MapFS{
				"README.md": {Data: []byte("# Readme")},
			},
			expected: []Breadcrumb{
				{"/", "Home"},
				{"/README.md", "Readme"},
			},
		},
		{
			name:     "multi levels deep file",
			filePath: "references/subscription/overview.md",
			fsys: fstest.MapFS{
				"references/subscription/overview.md": {Data: []byte("# Overview")},
			},
			expected: []Breadcrumb{
				{"/", "Home"},
				{"/references/", "References"},
				{"/references/subscription/", "Subscription"},
				{"/references/subscription/overview.md", "Overview"},
			},
		},
		{
			name:     "path with leading slash for file",
			filePath: "/docs/guide/setup.md",
			fsys: fstest.MapFS{
				"docs/guide/setup.md": {Data: []byte("# Setup Guide")},
			},
			expected: []Breadcrumb{
				{"/", "Home"},
				{"/docs/", "Docs"},
				{"/docs/guide/", "Guide"},
				{"/docs/guide/setup.md", "Setup"},
			},
		},
		{
			name:     "user example: file path",
			filePath: "/howtos/core/test.md",
			fsys: fstest.MapFS{
				"howtos/core/test.md": {Data: []byte("# Test Instructions")},
			},
			expected: []Breadcrumb{
				{"/", "Home"},
				{"/howtos/", "Howtos"},
				{"/howtos/core/", "Core"},
				{"/howtos/core/test.md", "Test"},
			},
		},
		{
			name:     "user example: directory path without trailing slash",
			filePath: "/howtos/core",
			fsys: fstest.MapFS{
				"howtos/core/README.md": {Data: []byte("# Core Documentation")},
			},
			expected: []Breadcrumb{
				{"/", "Home"},
				{"/howtos/", "Howtos"},
				{"/howtos/core/", "Core"},
			},
		},
		{
			name:     "user example: directory path with trailing slash",
			filePath: "/howtos/core/",
			fsys: fstest.MapFS{
				"howtos/core/index.md": {Data: []byte("# Core Index")},
			},
			expected: []Breadcrumb{
				{"/", "Home"},
				{"/howtos/", "Howtos"},
				{"/howtos/core/", "Core"},
			},
		},
		{
			name:     "empty path edge case",
			filePath: "",
			fsys:     fstest.MapFS{},
			expected: []Breadcrumb{
				{"/", "Home"},
			},
		},
		{
			name:     "root directory with trailing slash",
			filePath: "/",
			fsys:     fstest.MapFS{},
			expected: []Breadcrumb{
				{"/", "Home"},
			},
		},
		{
			name:     "excessively long path (DoS protection)",
			filePath: "/" + strings.Repeat("a/", 1100) + "file.md", // ~3300 chars before clean, exceeds 2048 limit
			fsys:     fstest.MapFS{},
			expected: []Breadcrumb{
				{"/", "Home"}, // Should only return root breadcrumb
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

			for i, expected := range tt.expected {
				if result[i].Path != expected.Path {
					t.Errorf("Breadcrumb %d: expected path %q, got %q", i, expected.Path, result[i].Path)
				}
				if result[i].Label != expected.Label {
					t.Errorf("Breadcrumb %d: expected label %q, got %q", i, expected.Label, result[i].Label)
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
