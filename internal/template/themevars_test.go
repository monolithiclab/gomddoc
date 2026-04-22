package template

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestBuildThemeVarsCSS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		vars         map[string]string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "nil vars produces empty CSS",
			vars:      nil,
			wantEmpty: true,
		},
		{
			name:      "empty vars produces empty CSS",
			vars:      map[string]string{},
			wantEmpty: true,
		},
		{
			name: "vars become --theme- custom properties",
			vars: map[string]string{
				"bg":      "#fafafa",
				"primary": "#e63946",
			},
			wantContains: []string{
				":root {",
				"--theme-bg: #fafafa",
				"--theme-primary: #e63946",
			},
		},
		{
			name: "dark- prefix preserved in variable name",
			vars: map[string]string{
				"dark-primary": "#ff0000",
				"dark-bg":      "#111111",
			},
			wantContains: []string{
				"--theme-dark-bg: #111111",
				"--theme-dark-primary: #ff0000",
			},
		},
		{
			name: "multiple vars all present",
			vars: map[string]string{
				"primary": "#e63946",
				"bg":      "#fafafa",
				"text":    "#333",
			},
			wantContains: []string{
				"--theme-bg: #fafafa",
				"--theme-primary: #e63946",
				"--theme-text: #333",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			css := string(buildThemeVarsCSS(tt.vars))

			if tt.wantEmpty {
				if css != "" {
					t.Errorf("expected empty CSS, got:\n%s", css)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(css, want) {
					t.Errorf("CSS should contain %q, got:\n%s", want, css)
				}
			}
		})
	}
}

func TestGenerateThemeVarsCSS_InTemplate(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`<style>{{ themeVarsCSS }}</style>`),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Theme.Vars = map[string]string{
		"primary":      "#e63946",
		"dark-primary": "#ff6b6b",
	}
	r := NewHTMLRenderer(&siteConfig, testFS)

	ctx := &TemplateContext{
		Site: &siteConfig,
		Page: PageContext{Path: "/"},
	}

	result, err := r.Render(t.Context(), "default.html.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "--theme-primary: #e63946") {
		t.Error("Expected rendered template to contain --theme-primary")
	}
	if !strings.Contains(output, "--theme-dark-primary: #ff6b6b") {
		t.Error("Expected rendered template to contain --theme-dark-primary")
	}
}

func TestGenerateThemeVarsCSS_Caching(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`{{ themeVarsCSS }}`),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Theme.Vars = map[string]string{"primary": "#2563eb"}
	r := NewHTMLRenderer(&siteConfig, testFS)

	css1 := r.generateThemeVarsCSS()
	css2 := r.generateThemeVarsCSS()

	if css1 != css2 {
		t.Errorf("Cached CSS should be identical: %q vs %q", css1, css2)
	}
}

func TestGenerateThemeVarsCSS_NoVars(t *testing.T) {
	t.Parallel()

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(`[{{ themeVarsCSS }}]`),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	r := NewHTMLRenderer(&siteConfig, testFS)

	css := r.generateThemeVarsCSS()
	if css != "" {
		t.Errorf("Expected empty CSS when no vars configured, got %q", css)
	}
}
