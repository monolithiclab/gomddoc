package template

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

func TestNewHTMLRenderer(t *testing.T) {
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/test.html.tmpl": {
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
		"assets/themes/default/layouts/default.html.tmpl": {
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

	result, err := renderer.Render(context.Background(), "default.html.tmpl", ctx)
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
		"assets/themes/default/layouts/breadcrumbs.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")

	gen := breadcrumb.NewGenerator(func(path string) bool {
		return path == "/" // Only root is dir
	})

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
		"assets/themes/default/layouts/toc.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	tocRoot := &enricher.TOCNode{
		Level: 0,
		Children: []*enricher.TOCNode{
			{Level: 1, Text: "H1", ID: "h1", Children: []*enricher.TOCNode{
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
		"assets/themes/default/layouts/toc_filter.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	tocRoot := &enricher.TOCNode{
		Level: 0,
		Children: []*enricher.TOCNode{
			{Level: 1, Text: "H1", ID: "h1", Children: []*enricher.TOCNode{
				{Level: 2, Text: "H2", ID: "h2", Children: []*enricher.TOCNode{
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
		"assets/themes/default/layouts/test.html.tmpl": {
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
		"assets/themes/default/layouts/test.html.tmpl": {
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
			"assets/themes/default/layouts/default.html.tmpl": {
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
			"assets/themes/other/layouts/default.html.tmpl": {
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
		"assets/themes/default/layouts/test.html.tmpl": {
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
	templatePath := "assets/themes/default/layouts/test.html.tmpl"
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

func TestParseTemplateWithPartials(t *testing.T) {
	t.Parallel()

	// Layout skeleton that calls partials
	layoutContent := `<!DOCTYPE html>
<html>
{{ template "head" . }}
<body>
{{ template "header" . }}
{{ .Page.Content }}
</body>
</html>`

	headPartial := `{{ define "head" }}<head><title>{{ .Site.Meta.Title }}</title></head>{{ end }}`
	headerPartial := `{{ define "header" }}<header>{{ .Site.Meta.Title }}</header>{{ end }}`

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(layoutContent)},
		"assets/themes/default/partials/head.html.tmpl":   {Data: []byte(headPartial)},
		"assets/themes/default/partials/header.html.tmpl": {Data: []byte(headerPartial)},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Partials Test"

	r := NewHTMLRenderer(&siteConfig, testFS)

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Content: "<p>Hello</p>",
			Path:    "/",
		},
	}

	result, err := r.Render(context.Background(), "default.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render with partials failed: %v", err)
	}

	resultStr := string(result)

	if !strings.Contains(resultStr, "<title>Partials Test</title>") {
		t.Error("Expected head partial to render title")
	}
	if !strings.Contains(resultStr, "<header>Partials Test</header>") {
		t.Error("Expected header partial to render site title")
	}
	if !strings.Contains(resultStr, "<p>Hello</p>") {
		t.Error("Expected content to be rendered")
	}
}

func TestParseTemplateFallback(t *testing.T) {
	// Only default theme has the template
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
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
	result, err := renderer.Render(context.Background(), "default.html.tmpl", ctx)
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
		"assets/themes/default/layouts/toc_empty.html.tmpl": {
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
	ctx.Page.TOC = &enricher.TOCNode{Level: 0, Children: nil}
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
		"assets/themes/default/layouts/toc_deep.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	// Create a deep structure where H2 is nested under H1
	// This tests the hasVisibleDescendants recursive path
	tocRoot := &enricher.TOCNode{
		Level: 0,
		Children: []*enricher.TOCNode{
			{
				Level: 1, Text: "H1", ID: "h1",
				Children: []*enricher.TOCNode{
					{
						Level: 2, Text: "H2", ID: "h2",
						Children: []*enricher.TOCNode{
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

func TestEditURLFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		editURL  string
		pagePath string
		want     string
	}{
		{
			name:     "empty edit URL returns empty string",
			editURL:  "",
			pagePath: "/docs/guide.md",
			want:     "",
		},
		{
			name:     "basic edit URL with leading slash path",
			editURL:  "https://github.com/org/repo/edit/main",
			pagePath: "/docs/guide.md",
			want:     "https://github.com/org/repo/edit/main/docs/guide.md",
		},
		{
			name:     "edit URL with trailing slash",
			editURL:  "https://github.com/org/repo/edit/main/",
			pagePath: "/docs/guide.md",
			want:     "https://github.com/org/repo/edit/main/docs/guide.md",
		},
		{
			name:     "path without leading slash",
			editURL:  "https://github.com/org/repo/edit/main",
			pagePath: "docs/guide.md",
			want:     "https://github.com/org/repo/edit/main/docs/guide.md",
		},
		{
			name:     "root path",
			editURL:  "https://github.com/org/repo/edit/main",
			pagePath: "/README.md",
			want:     "https://github.com/org/repo/edit/main/README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			siteConfig.EditURL = tt.editURL

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/test.html.tmpl": {
					Data: []byte("<html></html>"),
				},
			}
			r := NewHTMLRenderer(&siteConfig, testFS)

			got := r.generateEditURL(tt.pagePath)
			if got != tt.want {
				t.Errorf("generateEditURL(%q) = %q, want %q", tt.pagePath, got, tt.want)
			}
		})
	}
}

func TestEditURLInTemplate(t *testing.T) {
	t.Parallel()

	templateContent := `{{- $editLink := editURL .Page.Path }}{{- if $editLink }}<a href="{{ $editLink }}">Edit</a>{{- end }}`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/edit.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	t.Run("edit link shown when configured", func(t *testing.T) {
		t.Parallel()

		siteConfig := config.NewSiteConfig(".")
		siteConfig.EditURL = "https://github.com/org/repo/edit/main"

		r := NewHTMLRenderer(&siteConfig, testFS)
		ctx := &TemplateContext{
			Site: &siteConfig,
			Page: PageContext{Path: "/docs/guide.md"},
		}

		result, err := r.Render(context.Background(), "edit.html.tmpl", ctx)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		if !strings.Contains(string(result), "https://github.com/org/repo/edit/main/docs/guide.md") {
			t.Errorf("Expected edit link in output, got %q", string(result))
		}
	})

	t.Run("edit link hidden when not configured", func(t *testing.T) {
		t.Parallel()

		siteConfig := config.NewSiteConfig(".")
		// EditURL is empty by default

		r := NewHTMLRenderer(&siteConfig, testFS)
		ctx := &TemplateContext{
			Site: &siteConfig,
			Page: PageContext{Path: "/docs/guide.md"},
		}

		result, err := r.Render(context.Background(), "edit.html.tmpl", ctx)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		if string(result) != "" {
			t.Errorf("Expected empty output when EditURL not configured, got %q", string(result))
		}
	})
}

func TestNavigationPartial(t *testing.T) {
	t.Parallel()

	navItemTmpl := `{{ define "nav-item" }}<li>{{- if .IsDir }}<details{{ if .Open }} open{{ end }}><summary>{{ .Title }}</summary>{{- if .Children }}<ul>{{ range .Children }}{{ template "nav-item" . }}{{ end }}</ul>{{- end }}</details>{{- else }}<a href="{{ .Path }}"{{ if .Active }} class="active"{{ end }}>{{ .Title }}</a>{{- end }}</li>{{ end }}`
	layoutTmpl := navItemTmpl + `{{ if and .Page.Navigation .Page.Navigation.Items }}<ul>{{ range .Page.Navigation.Items }}{{ template "nav-item" . }}{{ end }}</ul>{{ end }}`

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/nav.html.tmpl": {
			Data: []byte(layoutTmpl),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Path: "/guide.md",
			Navigation: &enricher.NavTree{
				Items: []enricher.NavItem{
					{Title: "Getting Started", Path: "/guide.md", Active: true},
					{Title: "Docs", Path: "/docs/", IsDir: true, Open: true, Children: []enricher.NavItem{
						{Title: "Installation", Path: "/docs/install.md"},
					}},
				},
			},
		},
	}

	result, err := r.Render(context.Background(), "nav.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	resultStr := string(result)

	if !strings.Contains(resultStr, "Getting Started") {
		t.Error("Expected navigation to contain 'Getting Started'")
	}
	if !strings.Contains(resultStr, `class="active"`) {
		t.Error("Expected active class on current page")
	}
	if !strings.Contains(resultStr, "Installation") {
		t.Error("Expected navigation to contain 'Installation'")
	}
	if !strings.Contains(resultStr, "<details open>") {
		t.Error("Expected open details element for active directory")
	}
}

func TestNavigationPartial_NilNavTree(t *testing.T) {
	t.Parallel()

	layoutTmpl := `[{{ if and .Page.Navigation .Page.Navigation.Items }}nav{{ end }}]`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/nav_nil.html.tmpl": {
			Data: []byte(layoutTmpl),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{
			Path: "/test",
			// Navigation is nil
		},
	}

	result, err := r.Render(context.Background(), "nav_nil.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if string(result) != "[]" {
		t.Errorf("Expected empty navigation without nav tree, got %q", string(result))
	}
}

func TestFuncMap_InlineAsset(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<script>{{inlineAsset "test.js"}}</script>`),
		},
		"assets/themes/default/test.js": {
			Data: []byte("console.log('hello');"),
		},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(string(result), "console.log('hello');") {
		t.Errorf("Expected inlined JS content, got %q", string(result))
	}
}

func TestFuncMap_InlineAsset_SharedFallback(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<script>{{inlineAsset "shared.js"}}</script>`),
		},
		"assets/shared/shared.js": {
			Data: []byte("shared code"),
		},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(string(result), "shared code") {
		t.Errorf("Expected shared asset content, got %q", string(result))
	}
}

func TestFuncMap_InlineAsset_NotFound(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`{{inlineAsset "missing.js"}}`),
		},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	_, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	})
	if err == nil {
		t.Fatal("Render should fail when asset is not found")
	}
}

func TestAssetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		asset   string
		wantURL string
	}{
		{"simple file", "style.css", "/_assets/style.css"},
		{"nested path", "js/app.js", "/_assets/js/app.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {
					Data: []byte(`{{assetURL "` + tt.asset + `"}}`),
				},
			}
			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
				Site: &siteConfig,
				Page: PageContext{Path: "/"},
			})
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}
			got := strings.TrimSpace(string(result))
			if got != tt.wantURL {
				t.Errorf("assetURL = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestHasVisibleDescendants_Direct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node *enricher.TOCNode
		min  int
		max  int
		want bool
	}{
		{
			name: "nil children",
			node: &enricher.TOCNode{},
			min:  1,
			max:  3,
			want: false,
		},
		{
			name: "direct visible child",
			node: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 2},
			}},
			min:  1,
			max:  3,
			want: true,
		},
		{
			name: "nested visible descendant via below-min parent",
			node: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 0, Children: []*enricher.TOCNode{
					{Level: 2},
				}},
			}},
			min:  2,
			max:  3,
			want: true,
		},
		{
			name: "all children out of range above max",
			node: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 5},
			}},
			min:  1,
			max:  3,
			want: false,
		},
		{
			name: "deeply nested visible descendant",
			node: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 0, Children: []*enricher.TOCNode{
					{Level: 0, Children: []*enricher.TOCNode{
						{Level: 2},
					}},
				}},
			}},
			min:  2,
			max:  3,
			want: true,
		},
		{
			name: "below-min child without visible descendants",
			node: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 0, Children: []*enricher.TOCNode{
					{Level: 5},
				}},
			}},
			min:  2,
			max:  3,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := hasVisibleDescendants(tt.node, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("hasVisibleDescendants() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateEditURL_ViaTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		editURL  string
		pagePath string
		want     string
	}{
		{"empty config", "", "/page.md", ""},
		{"basic", "https://github.com/user/repo/edit/main", "/docs/page.md", "https://github.com/user/repo/edit/main/docs/page.md"},
		{"trailing slash", "https://github.com/user/repo/edit/main/", "/page.md", "https://github.com/user/repo/edit/main/page.md"},
		{"no leading slash on path", "https://example.com/edit", "page.md", "https://example.com/edit/page.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {
					Data: []byte(`{{editURL .Page.Path}}`),
				},
			}
			siteConfig := config.NewSiteConfig(".")
			siteConfig.EditURL = tt.editURL
			r := NewHTMLRenderer(&siteConfig, testFS)
			result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
				Site: &siteConfig,
				Page: PageContext{Path: tt.pagePath},
			})
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}
			got := strings.TrimSpace(string(result))
			if got != tt.want {
				t.Errorf("editURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateTOC_WithLevels(t *testing.T) {
	t.Parallel()

	toc := &enricher.TOCNode{
		Children: []*enricher.TOCNode{
			{Level: 1, ID: "intro", Text: "Introduction", Children: []*enricher.TOCNode{
				{Level: 2, ID: "setup", Text: "Setup"},
				{Level: 2, ID: "usage", Text: "Usage"},
			}},
		},
	}
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`{{toc .Page.TOC 1 2}}`),
		},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/", TOC: toc},
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "Introduction") {
		t.Errorf("TOC should contain 'Introduction', got %q", output)
	}
	if !strings.Contains(output, "Setup") {
		t.Errorf("TOC should contain 'Setup', got %q", output)
	}
	if !strings.Contains(output, "Usage") {
		t.Errorf("TOC should contain 'Usage', got %q", output)
	}
}

