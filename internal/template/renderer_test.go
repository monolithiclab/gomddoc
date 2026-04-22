package template

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/renderer"
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
			Path:    "/test",
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

	if ctx.Page.Path != "/test" {
		t.Errorf("Expected path '/test', got %q", ctx.Page.Path)
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
			Path:    "/",
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

// MockInfoProvider for testing breadcrumbs
type MockInfoProvider struct{}

func (m *MockInfoProvider) IsDir(path string) bool {
	return path == "/" // Only root is dir
}

func TestBreadcrumbsFunction(t *testing.T) {
	// Template that uses the breadcrumbs function
	templateContent := `
{{- $crumbs := breadcrumbs .Page.Path -}}
Count: {{ len $crumbs }}
{{- range $crumbs -}}
Path: {{ .Path }}, Label: {{ .Label }}|
{{- end -}}
`
	testFS := fstest.MapFS{
		"assets/themes/default/breadcrumbs.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")

	// Create generator with mock provider
	gen := breadcrumb.NewGenerator(&MockInfoProvider{})

	renderer := NewHTMLRenderer(&siteConfig, testFS, WithBreadcrumbGenerator(gen))

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Path: "/foo/bar",
		},
	}

	result, err := renderer.Render(context.Background(), "breadcrumbs.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	resultStr := string(result)

	// Expected: Home (/) -> Foo (/foo/) -> Bar (/foo/bar)
	// MockInfoProvider says only / is dir, so /foo/ is NOT dir -> no trailing slash?
	// Wait, Generator logic: intermediate segments are assumed dirs if I recall correctly, OR it checks.
	// Let's check generator.go again.
	// "All intermediate segments are directories - add trailing slash"
	// "For the last segment... check IsDir"

	// So:
	// 1. Home: Path: /, Label: Home
	// 2. Foo: Path: /foo/, Label: Foo (Intermediate)
	// 3. Bar: Path: /foo/bar, Label: Bar (Last, IsDir("/foo/bar") -> false -> no trailing slash)

	expected := "Count: 3"
	if !strings.Contains(resultStr, expected) {
		t.Errorf("Expected %q, got %q", expected, resultStr)
	}

	expectedCrumbs := "Path: /, Label: Home|Path: /foo/, Label: Foo|Path: /foo/bar, Label: Bar|"
	// Normalize whitespace for comparison if needed, but template is compact
	if !strings.Contains(strings.ReplaceAll(resultStr, "\n", ""), expectedCrumbs) {
		t.Errorf("Expected breadcrumbs %q, got %q", expectedCrumbs, resultStr)
	}
}

func TestTOCFunction(t *testing.T) {
	// Template that uses the toc function
	templateContent := `{{ toc .Page.TOC }}`
	testFS := fstest.MapFS{
		"assets/themes/default/toc.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	tocRoot := &renderer.TOCNode{
		Level: 0,
		Children: []*renderer.TOCNode{
			{Level: 1, Text: "H1", ID: "h1", Children: []*renderer.TOCNode{
				{Level: 2, Text: "H2", ID: "h2"},
			}},
			{Level: 1, Text: "H1-2", ID: "h1-2"},
		},
	}

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			TOC: tocRoot,
		},
	}

	result, err := rendererObj.Render(context.Background(), "toc.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	res := string(result)
	// Expected: <ul><li><a href="#h1">H1</a><ul><li><a href="#h2">H2</a></li></ul></li><li><a href="#h1-2">H1-2</a></li></ul>

	if !strings.Contains(res, `<a href="#h1">H1</a>`) {
		t.Error("Missing H1 link")
	}
	if !strings.Contains(res, `<a href="#h2">H2</a>`) {
		t.Error("Missing H2 link")
	}
}

