package template

import (
	"html/template"
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// themeVarsCache caches the generated CSS for the renderer's lifetime.
type themeVarsCache struct {
	once sync.Once
	css  template.CSS
}

// generateThemeVarsCSS returns CSS custom properties from config theme.vars.
// Each key becomes --theme-{key} in a :root block (e.g., "bg" → --theme-bg,
// "dark-bg" → --theme-dark-bg). Themes reference these with var() fallbacks.
func (h *HTMLRenderer) generateThemeVarsCSS() template.CSS {
	h.themeVars.once.Do(func() {
		h.themeVars.css = buildThemeVarsCSS(h.siteConfig.Theme.Vars)
	})
	return h.themeVars.css
}

// validThemeVarKey matches safe CSS custom property name suffixes:
// alphanumeric characters and hyphens only.
var validThemeVarKey = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

// unsafeCSSValue returns true if the value contains characters that could
// escape a CSS property value context (braces, semicolons, angle brackets).
func unsafeCSSValue(v string) bool {
	return strings.ContainsAny(v, "{}<>;")
}

func buildThemeVarsCSS(vars map[string]string) template.CSS {
	if len(vars) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(":root {\n")
	for k, v := range vars {
		if !validThemeVarKey.MatchString(k) {
			slog.Warn("skipping theme var with invalid key (only a-z/0-9 allowed)", "key", k)
			continue
		}
		if unsafeCSSValue(v) {
			slog.Warn("skipping theme var with unsafe value", "key", k, "value", v)
			continue
		}
		b.WriteString("  --theme-")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString(";\n")
	}
	b.WriteString("}\n")

	// If all vars were invalid, return empty instead of an empty :root block.
	if b.Len() == len(":root {\n}\n") {
		return ""
	}

	return template.CSS(b.String()) // #nosec G203 -- validated config CSS
}
