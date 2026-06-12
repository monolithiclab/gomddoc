package template

import (
	"context"
	"html/template"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/resolve"
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

// tocItemPartial is the recursive toc-item partial used in tests.
// Matches the default theme's toc.html.tmpl definition.
const tocItemPartial = `{{ define "toc-item" }}<li><a href="#{{ .ID }}">{{ .Text }}</a>{{ if .Children }}<ul>{{ range .Children }}{{ template "toc-item" . }}{{ end }}</ul>{{ end }}</li>{{ end }}`

func TestTOCFunction(t *testing.T) {
	templateContent := tocItemPartial + `{{ range toc .Page.TOC }}<ul>{{ template "toc-item" . }}</ul>{{ end }}`
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

	if !strings.Contains(res, `<a href="#h1">H1</a>`) {
		t.Error("Missing H1 link")
	}
	if !strings.Contains(res, `<a href="#h2">H2</a>`) {
		t.Error("Missing H2 link")
	}
}

func TestTOCFunction_Filtering(t *testing.T) {
	// Template filtering levels 2-3
	templateContent := tocItemPartial + `{{ range toc .Page.TOC 2 3 }}<ul>{{ template "toc-item" . }}</ul>{{ end }}`
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
	templateContent := tocItemPartial + `{{ range toc .Page.TOC }}{{ template "toc-item" . }}{{ end }}`
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

func TestTOCFunction_TransparentTraversal(t *testing.T) {
	// Template filtering levels 2-2 (only H2) — H1 must be traversed transparently
	templateContent := tocItemPartial + `{{ range toc .Page.TOC 2 2 }}<ul>{{ template "toc-item" . }}</ul>{{ end }}`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/toc_deep.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	rendererObj := NewHTMLRenderer(&siteConfig, testFS)

	// Create a deep structure where H2 is nested under H1
	// This tests transparent traversal of below-min nodes
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

	// H2 should be present (promoted through transparent H1 traversal)
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

func TestFilterTOCNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		root     *enricher.TOCNode
		min, max int
		wantIDs  []string // expected top-level node IDs in order
	}{
		{
			name:    "nil root",
			root:    nil,
			min:     1,
			max:     3,
			wantIDs: nil,
		},
		{
			name:    "empty children",
			root:    &enricher.TOCNode{},
			min:     1,
			max:     3,
			wantIDs: nil,
		},
		{
			name: "direct visible children",
			root: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 1, ID: "a"},
				{Level: 2, ID: "b"},
			}},
			min:     1,
			max:     3,
			wantIDs: []string{"a", "b"},
		},
		{
			name: "above max dropped",
			root: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 1, ID: "a"},
				{Level: 5, ID: "b"},
			}},
			min:     1,
			max:     3,
			wantIDs: []string{"a"},
		},
		{
			name: "below min promotes children",
			root: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 1, ID: "h1", Children: []*enricher.TOCNode{
					{Level: 2, ID: "h2a"},
					{Level: 2, ID: "h2b"},
				}},
			}},
			min:     2,
			max:     3,
			wantIDs: []string{"h2a", "h2b"},
		},
		{
			name: "deeply nested promotion",
			root: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 0, ID: "skip1", Children: []*enricher.TOCNode{
					{Level: 0, ID: "skip2", Children: []*enricher.TOCNode{
						{Level: 2, ID: "found"},
					}},
				}},
			}},
			min:     2,
			max:     3,
			wantIDs: []string{"found"},
		},
		{
			name: "children filtered recursively",
			root: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 2, ID: "h2", Children: []*enricher.TOCNode{
					{Level: 3, ID: "h3"},
					{Level: 4, ID: "h4"},
				}},
			}},
			min:     2,
			max:     3,
			wantIDs: []string{"h2"},
		},
		{
			name: "below-min without visible descendants returns empty",
			root: &enricher.TOCNode{Children: []*enricher.TOCNode{
				{Level: 0, Children: []*enricher.TOCNode{
					{Level: 5, ID: "too-deep"},
				}},
			}},
			min:     2,
			max:     3,
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := filterTOC(tt.root, tt.min, tt.max)
			var gotIDs []string
			for _, n := range got {
				gotIDs = append(gotIDs, n.ID)
			}

			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("filterTOC() returned %d nodes %v, want %d nodes %v", len(gotIDs), gotIDs, len(tt.wantIDs), tt.wantIDs)
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Errorf("filterTOC()[%d].ID = %q, want %q", i, gotIDs[i], tt.wantIDs[i])
				}
			}
		})
	}
}

