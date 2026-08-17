package template

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/testutil/fanout"
)

func TestBuildThemeVarsCSS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		vars           map[string]string
		wantContains   []string
		wantNotContain []string
		wantEmpty      bool
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
		{
			name: "key with spaces rejected",
			vars: map[string]string{
				"bg color": "#fff",
			},
			wantEmpty: true,
		},
		{
			name: "key with CSS injection rejected",
			vars: map[string]string{
				"bg: red; } body { background": "#000",
			},
			wantEmpty: true,
		},
		{
			name: "mixed valid and invalid keys",
			vars: map[string]string{
				"primary":  "#e63946",
				"evil key": "#000",
			},
			wantContains: []string{
				"--theme-primary: #e63946",
			},
			wantNotContain: []string{
				"evil",
			},
		},
		{
			name: "value with CSS injection via braces rejected",
			vars: map[string]string{
				"bg": "red; } body { display:none } :root {",
			},
			wantEmpty: true,
		},
		{
			name: "value with semicolon rejected",
			vars: map[string]string{
				"bg": "red; background: url(evil)",
			},
			wantEmpty: true,
		},
		{
			name: "value with HTML tag rejected",
			vars: map[string]string{
				"bg": "</style><script>alert(1)</script>",
			},
			wantEmpty: true,
		},
		{
			name: "safe value with parentheses allowed",
			vars: map[string]string{
				"font": "rgb(255, 0, 0)",
			},
			wantContains: []string{
				"--theme-font: rgb(255, 0, 0)",
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
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(css, notWant) {
					t.Errorf("CSS should NOT contain %q, got:\n%s", notWant, css)
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

// TestGenerateThemeVarsCSS_Concurrent covers the sync.Once from the state it is
// actually used in: the renderer is shared by every handler and the vars CSS is
// first built by whichever requests arrive together.
//
// buildThemeVarsCSS is a deterministic function of config the renderer already
// holds, so no count can tell one build from fifty. What this test provides is
// the concurrent execution that `make test`'s -race needs in order to flag the
// unsynchronized write, and a zero-value check for the other half of the same
// mutation — a caller losing an `if css == ""` race returns the zero value.
func TestGenerateThemeVarsCSS_Concurrent(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"primary": "#2563eb", "dark-primary": "#60a5fa"}
	// No layout in the FS: generateThemeVarsCSS reads Theme.Vars and never opens
	// a file. Nothing here renders.
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Theme.Vars = vars
	r := NewHTMLRenderer(&siteConfig, fstest.MapFS{})

	want := buildThemeVarsCSS(vars)
	if want == "" {
		t.Fatal("fixture produced no CSS; the concurrent assertion below would be vacuous")
	}

	fanout.Run(50, func(i int) {
		if got := r.generateThemeVarsCSS(); got != want {
			t.Errorf("goroutine %d got %q, want %q", i, got, want)
		}
	})
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