func TestPartialOverrideResolution(t *testing.T) {
	t.Parallel()

	// Layout that calls two partials: head and footer
	layout := `{{ template "head" . }}|{{ template "footer" . }}`
	themeHead := `{{ define "head" }}theme-head{{ end }}`
	themeFooter := `{{ define "footer" }}theme-footer{{ end }}`

	tests := []struct {
		name     string
		fs       fstest.MapFS
		theme    string
		wantBody string
	}{
		{
			name: "no site partials uses theme partials only",
			fs: fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(layout)},
				"assets/themes/default/partials/head.html.tmpl":   {Data: []byte(themeHead)},
				"assets/themes/default/partials/footer.html.tmpl": {Data: []byte(themeFooter)},
			},
			theme:    "default",
			wantBody: "theme-head|theme-footer",
		},
		{
			name: "site partial overrides one theme partial",
			fs: fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(layout)},
				"assets/themes/default/partials/head.html.tmpl":   {Data: []byte(themeHead)},
				"assets/themes/default/partials/footer.html.tmpl": {Data: []byte(themeFooter)},
				"partials/head.html.tmpl":                         {Data: []byte(`{{ define "head" }}site-head{{ end }}`)},
			},
			theme:    "default",
			wantBody: "site-head|theme-footer",
		},
		{
			name: "site partial overrides all theme partials",
			fs: fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(layout)},
				"assets/themes/default/partials/head.html.tmpl":   {Data: []byte(themeHead)},
				"assets/themes/default/partials/footer.html.tmpl": {Data: []byte(themeFooter)},
				"partials/head.html.tmpl":                         {Data: []byte(`{{ define "head" }}site-head{{ end }}`)},
				"partials/footer.html.tmpl":                       {Data: []byte(`{{ define "footer" }}site-footer{{ end }}`)},
			},
			theme:    "default",
			wantBody: "site-head|site-footer",
		},
		{
			name: "custom theme with site partial override",
			fs: fstest.MapFS{
				"assets/themes/mytheme/layouts/default.html.tmpl": {Data: []byte(layout)},
				"assets/themes/mytheme/partials/head.html.tmpl":   {Data: []byte(`{{ define "head" }}mytheme-head{{ end }}`)},
				"assets/themes/mytheme/partials/footer.html.tmpl": {Data: []byte(`{{ define "footer" }}mytheme-footer{{ end }}`)},
				"partials/head.html.tmpl":                         {Data: []byte(`{{ define "head" }}site-head{{ end }}`)},
			},
			theme:    "mytheme",
			wantBody: "site-head|mytheme-footer",
		},
		{
			name: "custom theme inherits default partials for missing definitions",
			fs: fstest.MapFS{
				"assets/themes/mytheme/layouts/default.html.tmpl": {Data: []byte(layout)},
				"assets/themes/mytheme/partials/head.html.tmpl":   {Data: []byte(`{{ define "head" }}mytheme-head{{ end }}`)},
				"assets/themes/default/partials/head.html.tmpl":   {Data: []byte(themeHead)},
				"assets/themes/default/partials/footer.html.tmpl": {Data: []byte(themeFooter)},
			},
			theme:    "mytheme",
			wantBody: "mytheme-head|theme-footer",
		},
		{
			name: "site partial overrides default partial inherited by custom theme",
			fs: fstest.MapFS{
				"assets/themes/mytheme/layouts/default.html.tmpl": {Data: []byte(layout)},
				"assets/themes/mytheme/partials/head.html.tmpl":   {Data: []byte(`{{ define "head" }}mytheme-head{{ end }}`)},
				"assets/themes/default/partials/head.html.tmpl":   {Data: []byte(themeHead)},
				"assets/themes/default/partials/footer.html.tmpl": {Data: []byte(themeFooter)},
				"partials/footer.html.tmpl":                       {Data: []byte(`{{ define "footer" }}site-footer{{ end }}`)},
			},
			theme:    "mytheme",
			wantBody: "mytheme-head|site-footer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			siteConfig.Theme.Name = tt.theme

			r := NewHTMLRenderer(&siteConfig, tt.fs)
			ctx := &TemplateContext{
				Site: &siteConfig,
				Page: PageContext{Path: "/"},
			}

			result, err := r.Render(context.Background(), "default.html.tmpl", ctx)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if got := string(result); got != tt.wantBody {
				t.Errorf("got %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestHasTemplate(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte("<html></html>")},
		"assets/themes/default/layouts/page.html.tmpl":    {Data: []byte("<html>page</html>")},
	}

	tests := []struct {
		name         string
		templateName string
		want         bool
	}{
		{"existing default template", "default.html.tmpl", true},
		{"existing page template", "page.html.tmpl", true},
		{"nonexistent template", "missing.html.tmpl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			got := r.HasTemplate(tt.templateName)
			if got != tt.want {
				t.Errorf("HasTemplate(%q) = %v, want %v", tt.templateName, got, tt.want)
			}
		})
	}
}

