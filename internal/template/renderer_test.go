package template

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

func TestNewHTMLRenderer(t *testing.T) {
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte("<html></html>"),
		},
	}
	siteConfig := config.NewSiteConfig(".")

	// Test with explicit cache
	cache := &CachedTemplateStore{}
	renderer := NewHTMLRenderer(&siteConfig, testFS, WithCache(cache))
	if renderer == nil {
		t.Fatal("Renderer should not be nil")
	}

	// Test with default cache (no options)
	renderer2 := NewHTMLRenderer(&siteConfig, testFS)
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
		Site: &siteConfig,
		Page: PageContext{
			Content: template.HTML("<p>Test content</p>"),
			Breadcrumbs: []breadcrumb.Breadcrumb{
				{Path: "/", Label: "Home"},
				{Path: "/test", Label: "Test"},
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
	renderer := NewHTMLRenderer(&siteConfig, testFS, WithCache(cache))

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Content: template.HTML("<h1>Hello World</h1>"),
			Breadcrumbs: []breadcrumb.Breadcrumb{
				{Path: "/", Label: "Home"},
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
	renderer := NewHTMLRenderer(&siteConfig, testFS, WithCache(cache))

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Breadcrumbs: []breadcrumb.Breadcrumb{
				{Path: "/", Label: "Home"},
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
	t.Parallel()

	templateContent := `<h1>{{.Site.Meta.Title}}</h1>`
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	renderer := NewHTMLRenderer(&siteConfig, testFS)

	templateCtx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Breadcrumbs: []breadcrumb.Breadcrumb{{Path: "/", Label: "Home"}}},
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	_, err := renderer.Render(ctx, "test.html.tmpl", templateCtx)
	if err != context.Canceled {
		t.Errorf("Render() with cancelled context: got error %v, want context.Canceled", err)
	}
}