func TestFilterTOCNodes_PreservesChildHierarchy(t *testing.T) {
	t.Parallel()

	root := &enricher.TOCNode{Children: []*enricher.TOCNode{
		{Level: 2, ID: "h2", Text: "H2", Children: []*enricher.TOCNode{
			{Level: 3, ID: "h3", Text: "H3"},
			{Level: 4, ID: "h4", Text: "H4"},
		}},
	}}

	got := filterTOC(root, 2, 3)
	if len(got) != 1 || got[0].ID != "h2" {
		t.Fatalf("expected [h2], got %v", got)
	}
	// h3 should be kept, h4 dropped
	if len(got[0].Children) != 1 || got[0].Children[0].ID != "h3" {
		t.Errorf("h2 children: expected [h3], got %v", got[0].Children)
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

func TestTOCFunction_IntegrationWithPartial(t *testing.T) {
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
			Data: []byte(tocItemPartial + `{{ $items := toc .Page.TOC 1 2 }}{{ if $items }}<ul>{{ range $items }}{{ template "toc-item" . }}{{ end }}</ul>{{ end }}`),
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
	// Verify nested structure: Setup and Usage should be children of Introduction
	if !strings.Contains(output, `<a href="#intro">Introduction</a>`) {
		t.Errorf("Missing Introduction link, got %q", output)
	}
	if !strings.Contains(output, `<a href="#setup">Setup</a>`) {
		t.Errorf("Missing Setup link, got %q", output)
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

func TestJSONLDFunction(t *testing.T) {
	t.Parallel()

	layoutContent := `{{ template "jsonld" . }}`
	partialContent := `{{ define "jsonld" }}{{ $json := jsonLD .Page }}{{ if $json }}<script type="application/ld+json">{{ $json }}</script>{{ end }}{{ end }}`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/jsonld.html.tmpl": {
			Data: []byte(layoutContent),
		},
		"assets/themes/default/partials/jsonld.html.tmpl": {
			Data: []byte(partialContent),
		},
	}

	t.Run("with domain produces script tag", func(t *testing.T) {
		t.Parallel()

		siteConfig := config.NewSiteConfig(".")
		siteConfig.Meta.Domain = "docs.example.com"
		siteConfig.Meta.Title = "My Docs"

		r := NewHTMLRenderer(&siteConfig, testFS, WithSearchIndex())
		ctx := &TemplateContext{
			Site: &siteConfig,
			Page: PageContext{
				Path: "/guide.md",
				Meta: map[string]any{
					"title":       "Guide",
					"description": "A guide",
					"author":      "Alice",
				},
			},
		}

		result, err := r.Render(context.Background(), "jsonld.html.tmpl", ctx)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		output := string(result)
		if !strings.Contains(output, `<script type="application/ld+json">`) {
			t.Error("Expected script tag in output")
		}
		if !strings.Contains(output, `"TechArticle"`) {
			t.Error("Expected TechArticle type")
		}
		if !strings.Contains(output, `"Guide"`) {
			t.Error("Expected headline")
		}
	})

	t.Run("without domain produces empty", func(t *testing.T) {
		t.Parallel()

		siteConfig := config.NewSiteConfig(".")

		r := NewHTMLRenderer(&siteConfig, testFS)
		ctx := &TemplateContext{
			Site: &siteConfig,
			Page: PageContext{
				Path: "/guide.md",
				Meta: map[string]any{"title": "Guide"},
			},
		}

		result, err := r.Render(context.Background(), "jsonld.html.tmpl", ctx)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		if string(result) != "" {
			t.Errorf("Expected empty output without domain, got %q", string(result))
		}
	})

	t.Run("index page includes WebSite schema", func(t *testing.T) {
		t.Parallel()

		siteConfig := config.NewSiteConfig(".")
		siteConfig.Meta.Domain = "docs.example.com"
		siteConfig.Meta.Title = "My Docs"
		siteConfig.DefaultIndex = "README.md"

		r := NewHTMLRenderer(&siteConfig, testFS)
		ctx := &TemplateContext{
			Site: &siteConfig,
			Page: PageContext{
				Path: "/README.md",
				Meta: map[string]any{"title": "Home"},
			},
		}

		result, err := r.Render(context.Background(), "jsonld.html.tmpl", ctx)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		output := string(result)
		if !strings.Contains(output, `"WebSite"`) {
			t.Error("Expected WebSite schema for index page")
		}
	})

	t.Run("with breadcrumb generator", func(t *testing.T) {
		t.Parallel()

		siteConfig := config.NewSiteConfig(".")
		siteConfig.Meta.Domain = "docs.example.com"

		stubGen := &stubBreadcrumbGen{
			crumbs: []breadcrumb.Breadcrumb{
				{Path: "/", Label: "Home"},
				{Path: "/guide/", Label: "Guide"},
				{Path: "/guide/setup.md", Label: "Setup"},
			},
		}

		r := NewHTMLRenderer(&siteConfig, testFS, WithBreadcrumbGenerator(stubGen))
		ctx := &TemplateContext{
			Site: &siteConfig,
			Page: PageContext{
				Path: "/guide/setup.md",
				Meta: map[string]any{"title": "Setup"},
			},
		}

		result, err := r.Render(context.Background(), "jsonld.html.tmpl", ctx)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}

		output := string(result)
		if !strings.Contains(output, `"BreadcrumbList"`) {
			t.Error("Expected BreadcrumbList schema with breadcrumbs")
		}
	})
}

type stubBreadcrumbGen struct {
	crumbs []breadcrumb.Breadcrumb
}

func (s *stubBreadcrumbGen) Generate(_ string) []breadcrumb.Breadcrumb {
	return s.crumbs
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

// countingCache wraps CachedTemplateStore and counts Set calls to detect
// how many times a template was actually parsed vs served from cache.
type countingCache struct {
	CachedTemplateStore
	setCalls atomic.Int64
}

func (c *countingCache) Set(key string, tmpl *template.Template) {
	c.setCalls.Add(1)
	c.CachedTemplateStore.Set(key, tmpl)
}

func TestRender_SingleflightCoalescesConcurrentParsing(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<h1>{{.Site.Meta.Title}}</h1>`),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test"
	cache := &countingCache{}
	renderer := NewHTMLRenderer(&siteConfig, testFS, WithCache(cache))

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	}

	// Launch many concurrent renders on a cold cache
	const numGoroutines = 50
	var wg sync.WaitGroup
	errs := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = renderer.Render(context.Background(), "default.html.tmpl", ctx)
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: Render failed: %v", i, err)
		}
	}

	// Without singleflight, all 50 goroutines would parse and call Set.
	// With singleflight, only 1 should parse and call Set.
	sets := cache.setCalls.Load()
	if sets != 1 {
		t.Errorf("expected exactly 1 cache Set (singleflight coalescing), got %d", sets)
	}
}

func TestTemplateContext_Feature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		features map[string]bool
		pageMeta map[string]any
		feature  string
		want     bool
	}{
		{
			name:     "nil features defaults true",
			features: nil,
			pageMeta: nil,
			feature:  "katex",
			want:     true,
		},
		{
			name:     "explicitly disabled",
			features: map[string]bool{"katex": false},
			pageMeta: nil,
			feature:  "katex",
			want:     false,
		},
		{
			name:     "explicitly enabled",
			features: map[string]bool{"katex": true},
			pageMeta: nil,
			feature:  "katex",
			want:     true,
		},
		{
			name:     "page override disables",
			features: map[string]bool{"katex": true},
			pageMeta: map[string]any{"features": map[string]any{"katex": false}},
			feature:  "katex",
			want:     false,
		},
		{
			name:     "page override enables",
			features: map[string]bool{"katex": false},
			pageMeta: map[string]any{"features": map[string]any{"katex": true}},
			feature:  "katex",
			want:     true,
		},
		{
			name:     "unknown feature defaults true",
			features: map[string]bool{"katex": false},
			pageMeta: nil,
			feature:  "dark_mode",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := &TemplateContext{
				Site: &config.SiteConfig{Theme: config.ThemeConfig{Features: tt.features}},
				Page: PageContext{
					Meta:     tt.pageMeta,
					Features: config.MergeFeatures(tt.features, config.ExtractPageFeatures(tt.pageMeta)),
				},
			}
			if got := ctx.Feature(tt.feature); got != tt.want {
				t.Errorf("Feature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestContentURL(t *testing.T) {
	t.Parallel()

	// Build a resolver with some .md files
	contentFS := fstest.MapFS{
		"docs/guide.md":  {Data: []byte("# Guide")},
		"docs/setup.md":  {Data: []byte("# Setup")},
		"docs/README.md": {Data: []byte("# Docs index")},
		"README.md":      {Data: []byte("# Root")},
		"docs/image.jpg": {Data: []byte("binary")},
	}
	resolver := resolve.Build(contentFS, []string{".md"}, func(string) bool { return true })

	tests := []struct {
		name     string
		path     string
		resolver *resolve.PathResolver
		want     string
	}{
		{"extensionless with resolver", "docs/guide.md", resolver, "/docs/guide"},
		{"leading slash stripped", "/docs/setup.md", resolver, "/docs/setup"},
		{"default index stripped", "docs/README.md", resolver, "/docs"},
		{"root index", "README.md", resolver, "/"},
		{"no resolver passthrough", "docs/guide.md", nil, "/docs/guide.md"},
		{"non-mapped file passthrough", "docs/image.jpg", resolver, "/docs/image.jpg"},
		{"already clean path", "docs/guide", resolver, "/docs/guide"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, fstest.MapFS{})
			if tt.resolver != nil {
				r.Configure(WithResolver(tt.resolver))
			}

			got := r.contentURL(tt.path)
			if got != tt.want {
				t.Errorf("contentURL(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestContentURLInTemplate(t *testing.T) {
	t.Parallel()

	contentFS := fstest.MapFS{
		"docs/guide.md": {Data: []byte("# Guide")},
	}
	resolver := resolve.Build(contentFS, []string{".md"}, func(string) bool { return true })

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`{{contentURL "docs/guide.md"}}`),
		},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS, WithResolver(resolver))

	result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	got := strings.TrimSpace(string(result))
	if got != "/docs/guide" {
		t.Errorf("contentURL in template = %q, want %q", got, "/docs/guide")
	}
}

func TestTemplateContext_T(t *testing.T) {
	t.Parallel()

	tc := &TemplateContext{
		Site: &config.SiteConfig{Language: "en-US"},
		Page: PageContext{
			Meta: map[string]any{},
		},
		tFunc: func(key string) string {
			switch key {
			case "toc_title":
				return "On this page"
			default:
				return key
			}
		},
	}

	if got := tc.T("toc_title"); got != "On this page" {
		t.Errorf("T(toc_title) = %q, want %q", got, "On this page")
	}
	if got := tc.T("unknown"); got != "unknown" {
		t.Errorf("T(unknown) = %q, want %q", got, "unknown")
	}
}

func TestTemplateContext_T_NilFunc(t *testing.T) {
	t.Parallel()

	tc := &TemplateContext{
		Site: &config.SiteConfig{Language: "en-US"},
		Page: PageContext{Meta: map[string]any{}},
	}

	// Without tFunc, T returns the key itself
	if got := tc.T("toc_title"); got != "toc_title" {
		t.Errorf("T(toc_title) = %q, want %q", got, "toc_title")
	}
}

func TestTemplateContext_Lang(t *testing.T) {
	t.Parallel()

	tc := &TemplateContext{
		Site: &config.SiteConfig{Language: "fr-FR"},
		Page: PageContext{Meta: map[string]any{}},
		lang: "fr-FR",
	}

	if got := tc.Lang(); got != "fr-FR" {
		t.Errorf("Lang() = %q, want %q", got, "fr-FR")
	}
}

func TestTemplateContext_LangOverrideFromMeta(t *testing.T) {
	t.Parallel()

	tc := &TemplateContext{
		Site: &config.SiteConfig{Language: "en-US"},
		Page: PageContext{Meta: map[string]any{"lang": "de-DE"}},
		lang: "en-US",
	}

	// Page-level lang in frontmatter takes precedence
	if got := tc.Lang(); got != "de-DE" {
		t.Errorf("Lang() = %q, want %q", got, "de-DE")
	}
}

func TestTemplateContext_LangFallbackToSiteConfig(t *testing.T) {
	t.Parallel()

	tc := &TemplateContext{
		Site: &config.SiteConfig{Language: "en-US"},
		Page: PageContext{Meta: map[string]any{}},
		// lang is empty, no frontmatter override
	}

	if got := tc.Lang(); got != "en-US" {
		t.Errorf("Lang() = %q, want %q", got, "en-US")
	}
}

// stubNamer implements LanguageNamer for testing.
type stubNamer struct {
	names map[string]string
}

func (s stubNamer) LanguageName(lang string) string {
	if name, ok := s.names[lang]; ok {
		return name
	}
	return lang
}

func TestBuildLanguageInfos(t *testing.T) {
	t.Parallel()

	namer := stubNamer{names: map[string]string{
		"en-US": "English",
		"fr-FR": "Français",
		"de-DE": "Deutsch",
	}}

	tests := []struct {
		name        string
		defaultLang string
		langs       []string
		wantLen     int
		wantFirst   LanguageInfo
	}{
		{
			name:        "single non-default language",
			defaultLang: "en-US",
			langs:       []string{"fr-FR"},
			wantLen:     2,
			wantFirst:   LanguageInfo{Code: "en-US", Name: "English", Default: true},
		},
		{
			name:        "multiple non-default languages",
			defaultLang: "en-US",
			langs:       []string{"fr-FR", "de-DE"},
			wantLen:     3,
			wantFirst:   LanguageInfo{Code: "en-US", Name: "English", Default: true},
		},
		{
			name:        "no non-default languages",
			defaultLang: "en-US",
			langs:       nil,
			wantLen:     1,
			wantFirst:   LanguageInfo{Code: "en-US", Name: "English", Default: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			infos := BuildLanguageInfos(namer, tt.defaultLang, tt.langs)
			if len(infos) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(infos), tt.wantLen)
			}
			if infos[0] != tt.wantFirst {
				t.Errorf("first = %+v, want %+v", infos[0], tt.wantFirst)
			}
			// Non-default entries should not have Default set
			for _, info := range infos[1:] {
				if info.Default {
					t.Errorf("non-default language %q has Default=true", info.Code)
				}
			}
		})
	}
}

func TestWithActiveLang(t *testing.T) {
	t.Parallel()

	base := []LanguageInfo{
		{Code: "en-US", Name: "English", Default: true},
		{Code: "fr-FR", Name: "Français"},
		{Code: "de-DE", Name: "Deutsch"},
	}

	tests := []struct {
		name       string
		activeLang string
		wantActive string
	}{
		{"default language active", "en-US", "en-US"},
		{"non-default language active", "fr-FR", "fr-FR"},
		{"another non-default", "de-DE", "de-DE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := WithActiveLang(base, tt.activeLang)

			// Should not mutate the original
			for _, info := range base {
				if info.Active {
					t.Fatal("WithActiveLang mutated original slice")
				}
			}

			activeCount := 0
			for _, info := range result {
				if info.Active {
					activeCount++
					if info.Code != tt.wantActive {
						t.Errorf("active code = %q, want %q", info.Code, tt.wantActive)
					}
				}
				// Default flag should be preserved
				if info.Code == "en-US" && !info.Default {
					t.Error("Default flag not preserved for en-US")
				}
			}
			if activeCount != 1 {
				t.Errorf("active count = %d, want 1", activeCount)
			}
		})
	}
}

func TestWithActiveLang_Empty(t *testing.T) {
	t.Parallel()

	result := WithActiveLang(nil, "en-US")
	if result != nil {
		t.Errorf("WithActiveLang(nil) = %v, want nil", result)
	}
}

func TestTagURL(t *testing.T) {
	t.Parallel()

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, fstest.MapFS{})
	fn := r.funcMap()["tagURL"].(func(lang, tag string) string)

	tests := []struct {
		name, lang, tag, want string
	}{
		{"default lang", "", "go", "/tags/go"},
		{"default lang space", "", "machine learning", "/tags/machine%20learning"},
		{"default lang explicit", "en-US", "go", "/tags/go"},
		{"non-default lang", "fr", "go", "/fr/tags/go"},
		{"non-default lang space", "de", "machine learning", "/de/tags/machine%20learning"},
		{"unicode tag", "", "café", "/tags/caf%C3%A9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := fn(tt.lang, tt.tag); got != tt.want {
				t.Errorf("tagURL(%q, %q) = %q, want %q", tt.lang, tt.tag, got, tt.want)
			}
		})
	}
}

func TestPageTags(t *testing.T) {
	t.Parallel()

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, fstest.MapFS{})
	fn := r.funcMap()["pageTags"].(func(meta map[string]any) []string)

	tests := []struct {
		name string
		meta map[string]any
		want []string
	}{
		{"nil", nil, nil},
		{"missing key", map[string]any{"title": "x"}, nil},
		{"yaml-style []any", map[string]any{"tags": []any{"go", "docs"}}, []string{"go", "docs"}},
		{"already []string", map[string]any{"tags": []string{"go", "docs"}}, []string{"go", "docs"}},
		{"non-string entries skipped", map[string]any{"tags": []any{"go", 42, "docs"}}, []string{"go", "docs"}},
		{"non-list value", map[string]any{"tags": "go"}, nil},
		// Normalization must match the metadata index (REVIEW §9.4): lowercase,
		// trim, drop slash tags, dedup — so chip labels and tagURL links resolve.
		{"lowercased and trimmed", map[string]any{"tags": []any{"Deployment", " Ops "}}, []string{"deployment", "ops"}},
		{"slash tags dropped", map[string]any{"tags": []any{"team/x", "ok"}}, []string{"ok"}},
		{"duplicates removed", map[string]any{"tags": []any{"go", "Go", " go "}}, []string{"go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fn(tt.meta)
			if !slices.Equal(got, tt.want) {
				t.Errorf("pageTags(%v) = %v, want %v", tt.meta, got, tt.want)
			}
		})
	}
}

func TestRender_TagChips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tags        []any
		featureOff  bool
		wantContain []string
		wantNot     []string
	}{
		{
			name:        "renders chips for tags",
			tags:        []any{"go", "docs"},
			wantContain: []string{`href="/tags/go"`, `href="/tags/docs"`, `>go<`, `>docs<`, `class="tag-chips"`},
		},
		{
			name:       "no chips when feature off",
			tags:       []any{"go"},
			featureOff: true,
			wantNot:    []string{"tag-chips"},
		},
		{
			name:    "no chips when no tags",
			tags:    nil,
			wantNot: []string{"tag-chips"},
		},
		{
			name:        "non-default language scopes URL",
			tags:        []any{"go"},
			wantContain: []string{`href="/fr/tags/go"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
					`<!doctype html><html><body>{{ template "tag-chips" . }}</body></html>`,
				)},
				"assets/themes/default/partials/tag-chips.html.tmpl": {Data: tagChipsPartialBytes(t)},
			}
			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			page := PageContext{Path: "/x.md", Meta: map[string]any{"tags": tt.tags}}
			if tt.featureOff {
				page.Features = map[string]bool{"tag_chips": false}
			}
			tc := &TemplateContext{Site: &siteConfig, Page: page}
			if tt.name == "non-default language scopes URL" {
				tc.WithI18n("fr", nil, nil)
			}

			out, err := r.Render(context.Background(), "default.html.tmpl", tc)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			s := string(out)
			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("output missing %q\noutput: %s", want, s)
				}
			}
			for _, banned := range tt.wantNot {
				if strings.Contains(s, banned) {
					t.Errorf("output should not contain %q\noutput: %s", banned, s)
				}
			}
		})
	}
}

