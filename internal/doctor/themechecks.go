package doctor

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/template"
)

// scanTheme is the active theme's template scan, or an unavailable one when
// there are no assets to scan.
func scanTheme(ins config.Inspection, assets fs.FS) template.ThemeInfo {
	name := ins.Config.Site.Theme.Name
	if assets == nil {
		return template.ThemeInfo{Name: name, Source: template.ThemeSourceUnavailable}
	}
	return template.ThemeFeatures(assets, name)
}

// themeChecks compares the configured theme, feature toggles and vars with
// what the theme's templates actually read. The scan knows names only, so an
// unknown toggle is a warning, not an error.
func themeChecks(ins config.Inspection, scan template.ThemeInfo) []diag.Finding {
	if scan.Source == template.ThemeSourceUnavailable {
		return nil
	}
	site := ins.Config.Site
	var findings []diag.Finding
	if scan.Source == template.ThemeSourceFallbackDefault {
		findings = append(findings, diag.New("theme.not-installed", configFileIf(ins, "theme.name"), ins.Line("theme.name"), "theme.name",
			fmt.Sprintf("theme %q is not installed; the default theme renders", site.Theme.Name),
			fmt.Sprintf("install it under .gomddoc/assets/themes/%s/ (see gomddoc-themes), or set theme.name: default", site.Theme.Name)))
	}
	for _, k := range slices.Sorted(maps.Keys(site.Theme.Features)) {
		key := "theme.features." + k
		if f, ok := unknownFeature(k, scan, configFileIf(ins, key), ins.Line(key), key); ok {
			findings = append(findings, f)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(site.Theme.Vars)) {
		if slices.Contains(scan.Vars, k) {
			continue
		}
		key := "theme.vars." + k
		fix := "the theme's CSS reads: " + strings.Join(scan.Vars, ", ")
		if best, ok := closest(k, scan.Vars, 2); ok {
			fix = fmt.Sprintf("did you mean `%s`?", best)
		}
		findings = append(findings, diag.New("theme.unknown-var", configFileIf(ins, key), ins.Line(key), key,
			fmt.Sprintf("theme %q never reads --theme-%s", scan.Name, k), fix))
	}
	return findings
}

// unknownFeature is the finding for a feature toggle the scanned theme never
// reads; page frontmatter reuses it.
func unknownFeature(k string, scan template.ThemeInfo, file string, line int, key string) (diag.Finding, bool) {
	if scan.Source == template.ThemeSourceUnavailable || slices.Contains(scan.Features, k) {
		return diag.Finding{}, false
	}
	fix := "the theme reads: " + strings.Join(scan.Features, ", ")
	if best, ok := closest(k, scan.Features, 2); ok {
		fix = fmt.Sprintf("did you mean `%s`?", best)
	}
	return diag.New("theme.unknown-feature", file, line, key,
		fmt.Sprintf("theme %q never reads the feature toggle %q", scan.Name, k), fix), true
}

// configFileIf is the config file when it sets key, else "" (the value came
// from an env var).
func configFileIf(ins config.Inspection, key string) string {
	if ins.Line(key) == 0 {
		return ""
	}
	return config.ConfigFile
}

// excludeMatchesNothing reports exclude patterns that match no path under
// content — usually a typo, which leaves the content it meant to hide
// published. Hidden paths are skipped: they are never served anyway.
func excludeMatchesNothing(ins config.Inspection, content fs.FS) []diag.Finding {
	patterns := ins.Config.Site.Exclude
	if content == nil || len(patterns) == 0 {
		return nil
	}
	matched := make([]bool, len(patterns))
	_ = fs.WalkDir(content, ".", func(p string, d fs.DirEntry, err error) error {
		// Best effort: an unreadable subtree only narrows what can match.
		if err != nil || p == "." {
			return nil
		}
		if provider.IsHiddenPath(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		for i, pat := range patterns {
			if !matched[i] && provider.IsExcludedPath(p, []string{pat}) {
				matched[i] = true
			}
		}
		return nil
	})
	var findings []diag.Finding
	for i, pat := range patterns {
		if matched[i] {
			continue
		}
		findings = append(findings, diag.New("exclude.matches-nothing", configFileIf(ins, "exclude"), ins.Line("exclude"), pat,
			fmt.Sprintf("exclude pattern %q matches no file or directory", pat),
			"check the pattern: a trailing / matches a directory, a pattern without / matches any path segment"))
	}
	return findings
}
