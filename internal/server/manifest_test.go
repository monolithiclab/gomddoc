package server

import (
	"encoding/json"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestGenerateWebManifest(t *testing.T) {
	t.Parallel()

	var got webManifest
	site := config.NewSiteConfig(".")
	site.Meta = config.MetaConfig{Title: "Handbook", Description: "How we work."}
	data := GenerateWebManifest(site)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("manifest is not JSON: %v\n%s", err, data)
	}
	if got.Name != "Handbook" || got.ShortName != "Handbook" || got.Description != "How we work." {
		t.Errorf("name/short_name/description = %q/%q/%q, want the site's", got.Name, got.ShortName, got.Description)
	}
	if got.StartURL != "/" || got.Display != "standalone" || got.ThemeColor != "#2563eb" || got.BackgroundColor != "#ffffff" {
		t.Errorf("start_url/display/colours = %q/%q/%q/%q", got.StartURL, got.Display, got.ThemeColor, got.BackgroundColor)
	}
	var icons []string
	for _, i := range got.Icons {
		icons = append(icons, i.Src+" "+i.Sizes+" "+i.Purpose)
	}
	want := []string{
		"/_assets/android-chrome-192x192.png 192x192 any",
		"/_assets/android-chrome-512x512.png 512x512 any",
		"/_assets/maskable-icon-192x192.png 192x192 maskable",
		"/_assets/maskable-icon-512x512.png 512x512 maskable",
	}
	if !slices.Equal(icons, want) {
		t.Errorf("icons = %v, want %v", icons, want)
	}
}

func TestGenerateWebManifest_NoDescription(t *testing.T) {
	t.Parallel()

	var got map[string]any
	if err := json.Unmarshal(GenerateWebManifest(config.NewSiteConfig(".")), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["description"]; ok {
		t.Error("description present without a site description")
	}
}

// TestGenerateWebManifest_BackgroundFromThemeVars: the splash background is the
// page's, the same theme.vars bg the themes' theme-color meta reads.
func TestGenerateWebManifest_BackgroundFromThemeVars(t *testing.T) {
	t.Parallel()

	site := config.NewSiteConfig(".")
	site.Theme.Vars = map[string]string{"bg": "#fafafa"}
	var got webManifest
	if err := json.Unmarshal(GenerateWebManifest(site), &got); err != nil {
		t.Fatal(err)
	}
	if got.BackgroundColor != "#fafafa" {
		t.Errorf("background_color = %q, want theme.vars bg #fafafa", got.BackgroundColor)
	}
}

func TestWebManifestHandler(t *testing.T) {
	t.Parallel()

	assertRevalidates(t, revalidationCase{
		Handler:   NewWebManifestHandler(config.NewSiteConfig("."), nil),
		Path:      WebManifestPath,
		WantType:  mimeManifest,
		WantCache: cacheDynamic,
	})
}

// TestSiteWebManifest: a site or theme that ships its own site.webmanifest keeps
// it (its name and colours); otherwise the generated one is published.
func TestSiteWebManifest(t *testing.T) {
	t.Parallel()

	site := config.NewSiteConfig(".")
	site.Meta.Title = "Docs"
	own := []byte(`{"name":"Mine"}`)
	if got := SiteWebManifest(site, fstest.MapFS{"site.webmanifest": {Data: own}}); string(got) != string(own) {
		t.Errorf("with a shipped manifest got %s, want it unchanged", got)
	}
	generated := GenerateWebManifest(site)
	for name, staticFS := range map[string]fs.FS{
		"no static layer":           nil,
		"static without a manifest": fstest.MapFS{"favicon.ico": {}},
	} {
		if got := SiteWebManifest(site, staticFS); string(got) != string(generated) {
			t.Errorf("%s: got %s, want the generated manifest", name, got)
		}
	}
}