func TestTOCFunction_Filtering(t *testing.T) {
	// Template filtering levels 2-3
	templateContent := `{{ toc .Page.TOC 2 3 }}`
	testFS := fstest.MapFS{
		"assets/themes/default/toc_filter.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	tocRoot := &renderer.TOCNode{
		Level: 0,
		Children: []*renderer.TOCNode{
			{Level: 1, Text: "H1", ID: "h1", Children: []*renderer.TOCNode{
				{Level: 2, Text: "H2", ID: "h2", Children: []*renderer.TOCNode{
					{Level: 3, Text: "H3", ID: "h3"},
					{Level: 4, Text: "H4", ID: "h4"},
				}},
			}},
		},
	}

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			TOC: tocRoot,
		},
	}

	result, err := rendererObj.Render(context.Background(), "toc_filter.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	res := string(result)

	// H1 (Level 1) should be skipped but traversed
	if strings.Contains(res, ">H1<") {
		t.Error("H1 should be skipped")
	}

	// H2 (Level 2) should be present
	if !strings.Contains(res, ">H2<") {
		t.Error("H2 should be present")
	}

	// H3 (Level 3) should be present
	if !strings.Contains(res, ">H3<") {
		t.Error("H3 should be present")
	}

	// H4 (Level 4) should be skipped
	if strings.Contains(res, ">H4<") {
		t.Error("H4 should be skipped")
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
			Path: "/",
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
		Page: PageContext{Path: "/"},
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	_, err := renderer.Render(ctx, "test.html.tmpl", templateCtx)
	if err != context.Canceled {
		t.Errorf("Render() with cancelled context: got error %v, want context.Canceled", err)
	}
}

func TestValidateDefaultTheme(t *testing.T) {
	t.Run("valid default theme", func(t *testing.T) {
		testFS := fstest.MapFS{
			"assets/themes/default/layout.html.tmpl": {
				Data: []byte("<html></html>"),
			},
		}
		siteConfig := config.NewSiteConfig(".")
		renderer := NewHTMLRenderer(&siteConfig, testFS)

		err := renderer.ValidateDefaultTheme()
		if err != nil {
			t.Errorf("ValidateDefaultTheme() should succeed, got error: %v", err)
		}
	})

	t.Run("missing default theme", func(t *testing.T) {
		testFS := fstest.MapFS{
			"assets/themes/other/layout.html.tmpl": {
				Data: []byte("<html></html>"),
			},
		}
		siteConfig := config.NewSiteConfig(".")
		renderer := NewHTMLRenderer(&siteConfig, testFS)

		err := renderer.ValidateDefaultTheme()
		if err == nil {
			t.Error("ValidateDefaultTheme() should fail when default theme is missing")
		}
	})
}

func TestClearCache(t *testing.T) {
	templateContent := `<h1>{{.Site.Meta.Title}}</h1>`
	testFS := fstest.MapFS{
		"assets/themes/default/test.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	cache := &CachedTemplateStore{}
	renderer := NewHTMLRenderer(&siteConfig, testFS, WithCache(cache))

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	}

	// First render - populates cache
	_, err := renderer.Render(context.Background(), "test.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Verify cache has entry
	templatePath := "assets/themes/default/test.html.tmpl"
	if cache.Get(templatePath) == nil {
		t.Error("Cache should have entry after render")
	}

	// Clear cache
	renderer.ClearCache()

	// Verify cache is empty
	if cache.Get(templatePath) != nil {
		t.Error("Cache should be empty after ClearCache()")
	}
}

func TestParseTemplateFallback(t *testing.T) {
	// Only default theme has the template
	testFS := fstest.MapFS{
		"assets/themes/default/layout.html.tmpl": {
			Data: []byte("<html>default</html>"),
		},
	}

	// Configure to use a custom theme that doesn't exist
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Theme.Name = "nonexistent"

	renderer := NewHTMLRenderer(&siteConfig, testFS)

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	}

	// Should fallback to default theme
	result, err := renderer.Render(context.Background(), "layout.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render should succeed with fallback, got error: %v", err)
	}

	if !strings.Contains(string(result), "default") {
		t.Error("Should have used default theme template")
	}
}

func TestTOCFunction_EmptyTOC(t *testing.T) {
	templateContent := `{{ toc .Page.TOC }}`
	testFS := fstest.MapFS{
		"assets/themes/default/toc_empty.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	// Test with nil TOC
	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			TOC: nil,
		},
	}

	result, err := rendererObj.Render(context.Background(), "toc_empty.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "" {
		t.Errorf("Empty TOC should produce empty output, got %q", string(result))
	}

	// Test with empty children
	ctx.Page.TOC = &renderer.TOCNode{Level: 0, Children: nil}
	result, err = rendererObj.Render(context.Background(), "toc_empty.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "" {
		t.Errorf("TOC with no children should produce empty output, got %q", string(result))
	}
}

func TestHasVisibleDescendants(t *testing.T) {
	// Template filtering levels 2-2 (only H2)
	templateContent := `{{ toc .Page.TOC 2 2 }}`
	testFS := fstest.MapFS{
		"assets/themes/default/toc_deep.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	// Create a deep structure where H2 is nested under H1
	// This tests the hasVisibleDescendants recursive path
	tocRoot := &renderer.TOCNode{
		Level: 0,
		Children: []*renderer.TOCNode{
			{
				Level: 1, Text: "H1", ID: "h1",
				Children: []*renderer.TOCNode{
					{
						Level: 2, Text: "H2", ID: "h2",
						Children: []*renderer.TOCNode{
							{Level: 3, Text: "H3", ID: "h3"},
						},
					},
				},
			},
		},
	}

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			TOC: tocRoot,
		},
	}

	result, err := rendererObj.Render(context.Background(), "toc_deep.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	res := string(result)

	// H1 should be skipped (level 1, but we want 2-2)
	if strings.Contains(res, ">H1<") {
		t.Error("H1 should be skipped")
	}

	// H2 should be present
	if !strings.Contains(res, ">H2<") {
		t.Error("H2 should be present")
	}

	// H3 should be skipped (level 3, but we want 2-2)
	if strings.Contains(res, ">H3<") {
		t.Error("H3 should be skipped")
	}
}

func TestGenerateBreadcrumbs_NilGenerator(t *testing.T) {
	templateContent := `{{ len (breadcrumbs .Page.Path) }}`
	testFS := fstest.MapFS{
		"assets/themes/default/bc.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	// Don't provide a breadcrumb generator
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Path: "/foo/bar",
		},
	}

	result, err := rendererObj.Render(context.Background(), "bc.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Should return empty slice (length 0)
	if string(result) != "0" {
		t.Errorf("Expected 0 breadcrumbs without generator, got %q", string(result))
	}
}
