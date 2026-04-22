package template

import (
	"html/template"
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

func buildThemeVarsCSS(vars map[string]string) template.CSS {
	if len(vars) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(":root {\n")
	for k, v := range vars {
		b.WriteString("  --theme-")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString(";\n")
	}
	b.WriteString("}\n")

	return template.CSS(b.String()) // #nosec G203 -- trusted config CSS
}
