package template

import (
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// Where ThemeInfo's lists come from.
const (
	ThemeSourceTemplateScan = "template-scan"
	ThemeSourceUnavailable  = "unavailable"
	// ThemeSourceFallbackDefault: the named theme is not installed, so the
	// renderer (and this scan) use the default theme.
	ThemeSourceFallbackDefault = "fallback-default"
)

// ThemeInfo lists the feature keys and CSS variables a theme reads.
type ThemeInfo struct {
	Name     string   `json:"active"`
	Source   string   `json:"source"`
	Features []string `json:"features"` // sorted, never nil
	Vars     []string `json:"vars"`     // sorted, never nil; theme.vars keys ("--theme-" stripped)
}

var (
	featureCall = regexp.MustCompile(`\.Feature\s+"([a-z][a-z0-9_]*)"`)
	themeVarUse = regexp.MustCompile(`var\(--theme-([a-z0-9-]+)`)
)

// ThemeFeatures reports the feature keys ({{ .Feature "x" }}) and CSS
// variables (var(--theme-x)) the named theme reads. It is best effort — names
// only, no descriptions or defaults — until themes declare their features (see
// docs/plans/2026-09-23-theme-manifest-parked.md).
//
// It scans the files the renderer would parse, not merely the theme's
// directory: parseThemeTemplate layers the default theme's partials under a
// non-default theme's own, then site partials (.gomddoc/partials/, at
// partials/ in the overlay) over both, and parseTemplate falls back to a
// default layout the theme lacks. A theme that ships six partials and
// inherits fourteen honours the features of all twenty. A theme that is not
// installed renders as default, and is reported that way. Only when not even
// the default theme has layouts is it unavailable — the caller is describing,
// not rendering, so that is not an error.
func ThemeFeatures(assets fs.FS, name string) ThemeInfo {
	info := ThemeInfo{Name: name, Source: ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}
	theme, source := name, ThemeSourceTemplateScan
	if !hasLayouts(assets, name) {
		if name == config.DefaultThemeName || !hasLayouts(assets, config.DefaultThemeName) {
			return info
		}
		theme, source = config.DefaultThemeName, ThemeSourceFallbackDefault
	}

	features, vars := map[string]struct{}{}, map[string]struct{}{}
	for _, p := range themeFiles(assets, theme) {
		data, err := fs.ReadFile(assets, p)
		if err != nil {
			continue // listed a moment ago; a vanished file only narrows a best-effort list
		}
		if path.Ext(p) == ".tmpl" {
			for _, m := range featureCall.FindAllSubmatch(data, -1) {
				features[string(m[1])] = struct{}{}
			}
		}
		for _, m := range themeVarUse.FindAllSubmatch(data, -1) {
			vars[string(m[1])] = struct{}{}
		}
	}
	info.Source = source
	info.Features = slices.Sorted(maps.Keys(features))
	info.Vars = slices.Sorted(maps.Keys(vars))
	return info
}

func hasLayouts(assets fs.FS, theme string) bool {
	matches, err := fs.Glob(assets, path.Join("assets", "themes", theme, "layouts", "*.html.tmpl"))
	return err == nil && len(matches) > 0
}

// themeFiles lists the templates the renderer resolves for theme, each
// logical file once (a later layer replaces an earlier one of the same name,
// as the renderer's later parse redefines it), plus the theme's own CSS.
func themeFiles(assets fs.FS, theme string) []string {
	resolved := map[string]string{} // "layouts/x.html.tmpl" → path that wins
	layer := func(dir, kind string) {
		matches, _ := fs.Glob(assets, path.Join(dir, "*.html.tmpl"))
		for _, m := range matches {
			resolved[kind+"/"+path.Base(m)] = m
		}
	}
	themeDir := func(t string) string { return path.Join("assets", "themes", t) }
	if theme != config.DefaultThemeName {
		layer(path.Join(themeDir(config.DefaultThemeName), "layouts"), "layouts")
		layer(path.Join(themeDir(config.DefaultThemeName), "partials"), "partials")
	}
	layer(path.Join(themeDir(theme), "layouts"), "layouts")
	layer(path.Join(themeDir(theme), "partials"), "partials")
	layer("partials", "partials")

	files := slices.Collect(maps.Values(resolved))
	_ = fs.WalkDir(assets, themeDir(theme), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(p) == ".css" {
			files = append(files, p)
		}
		return nil
	})
	slices.Sort(files)
	return files
}
