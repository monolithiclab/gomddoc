package template

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
)

// readAsset searches for a named asset in the theme directory first,
// then falls back to the shared assets directory (overlay semantics).
func (h *HTMLRenderer) readAsset(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid asset path %q", name)
	}
	themeDir := path.Join("assets", "themes", h.siteConfig.Theme.Name)
	for _, dir := range []string{themeDir, "assets/shared"} {
		data, err := fs.ReadFile(h.assetsFS, path.Join(dir, name))
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("asset %q not found in theme or shared", name)
}

// inlineJSAsset reads an asset and returns it as template.JS for safe
// embedding inside <script> tags without escaping.
func (h *HTMLRenderer) inlineJSAsset(name string) (template.JS, error) {
	data, err := h.readAsset(name)
	if err != nil {
		return "", err
	}
	return template.JS(data), nil // #nosec G203 -- trusted embedded asset
}

// inlineCSSAsset reads an asset and returns it as template.CSS for safe
// embedding inside <style> tags without escaping.
func (h *HTMLRenderer) inlineCSSAsset(name string) (template.CSS, error) {
	data, err := h.readAsset(name)
	if err != nil {
		return "", err
	}
	return template.CSS(data), nil // #nosec G203 -- trusted embedded asset
}

// inlineHTMLAsset reads an asset and returns it as template.HTML for safe
// embedding in HTML context (e.g. inline SVGs) without escaping.
func (h *HTMLRenderer) inlineHTMLAsset(name string) (template.HTML, error) {
	data, err := h.readAsset(name)
	if err != nil {
		return "", err
	}
	return template.HTML(data), nil // #nosec G203 -- trusted embedded asset
}
