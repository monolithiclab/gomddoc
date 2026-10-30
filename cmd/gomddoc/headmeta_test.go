package main

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

// reDescription pulls the content of <meta name="description">. The og: and
// twitter: variants are property=/name= on different attributes with a prefixed
// value, so anchoring on name="description" cannot pick them up by accident.
var reDescription = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)

// TestHeadMeta_DescriptionPrefersPageFrontmatter renders through the *embedded*
// default theme, not a stub, because the defect was in the theme partial:
// head-shared.html.tmpl emitted the site description into
// <meta name="description"> unconditionally while og:description and
// twitter:description right above it already preferred the page's own. Every
// page on a site therefore advertised the same snippet to search engines.
//
// Asserting the tag is present is not enough — that passed throughout. The
// assertion has to read the content back and, in the frontmatter case, deny the
// site description.
func TestHeadMeta_DescriptionPrefersPageFrontmatter(t *testing.T) {
	t.Parallel()

	const siteDesc = "Site-wide description"

	tests := []struct {
		name     string
		pageDesc any
		want     string
	}{
		{"page frontmatter wins", "Page-specific description", "Page-specific description"},
		{"no frontmatter falls back to the site", nil, siteDesc},
		{"empty frontmatter falls back to the site", "", siteDesc},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			siteConfig := config.NewSiteConfig(".")
			siteConfig.Meta.Description = siteDesc
			meta := map[string]any{"title": "A Page"}
			if tt.pageDesc != nil {
				meta["description"] = tt.pageDesc
			}
			out := renderDefaultPage(t, &siteConfig, meta)

			m := reDescription.FindSubmatch(out)
			if m == nil {
				t.Fatalf(`no <meta name="description"> in output`)
			}
			if got := string(m[1]); got != tt.want {
				t.Errorf("description = %q, want %q", got, tt.want)
			}
			// The site description leaking back in through og:/twitter: would
			// mean the shared $desc was not shared after all.
			if tt.want != siteDesc && strings.Contains(string(out), siteDesc) {
				t.Errorf("site description %q still appears; the page's own should have replaced it everywhere", siteDesc)
			}
		})
	}
}

// renderDefaultPage renders one page at /page through the embedded default
// theme with the given frontmatter.
func renderDefaultPage(t *testing.T, siteConfig *config.SiteConfig, meta map[string]any) []byte {
	t.Helper()
	pageCtx := tmpl.BuildPageContext(tmpl.PageContextInput{
		Site:       siteConfig,
		Path:       "/page",
		Content:    "<p>body</p>",
		Enrichment: &enricher.EnrichmentData{Metadata: meta},
	})
	out, err := tmpl.NewHTMLRenderer(siteConfig, embeddedAssets).Render(context.Background(), "default.html.tmpl", pageCtx)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return out
}

// TestHead_IconsAndThemeColor renders the embedded default theme. head-meta,
// which every theme calls, links the shared icon set through /_assets/ (the
// only path build writes it to) and the generated manifest; the theme's head
// sets theme-color to its backgrounds, or to the site's theme.vars.
func TestHead_IconsAndThemeColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		vars        map[string]string
		light, dark string
	}{
		{name: "theme defaults", light: "#ffffff", dark: "#13111C"},
		{name: "theme.vars override", vars: map[string]string{"bg": "#fafafa", "dark-bg": "#000000"}, light: "#fafafa", dark: "#000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			siteConfig := config.NewSiteConfig(".")
			siteConfig.Theme.Vars = tt.vars
			out := string(renderDefaultPage(t, &siteConfig, map[string]any{"title": "A Page"}))
			for _, want := range []string{
				`<link rel="icon" href="/_assets/favicon.ico" sizes="48x48">`,
				`<link rel="icon" href="/_assets/favicon.svg" sizes="any" type="image/svg+xml">`,
				`<link rel="apple-touch-icon" href="/_assets/apple-touch-icon.png">`,
				`<link rel="manifest" href="/site.webmanifest" crossorigin="use-credentials">`,
				`<meta name="theme-color" content="` + tt.light + `" media="(prefers-color-scheme: light)">`,
				`<meta name="theme-color" content="` + tt.dark + `" media="(prefers-color-scheme: dark)">`,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("head lacks %s", want)
				}
			}
			if n := strings.Count(out, `rel="icon"`); n != 2 {
				t.Errorf("want exactly the two icon links, got %d", n)
			}
		})
	}
}
