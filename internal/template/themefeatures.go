package template

import (
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
)

// Where ThemeInfo's lists come from.
const (
	ThemeSourceTemplateScan = "template-scan"
	ThemeSourceUnavailable  = "unavailable"
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

// ThemeFeatures scans a theme's templates for {{ .Feature "x" }} and its
// templates and CSS for var(--theme-x). It is best effort — names only, no
// descriptions or defaults — until themes declare their features (see
// docs/plans/2026-09-23-theme-manifest-parked.md). assets is the asset FS the
// renderer reads, so a site-level override is seen. A theme that is not
// installed, or has no templates, is reported unavailable rather than as an
// error: the caller is describing, not rendering.
func ThemeFeatures(assets fs.FS, name string) ThemeInfo {
	info := ThemeInfo{Name: name, Source: ThemeSourceUnavailable, Features: []string{}, Vars: []string{}}
	features, vars := map[string]struct{}{}, map[string]struct{}{}
	templates := 0
	err := fs.WalkDir(assets, path.Join("assets", "themes", name), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := path.Ext(p)
		if ext != ".tmpl" && ext != ".css" {
			return nil
		}
		data, err := fs.ReadFile(assets, p)
		if err != nil {
			return err
		}
		if ext == ".tmpl" {
			templates++
			for _, m := range featureCall.FindAllSubmatch(data, -1) {
				features[string(m[1])] = struct{}{}
			}
		}
		for _, m := range themeVarUse.FindAllSubmatch(data, -1) {
			vars[string(m[1])] = struct{}{}
		}
		return nil
	})
	if err != nil || templates == 0 {
		return info
	}
	info.Source = ThemeSourceTemplateScan
	info.Features = slices.Sorted(maps.Keys(features))
	info.Vars = slices.Sorted(maps.Keys(vars))
	return info
}
