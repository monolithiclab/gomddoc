package template

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"path"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/seo"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

// Renderer defines the interface for template rendering
type Renderer interface {
	// Render renders a template with the given data
	// The context can be used for cancellation, timeouts, and request-scoped values
	Render(ctx context.Context, templateName string, data any) ([]byte, error)

	// HasTemplate checks if a layout template exists for the current theme
	HasTemplate(name string) bool
}

// TemplateContext holds the data passed to templates
type TemplateContext struct {
	// Site configuration (public, safe to expose)
	Site *config.SiteConfig

	// Page-specific data (namespaced for extensibility)
	Page PageContext
}

type PageContext struct {
	Content    template.HTML
	Path       string            // Current request path
	Meta       map[string]any    // Extracted metadata (e.g., front matter)
	TOC        *enricher.TOCNode // Table of Contents
	Navigation *enricher.NavTree // Navigation tree (populated by enricher)
}

// bufferPool is a sync.Pool for reusing bytes.Buffer objects
// This reduces GC pressure in high-traffic scenarios by reusing buffers
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// HTMLRenderer implements Renderer for HTML templates.
//
// Thread-safe: Render() is safe for concurrent calls. The siteConfig and assetsFS
// fields are read-only after construction. Thread-safety of Render() depends on
// the injected TemplateCache implementation (CachedTemplateStore uses sync.Map,
// PassthroughTemplateStore is stateless).
//
// IMPORTANT: Do not mutate siteConfig after construction if using concurrently.
type HTMLRenderer struct {
	assetsFS      fs.FS
	siteConfig    *config.SiteConfig   // For theme name (NOT full Config - security)
	cache         TemplateCache        // Injected dependency (strategy pattern)
	breadcrumbGen breadcrumb.Generator // Optional breadcrumb generator
	themeVars     themeVarsCache       // Cached CSS custom properties from theme config
}

// RendererOption is a functional option for configuring HTMLRenderer
type RendererOption func(*HTMLRenderer)

// WithCache sets the template cache implementation for the renderer
// If not provided, defaults to PassthroughTemplateStore (no caching)
func WithCache(cache TemplateCache) RendererOption {
	return func(r *HTMLRenderer) {
		r.cache = cache
	}
}

// WithBreadcrumbGenerator sets the breadcrumb generator for the renderer
func WithBreadcrumbGenerator(gen breadcrumb.Generator) RendererOption {
	return func(r *HTMLRenderer) {
		r.breadcrumbGen = gen
	}
}

// NewHTMLRenderer creates a new HTML template renderer
// Required parameters: siteConfig and assetsFS (cannot operate without them)
// Optional parameters: cache (defaults to PassthroughTemplateStore)
// Only SiteConfig is stored (NOT full Config) to prevent leaking operational settings to templates
func NewHTMLRenderer(siteConfig *config.SiteConfig, assetsFS fs.FS, opts ...RendererOption) *HTMLRenderer {
	r := &HTMLRenderer{
		siteConfig: siteConfig,
		assetsFS:   assetsFS,
		cache:      &PassthroughTemplateStore{}, // Default: no caching
	}

	// Apply optional configurations
	r.Configure(opts...)

	return r
}

// Configure configures an HTMLRenderer instance with additional options
func (h *HTMLRenderer) Configure(opts ...RendererOption) {
	for _, opt := range opts {
		opt(h)
	}
}