// tagChipsPartialBytes loads the real partial from the embedded theme.
// Failing if it doesn't exist forces Step 3 of this task to create it.
func tagChipsPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tag-chips.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}

func TestRenderTagPage(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body><main>{{ .Page.Content }}</main></body></html>`,
		)},
		"assets/themes/default/partials/tags-list.html.tmpl": {Data: tagsListPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"
	r := NewHTMLRenderer(&siteConfig, testFS)

	pages := []metadata.PageInfo{
		{Path: "/guide.md", Title: "Guide", Description: "Setup walkthrough"},
		{Path: "/api.md", Title: "API"},
	}
	tFunc := func(k string) string {
		return map[string]string{
			"tags_tagged_as": "Pages tagged %s",
			"tags_empty":     "No pages tagged %s",
		}[k]
	}

	out, err := r.RenderTagPage(context.Background(), "" /* default lang */, tFunc, "go", pages)
	if err != nil {
		t.Fatalf("RenderTagPage: %v", err)
	}

	s := string(out)
	for _, want := range []string{
		`href="/guide.md"`, ">Guide<",
		`href="/api.md"`, ">API<",
		"Setup walkthrough",
		"Pages tagged go",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\noutput: %s", want, s)
		}
	}
}

func TestRenderTagPage_Empty(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ .Page.Content }}</body></html>`,
		)},
		"assets/themes/default/partials/tags-list.html.tmpl": {Data: tagsListPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tFunc := func(k string) string { return map[string]string{"tags_empty": "No pages tagged %s"}[k] }
	out, err := r.RenderTagPage(context.Background(), "", tFunc, "missing", nil)
	if err != nil {
		t.Fatalf("RenderTagPage empty: %v", err)
	}
	if !strings.Contains(string(out), "No pages tagged missing") {
		t.Errorf("expected empty-state string, got %s", out)
	}
}

func tagsListPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tags-list.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}

func TestRenderTagsIndex(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ .Page.Content }}</body></html>`,
		)},
		"assets/themes/default/partials/tags-index.html.tmpl": {Data: tagsIndexPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tags := []TagCount{
		{Tag: "go", Count: 4},
		{Tag: "kubernetes", Count: 1},
		{Tag: "tutorial", Count: 7},
	}

	tFunc := func(k string) string { return map[string]string{"tags_index_title": "All tags"}[k] }
	out, err := r.RenderTagsIndex(context.Background(), "", tFunc, tags)
	if err != nil {
		t.Fatalf("RenderTagsIndex: %v", err)
	}

	s := string(out)
	for _, want := range []string{
		"All tags",
		`href="/tags/go"`, ">go<", "(4)",
		`href="/tags/kubernetes"`, "(1)",
		`href="/tags/tutorial"`, "(7)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\noutput: %s", want, s)
		}
	}
}

func TestRenderTagsIndex_Empty(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ .Page.Content }}</body></html>`,
		)},
		"assets/themes/default/partials/tags-index.html.tmpl": {Data: tagsIndexPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tFunc := func(k string) string { return map[string]string{"tags_index_title": "All tags"}[k] }
	out, err := r.RenderTagsIndex(context.Background(), "", tFunc, nil)
	if err != nil {
		t.Fatalf("RenderTagsIndex empty: %v", err)
	}
	if !strings.Contains(string(out), "All tags") {
		t.Errorf("expected header in empty state, got %s", out)
	}
}

func tagsIndexPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/tags-index.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}

func TestRender_SeeAlso(t *testing.T) {
	t.Parallel()

	related := []enricher.RelatedDoc{
		{Path: "/api.md", Title: "API"},
		{Path: "/guide.md", Title: "Guide"},
	}

	tests := []struct {
		name        string
		related     []enricher.RelatedDoc
		featureOff  bool
		wantContain []string
		wantNot     []string
	}{
		{
			name:        "renders section for non-empty related list",
			related:     related,
			wantContain: []string{`class="see-also"`, `>See also<`, `href="/api.md"`, `href="/guide.md"`, `>API<`, `>Guide<`},
		},
		{
			name:       "no section when feature off",
			related:    related,
			featureOff: true,
			wantNot:    []string{"see-also"},
		},
		{
			name:    "no section when no related",
			related: nil,
			wantNot: []string{"see-also"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
					`<!doctype html><html><body>{{ template "see-also" . }}</body></html>`,
				)},
				"assets/themes/default/partials/see-also.html.tmpl": {Data: seeAlsoPartialBytes(t)},
			}
			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			page := PageContext{Path: "/x.md", RelatedDocs: tt.related}
			if tt.featureOff {
				page.Features = map[string]bool{"see_also": false}
			}
			tc := &TemplateContext{Site: &siteConfig, Page: page}
			tFunc := func(k string) string { return map[string]string{"see_also": "See also"}[k] }
			tc.WithI18n("", tFunc, nil)

			out, err := r.Render(context.Background(), "default.html.tmpl", tc)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			s := string(out)
			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("output missing %q\noutput: %s", want, s)
				}
			}
			for _, banned := range tt.wantNot {
				if strings.Contains(s, banned) {
					t.Errorf("output should not contain %q\noutput: %s", banned, s)
				}
			}
		})
	}
}

