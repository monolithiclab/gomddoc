package doctor

import (
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// embeddedAssets has the binary's assets/themes/default/ layout.
var embeddedAssets = os.DirFS("../../cmd/gomddoc")

func TestThemeChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yml  string
		want []string
	}{
		{"default theme, known toggles", "theme:\n  name: default\n  features: {toc: false}\n  vars: {primary: '#000'}\n", nil},
		{"not installed", "theme:\n  name: nord\n", []string{"theme.not-installed .gomddoc/config.yml:2 theme.name"}},
		{"unknown feature and var", "theme:\n  features:\n    serach: false\n  vars:\n    primray: red\n",
			[]string{"theme.unknown-feature .gomddoc/config.yml:3 theme.features.serach", "theme.unknown-var .gomddoc/config.yml:5 theme.vars.primray"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ins := inspect(t, tt.yml)
			got := themeChecks(ins, scanTheme(ins, embeddedAssets))
			if !slices.Equal(ids(got), tt.want) {
				t.Errorf("findings = %q, want %q", ids(got), tt.want)
			}
			for _, f := range got {
				if f.Code == "theme.unknown-feature" && !strings.Contains(f.Fix, "search") {
					t.Errorf("fix %q should suggest search", f.Fix)
				}
			}
		})
	}
	// No assets: nothing to compare against, nothing reported.
	if got := themeChecks(inspect(t, "theme: {features: {serach: false}}\n"), scanTheme(inspect(t, ""), nil)); got != nil {
		t.Errorf("no assets: %q", ids(got))
	}
}

func TestExcludeMatchesNothing(t *testing.T) {
	t.Parallel()
	content := fstest.MapFS{
		"README.md":        {Data: []byte("# Home")},
		"drafts/wip.md":    {Data: []byte("# WIP")},
		"guide/a.draft.md": {Data: []byte("# A")},
		".gomddoc/x.md":    {Data: []byte("hidden")},
	}
	ins := inspect(t, "exclude:\n  - drafts/\n  - \"*.draft.md\"\n  - archive/\n  - x.md\n")
	got := excludeMatchesNothing(ins, content)
	want := []string{"exclude.matches-nothing .gomddoc/config.yml:1 archive/", "exclude.matches-nothing .gomddoc/config.yml:1 x.md"}
	if !slices.Equal(ids(got), want) {
		t.Errorf("findings = %q, want %q", ids(got), want)
	}
}

// TestThemeChecks_FeatureFromEnv: a toggle set by env var has no place in the
// config file, so the finding names the variable's key without a file.
// No t.Parallel: t.Setenv.
func TestThemeChecks_FeatureFromEnv(t *testing.T) {
	t.Setenv("GOMDDOC_SITE_THEME_FEATURES_SERACH", "false")
	ins := inspect(t, "")
	got := themeChecks(ins, scanTheme(ins, embeddedAssets))
	if want := []string{"theme.unknown-feature :0 theme.features.serach"}; !slices.Equal(ids(got), want) {
		t.Errorf("findings = %q, want %q", ids(got), want)
	}
}