// Render renders an HTML template with the given data
// Cache behavior is determined by the injected TemplateCache implementation
// No conditional logic needed - PassthroughTemplateStore always returns nil (cache miss)
// The context is checked before expensive operations for cancellation support
func (h *HTMLRenderer) Render(ctx context.Context, templateName string, data any) ([]byte, error) {
	theme := h.siteConfig.Theme.Name
	cacheKey := fmt.Sprintf("assets/themes/%s/layouts/%s", theme, templateName)

	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try cache first (returns nil if PassthroughTemplateStore)
	tmpl := h.cache.Get(cacheKey)

	var err error
	if tmpl == nil {
		// Check context again before expensive template parsing
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Cache miss or dev mode - parse template (layout + partials)
		tmpl, err = h.parseTemplate(templateName)
		if err != nil {
			return nil, fmt.Errorf("parse template: %w", err)
		}

		// Store in cache (no-op if PassthroughTemplateStore)
		h.cache.Set(cacheKey, tmpl)
	}

	// Execute template with pooled buffer to reduce GC pressure
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		if buf.Cap() <= 65536 {
			bufferPool.Put(buf)
		}
	}()

	err = tmpl.Execute(buf, data)
	if err != nil {
		return nil, err
	}

	// Copy bytes since buffer will be reused
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// parseTemplate parses a layout template with its partials, with automatic fallback to default theme.
// Partials are discovered via glob in the theme's partials/ directory and parsed together
// with the layout so that {{ template "partial-name" . }} calls work.
func (h *HTMLRenderer) parseTemplate(templateName string) (*template.Template, error) {
	tmpl, err := h.parseThemeTemplate(templateName, h.siteConfig.Theme.Name)
	if err != nil && h.siteConfig.Theme.Name != config.DefaultThemeName {
		// Fallback to default theme
		slog.Warn("Theme template not found, falling back to default",
			slog.String("theme", h.siteConfig.Theme.Name),
			slog.String("template", templateName))
		tmpl, err = h.parseThemeTemplate(templateName, config.DefaultThemeName)
	}
	return tmpl, err
}

// parseThemeTemplate parses a layout and its partials for a specific theme.
// The layout is loaded from assets/themes/{theme}/layouts/{templateName}.
// Any partials in assets/themes/{theme}/partials/*.html.tmpl are parsed alongside it.
//
// Partial resolution order (last parsed wins):
//  1. Default theme partials (if theme != default) — baseline definitions
//  2. Theme partials — theme-specific overrides
//  3. Site-level partials at partials/*.html.tmpl — user overrides from .gomddoc/partials/
func (h *HTMLRenderer) parseThemeTemplate(templateName, theme string) (*template.Template, error) {
	layoutPath := path.Join("assets", "themes", theme, "layouts", templateName)
	themePartialsGlob := path.Join("assets", "themes", theme, "partials", "*.html.tmpl")

	// Start with layout only; partials are layered in priority order below.
	tmpl, err := template.New(templateName).Funcs(h.funcMap()).ParseFS(h.assetsFS, layoutPath)
	if err != nil {
		return nil, err
	}

	// For non-default themes, load default theme partials as a baseline
	// so the theme only needs to override the partials it changes.
	if theme != config.DefaultThemeName {
		defaultPartialsGlob := path.Join("assets", "themes", config.DefaultThemeName, "partials", "*.html.tmpl")
		if _, err := h.parseGlob(tmpl, defaultPartialsGlob); err != nil {
			return nil, fmt.Errorf("parse default theme partials: %w", err)
		}
	}

	// Theme partials override default partials.
	if _, err := h.parseGlob(tmpl, themePartialsGlob); err != nil {
		return nil, fmt.Errorf("parse theme partials: %w", err)
	}

	// Site-level partials override everything. These live at partials/*.html.tmpl
	// in the overlay FS, mapping to .gomddoc/partials/ on disk.
	sitePartialsGlob := path.Join("partials", "*.html.tmpl")
	if _, err := h.parseGlob(tmpl, sitePartialsGlob); err != nil {
		return nil, fmt.Errorf("parse site partials: %w", err)
	}

	return tmpl, nil
}

