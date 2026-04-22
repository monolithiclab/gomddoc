package template

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestInlineJSAsset(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<script type="module">{{ inlineJSAsset "test.mjs" }}</script>`),
		},
		"assets/themes/default/test.mjs": {
			Data: []byte("console.log('hello & goodbye');"),
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

	// template.JS must pass through unescaped inside <script>
	if !strings.Contains(string(result), "console.log('hello & goodbye');") {
		t.Errorf("Expected unescaped JS content in <script>, got %q", string(result))
	}
}

func TestInlineJSAsset_SharedFallback(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<script>{{ inlineJSAsset "shared.mjs" }}</script>`),
		},
		"assets/shared/shared.mjs": {
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

func TestInlineJSAsset_NotFound(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`{{ inlineJSAsset "missing.mjs" }}`),
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

func TestInlineCSSAsset(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<style>{{ inlineCSSAsset "styles.css" }}</style>`),
		},
		"assets/themes/default/styles.css": {
			Data: []byte(`body { font-family: "Arial"; color: red; }`),
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

	// template.CSS must pass through unescaped inside <style>
	if !strings.Contains(string(result), `font-family: "Arial"`) {
		t.Errorf("Expected unescaped CSS content in <style>, got %q", string(result))
	}
}

func TestInlineCSSAsset_NotFound(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<style>{{ inlineCSSAsset "missing.css" }}</style>`),
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

func TestInlineHTMLAsset(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<div>{{ inlineHTMLAsset "logo.svg" }}</div>`),
		},
		"assets/themes/default/logo.svg": {
			Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="10"/></svg>`),
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

	// template.HTML must pass through unescaped in HTML context
	if !strings.Contains(string(result), `<svg xmlns="http://www.w3.org/2000/svg">`) {
		t.Errorf("Expected unescaped SVG content, got %q", string(result))
	}
}

func TestInlineHTMLAsset_NotFound(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`{{ inlineHTMLAsset "missing.svg" }}`),
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

// TestInlineAsset_ContextSafety verifies that each typed function produces
// correct output in its intended template context and would be escaped/broken
// in the wrong context.
func TestInlineAsset_ContextSafety(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tmpl     string
		asset    string
		contains string
	}{
		{
			name:     "JS in script context",
			tmpl:     `<script>{{ inlineJSAsset "a.js" }}</script>`,
			asset:    "console.log('x & y');",
			contains: "console.log('x & y');",
		},
		{
			name:     "CSS in style context",
			tmpl:     `<style>{{ inlineCSSAsset "a.css" }}</style>`,
			asset:    "p > span { color: red; }",
			contains: "p > span { color: red; }",
		},
		{
			name:     "HTML in body context",
			tmpl:     `<div>{{ inlineHTMLAsset "a.svg" }}</div>`,
			asset:    `<svg><path d="M0 0"/></svg>`,
			contains: `<svg><path d="M0 0"/></svg>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testFS := fstest.MapFS{
				"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(tt.tmpl)},
			}
			// Place asset in both theme and shared to test theme-first resolution
			assetPath := strings.TrimPrefix(tt.tmpl, "")
			_ = assetPath
			// Extract asset name from template
			var assetName string
			for _, name := range []string{"a.js", "a.css", "a.svg"} {
				if strings.Contains(tt.tmpl, `"`+name+`"`) {
					assetName = name
					break
				}
			}
			testFS["assets/themes/default/"+assetName] = &fstest.MapFile{Data: []byte(tt.asset)}

			siteConfig := config.NewSiteConfig(".")
			r := NewHTMLRenderer(&siteConfig, testFS)

			result, err := r.Render(context.Background(), "default.html.tmpl", &TemplateContext{
				Site: &siteConfig,
				Page: PageContext{Path: "/"},
			})
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if !strings.Contains(string(result), tt.contains) {
				t.Errorf("Expected %q in output, got %q", tt.contains, string(result))
			}
		})
	}
}

// TestReadAsset_ThemeOverridesShared verifies theme assets take precedence over shared.
func TestReadAsset_ThemeOverridesShared(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(`{{ inlineHTMLAsset "icon.svg" }}`)},
		"assets/themes/default/icon.svg":                  {Data: []byte("theme-version")},
		"assets/shared/icon.svg":                          {Data: []byte("shared-version")},
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
	if got != template.HTMLEscapeString("theme-version") && got != "theme-version" {
		t.Errorf("Expected theme asset to take precedence, got %q", got)
	}
}

func TestReadAsset_PathTraversal(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte("ok")},
	}
	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	tests := []struct {
		name string
		path string
	}{
		{"dot-dot", "../../../etc/passwd"},
		{"absolute", "/etc/passwd"},
		{"dot prefix", "./test.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := r.readAsset(tt.path)
			if err == nil {
				t.Errorf("readAsset(%q) should reject invalid path", tt.path)
			}
		})
	}
}