// TestRender_SeeAlso_ContentURL proves see-also links are resolved through the
// resolver like every other link, so on a strip_extensions site they point to
// the clean URL (/api) rather than the raw index path (/api.md). Regression
// test for REVIEW.md §9.2.
func TestRender_SeeAlso_ContentURL(t *testing.T) {
	t.Parallel()

	contentFS := fstest.MapFS{
		"api.md":   {Data: []byte("# API")},
		"guide.md": {Data: []byte("# Guide")},
	}
	resolver := resolve.Build(contentFS, []string{".md"}, func(string) bool { return true })

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(
			`<!doctype html><html><body>{{ template "see-also" . }}</body></html>`,
		)},
		"assets/themes/default/partials/see-also.html.tmpl": {Data: seeAlsoPartialBytes(t)},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS, WithResolver(resolver))

	page := PageContext{Path: "/x.md", RelatedDocs: []enricher.RelatedDoc{
		{Path: "/api.md", Title: "API"},
		{Path: "/guide.md", Title: "Guide"},
	}}
	tc := &TemplateContext{Site: &siteConfig, Page: page}
	tc.WithI18n("", func(k string) string { return map[string]string{"see_also": "See also"}[k] }, nil)

	out, err := r.Render(context.Background(), "default.html.tmpl", tc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	for _, want := range []string{`href="/api"`, `href="/guide"`} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q\noutput: %s", want, s)
		}
	}
	for _, banned := range []string{`href="/api.md"`, `href="/guide.md"`} {
		if strings.Contains(s, banned) {
			t.Errorf("output should not contain raw path %q\noutput: %s", banned, s)
		}
	}
}

func seeAlsoPartialBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../cmd/gomddoc/assets/themes/default/partials/see-also.html.tmpl")
	if err != nil {
		t.Fatalf("partial not yet created: %v", err)
	}
	return data
}

// TestExecutePartial_Caching proves the parsed partial is cached when caching is
// enabled (so tag pages do not re-parse per request) and re-read otherwise.
// Regression test for REVIEW.md §9.3.
func TestExecutePartial_Caching(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/partials/probe.html.tmpl": {
			Data: []byte(`{{ define "probe" }}P:{{ . }}{{ end }}`),
		},
	}
	siteConfig := config.NewSiteConfig(".")

	t.Run("cached when enabled", func(t *testing.T) {
		t.Parallel()
		r := NewHTMLRenderer(&siteConfig, testFS, WithCache(&CachedTemplateStore{}))
		out, err := r.executePartial("probe", "x")
		if err != nil {
			t.Fatalf("executePartial: %v", err)
		}
		if string(out) != "P:x" {
			t.Fatalf("output = %q, want %q", out, "P:x")
		}
		t1, ok := r.partialCache.Load("default/probe")
		if !ok {
			t.Fatal("partial not cached after first render")
		}
		// Second call must reuse the same parsed template.
		if _, err := r.executePartial("probe", "y"); err != nil {
			t.Fatalf("executePartial 2: %v", err)
		}
		t2, _ := r.partialCache.Load("default/probe")
		if t1 != t2 {
			t.Error("partial re-parsed despite caching being enabled")
		}
		r.ClearCache()
		if _, ok := r.partialCache.Load("default/probe"); ok {
			t.Error("ClearCache did not clear the partial cache")
		}
	})

	t.Run("not cached when disabled", func(t *testing.T) {
		t.Parallel()
		r := NewHTMLRenderer(&siteConfig, testFS) // no WithCache => dev mode
		if _, err := r.executePartial("probe", "x"); err != nil {
			t.Fatalf("executePartial: %v", err)
		}
		if _, ok := r.partialCache.Load("default/probe"); ok {
			t.Error("partial cached even though caching is disabled (dev mode)")
		}
	})
}
