package template

import (
	"html/template"
	"strings"
	"testing"
	"testing/fstest"
)

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