// parseGlob parses files matching glob into tmpl if any matches exist.
// Returns true if files were parsed, false if no matches found.
func (h *HTMLRenderer) parseGlob(tmpl *template.Template, glob string) (bool, error) {
	if matches, _ := fs.Glob(h.assetsFS, glob); len(matches) > 0 {
		if _, err := tmpl.ParseFS(h.assetsFS, glob); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// funcMap returns the map of functions available in templates
func (h *HTMLRenderer) funcMap() template.FuncMap {
	themeDir := path.Join("assets", "themes", h.siteConfig.Theme.Name)
	return template.FuncMap{
		"breadcrumbs":  h.generateBreadcrumbs,
		"toc":          h.generateTOC,
		"editURL":      h.generateEditURL,
		"navigation":   h.generateNavigation,
		"themeVarsCSS": h.generateThemeVarsCSS,
		"canonicalURL": func(pagePath string) string {
			return seo.PageURL(h.siteConfig.Meta.Domain, pagePath, h.siteConfig.DefaultIndex)
		},
		"assetURL": func(name string) string {
			return "/_assets/" + name
		},
		"inlineAsset": func(name string) (template.JS, error) {
			// Search theme dir first, then shared (overlay semantics)
			for _, dir := range []string{themeDir, "assets/shared"} {
				data, err := fs.ReadFile(h.assetsFS, path.Join(dir, name))
				if err == nil {
					return template.JS(data), nil // #nosec G203 -- trusted embedded asset
				}
			}
			return "", fmt.Errorf("asset %q not found in theme or shared", name)
		},
	}
}

// generateEditURL generates the full edit URL for a page by combining
// the configured EditURL base with the page path. Returns an empty string
// if EditURL is not configured, which templates use to conditionally hide the link.
func (h *HTMLRenderer) generateEditURL(pagePath string) string {
	if h.siteConfig.EditURL == "" {
		return ""
	}
	base := strings.TrimRight(h.siteConfig.EditURL, "/")
	if !strings.HasPrefix(pagePath, "/") {
		pagePath = "/" + pagePath
	}
	return base + pagePath
}

// generateNavigation renders the navigation tree from enrichment data as HTML.
// Returns empty HTML if no navigation data is available.
func (h *HTMLRenderer) generateNavigation(navTree *enricher.NavTree) template.HTML {
	if navTree == nil || len(navTree.Items) == 0 {
		return ""
	}
	var buf bytes.Buffer
	renderNavItems(&buf, navTree.Items)
	return template.HTML(buf.String()) // #nosec G203
}

// renderNavItems renders enricher.NavItem slices as HTML with nested <ul>/<li> elements.
// Directories use <details>/<summary> for collapsible sections.
// Files use <a> links with class="active" when Active is true.
func renderNavItems(buf *bytes.Buffer, items []enricher.NavItem) {
	buf.WriteString("<ul>")
	for _, item := range items {
		buf.WriteString("<li>")
		if item.IsDir {
			buf.WriteString("<details")
			if item.Open {
				buf.WriteString(" open")
			}
			buf.WriteString("><summary>")
			buf.WriteString(template.HTMLEscapeString(item.Title))
			buf.WriteString("</summary>")
			if len(item.Children) > 0 {
				renderNavItems(buf, item.Children)
			}
			buf.WriteString("</details>")
		} else {
			buf.WriteString(`<a href="`)
			buf.WriteString(template.HTMLEscapeString(item.Path))
			buf.WriteString(`"`)
			if item.Active {
				buf.WriteString(` class="active"`)
			}
			buf.WriteString(">")
			buf.WriteString(template.HTMLEscapeString(item.Title))
			buf.WriteString("</a>")
		}
		buf.WriteString("</li>")
	}
	buf.WriteString("</ul>")
}

// generateBreadcrumbs generates breadcrumbs for the given path
func (h *HTMLRenderer) generateBreadcrumbs(path string) []breadcrumb.Breadcrumb {
	if h.breadcrumbGen == nil {
		return nil
	}
	return h.breadcrumbGen.Generate(path)
}

// generateTOC generates the Table of Contents HTML
func (h *HTMLRenderer) generateTOC(toc *enricher.TOCNode, levels ...int) template.HTML {
	if toc == nil || len(toc.Children) == 0 {
		return ""
	}

	minLevel := 1
	maxLevel := 2
	if len(levels) > 0 {
		minLevel = levels[0]
	}
	if len(levels) > 1 {
		maxLevel = levels[1]
	}

	var buf bytes.Buffer
	h.renderTOCNode(&buf, toc, minLevel, maxLevel)
	return template.HTML(buf.String()) // #nosec G203
}

func (h *HTMLRenderer) renderTOCNode(buf *bytes.Buffer, node *enricher.TOCNode, minLevel, maxLevel int) {
	// If this node is within range (or it's the root/container), render its children
	// Root is level 0.

	if node.Level > maxLevel {
		return
	}

	hasVisibleChildren := false
	for _, child := range node.Children {
		if child.Level >= minLevel && child.Level <= maxLevel {
			hasVisibleChildren = true
			break
		}
		// If child is below minLevel, it might contain visible descendants?
		// e.g. want h2, structure is h1 -> h2.
		// If we don't traverse h1, we miss h2.
		if child.Level < minLevel {
			// Check if this child has visible descendants
			if hasVisibleDescendants(child, minLevel, maxLevel) {
				hasVisibleChildren = true
				break
			}
		}
	}

	if !hasVisibleChildren {
		return
	}

	buf.WriteString("<ul>")
	for _, child := range node.Children {
		if child.Level >= minLevel && child.Level <= maxLevel {
			buf.WriteString("<li><a href=\"#")
			buf.WriteString(template.HTMLEscapeString(child.ID))
			buf.WriteString("\">")
			buf.WriteString(template.HTMLEscapeString(child.Text))
			buf.WriteString("</a>")
			h.renderTOCNode(buf, child, minLevel, maxLevel)
			buf.WriteString("</li>")
		} else if child.Level < minLevel {
			// Traverse transparently
			h.renderTOCNode(buf, child, minLevel, maxLevel)
		}
	}
	buf.WriteString("</ul>")
}

func hasVisibleDescendants(node *enricher.TOCNode, min, max int) bool {
	for _, child := range node.Children {
		if child.Level >= min && child.Level <= max {
			return true
		}
		if child.Level < min && hasVisibleDescendants(child, min, max) {
			return true
		}
	}
	return false
}

// ResolveLayout determines the template name to use based on frontmatter metadata.
// If the metadata contains a "layout" field and the corresponding template exists,
// it returns "{layout}.html.tmpl". Otherwise, it falls back to "default.html.tmpl".
func ResolveLayout(r Renderer, metadata map[string]any) string {
	const defaultTemplate = "default.html.tmpl"
	layout, ok := metadata["layout"].(string)
	if !ok || layout == "" {
		return defaultTemplate
	}
	candidate := layout + ".html.tmpl"
	if r.HasTemplate(candidate) {
		return candidate
	}
	return defaultTemplate
}

// HasTemplate checks if a layout template exists for the current theme.
// It checks only the configured theme directory (not the default fallback),
// since parseTemplate already handles theme-to-default fallback during rendering.
func (h *HTMLRenderer) HasTemplate(name string) bool {
	layoutPath := path.Join("assets", "themes", h.siteConfig.Theme.Name, "layouts", name)
	_, err := fs.Stat(h.assetsFS, layoutPath)
	return err == nil
}

// ClearCache clears the template cache (used in dev mode hot reload)
// Delegates to cache implementation (no-op for PassthroughTemplateStore)
func (h *HTMLRenderer) ClearCache() {
	h.cache.Clear()
	slog.Info("[DEV] Template cache cleared")
}

// ValidateDefaultTheme checks that the default theme exists in the asset filesystem
// This is a fatal error if missing, as the application cannot function without it
func (h *HTMLRenderer) ValidateDefaultTheme() error {
	defaultTemplate := path.Join("assets", "themes", config.DefaultThemeName, "layouts", "default.html.tmpl")
	f, err := h.assetsFS.Open(defaultTemplate)
	if err != nil {
		return fmt.Errorf("default theme not found: %w (this is a fatal error)", err)
	}
	_ = f.Close()
	slog.Debug("Default theme validated", slog.String("template", defaultTemplate))
	return nil
}
