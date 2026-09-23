package template

import (
	"io/fs"
	"os"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/provider"
)

func TestThemeFeatures(t *testing.T) {
	t.Parallel()
	// Same assets/themes/default/ layout as the binary's embed.
	embedded := os.DirFS("../../cmd/gomddoc")
	overlay := provider.NewOverlayFS(fstest.MapFS{
		"assets/themes/default/partials/banner.html.tmpl": {Data: []byte(`{{ if .Feature "banner" }}<div style="color: var(--theme-banner-fg)"></div>{{ end }}`)},
	}, embedded)

	wantDefault := ThemeInfo{
		Name:   "default",
		Source: ThemeSourceTemplateScan,
		Features: []string{"admonitions", "code_copy", "color_chips", "dark_mode", "heading_anchors", "katex",
			"mermaid", "search", "see_also", "tag_chips", "toc"},
		Vars: []string{"bg", "dark-bg", "dark-primary", "dark-text", "primary", "text"},
	}
	withBanner := wantDefault
	withBanner.Features = slices.Insert(slices.Clone(wantDefault.Features), 1, "banner")
	withBanner.Vars = slices.Insert(slices.Clone(wantDefault.Vars), 0, "banner-fg")
	unavailable := func(name string) ThemeInfo {
		return ThemeInfo{Name: name, Source: ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}
	}

	// A non-default theme that ships only a layout and one partial of its own,
	// as the gomddoc-themes themes do: every other partial comes from default.
	child := provider.NewOverlayFS(fstest.MapFS{
		"assets/themes/child/layouts/default.html.tmpl": {Data: []byte(`{{ template "toc" . }}{{ if .Feature "child_only" }}{{ end }}`)},
		"assets/themes/child/partials/toc.html.tmpl":    {Data: []byte(`{{ define "toc" }}{{ if .Feature "toc" }}<style>a{color:var(--theme-child-accent)}</style>{{ end }}{{ end }}`)},
	}, embedded)
	withChild := wantDefault
	withChild.Name = "child"
	withChild.Features = slices.Insert(slices.Clone(wantDefault.Features), 1, "child_only")
	withChild.Vars = slices.Insert(slices.Clone(wantDefault.Vars), 1, "child-accent")

	// .gomddoc/partials/ lands at partials/ in the overlay and overrides every theme.
	sitePartial := provider.NewOverlayFS(fstest.MapFS{
		"partials/banner.html.tmpl": {Data: []byte(`{{ define "banner" }}{{ if .Feature "site_banner" }}{{ end }}{{ end }}`)},
	}, embedded)
	withSitePartial := wantDefault
	withSitePartial.Features = slices.Concat(wantDefault.Features[:9], []string{"site_banner"}, wantDefault.Features[9:])

	// Not installed: the renderer falls back to default, so do its features.
	fallback := wantDefault
	fallback.Name = "nord"
	fallback.Source = ThemeSourceFallbackDefault

	tests := []struct {
		name   string
		assets fs.FS
		theme  string
		want   ThemeInfo
	}{
		{"embedded default", embedded, "default", wantDefault},
		{"site overlay adds a feature and a var", overlay, "default", withBanner},
		{"non-default theme inherits default partials", child, "child", withChild},
		{"site-level partials", sitePartial, "default", withSitePartial},
		{"theme not installed falls back to default", embedded, "nord", fallback},
		{"nothing installed at all", fstest.MapFS{"assets/themes/empty/static/x.css": {Data: []byte("a{color:var(--theme-x)}")}}, "empty", unavailable("empty")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ThemeFeatures(tt.assets, tt.theme)
			if got.Name != tt.want.Name || got.Source != tt.want.Source ||
				!slices.Equal(got.Features, tt.want.Features) || !slices.Equal(got.Vars, tt.want.Vars) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
			if got.Features == nil || got.Vars == nil {
				t.Error("lists must be non-nil so they marshal as [] not null")
			}
		})
	}
}