func TestResolveLayout(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte("<html></html>")},
		"assets/themes/default/layouts/page.html.tmpl":    {Data: []byte("<html>page</html>")},
		"assets/themes/default/layouts/wide.html.tmpl":    {Data: []byte("<html>wide</html>")},
	}

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tests := []struct {
		name     string
		metadata map[string]any
		want     string
	}{
		{
			name:     "no layout field uses default",
			metadata: map[string]any{"title": "Test"},
			want:     "default.html.tmpl",
		},
		{
			name:     "nil metadata uses default",
			metadata: nil,
			want:     "default.html.tmpl",
		},
		{
			name:     "empty layout uses default",
			metadata: map[string]any{"layout": ""},
			want:     "default.html.tmpl",
		},
		{
			name:     "existing layout is used",
			metadata: map[string]any{"layout": "page"},
			want:     "page.html.tmpl",
		},
		{
			name:     "another existing layout",
			metadata: map[string]any{"layout": "wide"},
			want:     "wide.html.tmpl",
		},
		{
			name:     "nonexistent layout falls back to default",
			metadata: map[string]any{"layout": "nonexistent"},
			want:     "default.html.tmpl",
		},
		{
			name:     "non-string layout type falls back to default",
			metadata: map[string]any{"layout": 42},
			want:     "default.html.tmpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveLayout(r, tt.metadata)
			if got != tt.want {
				t.Errorf("ResolveLayout() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLayoutSelectionRendering(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(`DEFAULT:{{.Page.Content}}`)},
		"assets/themes/default/layouts/page.html.tmpl":    {Data: []byte(`PAGE:{{.Page.Content}}`)},
	}

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tests := []struct {
		name     string
		metadata map[string]any
		contains string
	}{
		{
			name:     "default layout renders with DEFAULT prefix",
			metadata: map[string]any{},
			contains: "DEFAULT:",
		},
		{
			name:     "page layout renders with PAGE prefix",
			metadata: map[string]any{"layout": "page"},
			contains: "PAGE:",
		},
		{
			name:     "nonexistent layout falls back to DEFAULT prefix",
			metadata: map[string]any{"layout": "nonexistent"},
			contains: "DEFAULT:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			templateName := ResolveLayout(r, tt.metadata)
			ctx := &TemplateContext{
				Site: &siteConfig,
				Page: PageContext{
					Content: "hello",
					Path:    "/",
					Meta:    tt.metadata,
				},
			}

			result, err := r.Render(context.Background(), templateName, ctx)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if !strings.Contains(string(result), tt.contains) {
				t.Errorf("Expected output to contain %q, got %q", tt.contains, string(result))
			}
		})
	}
}

func TestGenerateBreadcrumbs_NilGenerator(t *testing.T) {
	templateContent := `{{ len (breadcrumbs .Page.Path) }}`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/bc.html.tmpl": {
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
