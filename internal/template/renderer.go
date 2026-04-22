package template

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"path"
	"slices"
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
	Path       string             // Current request path
	Meta       map[string]any     // Extracted metadata (e.g., front matter)
	TOC        *enricher.TOCNode  // Table of Contents
	Navigation *enricher.NavTree  // Navigation tree (populated by enricher)
	PrevPage   *enricher.PageLink // Previous page in navigation order
	NextPage   *enricher.PageLink // Next page in navigation order
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
	return slices.Clone(buf.Bytes()), nil
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
	return template.FuncMap{
		"breadcrumbs":  h.generateBreadcrumbs,
		"toc":          filterTOC,
		"editURL":      h.generateEditURL,
		"themeVarsCSS": h.generateThemeVarsCSS,
		"canonicalURL": func(pagePath string) string {
			return seo.PageURL(h.siteConfig.Meta.Domain, pagePath, h.siteConfig.DefaultIndex)
		},
		"jsonLD": func(page PageContext) template.HTML {
			return h.generateJSONLD(page)
		},
		"assetURL": func(name string) string {
			return "/_assets/" + name
		},
		"inlineJSAsset":   h.inlineJSAsset,
		"inlineCSSAsset":  h.inlineCSSAsset,
		"inlineHTMLAsset": h.inlineHTMLAsset,
	}
}

// generateJSONLD produces JSON-LD structured data script tags for a page.
func (h *HTMLRenderer) generateJSONLD(page PageContext) template.HTML {
	cfg := seo.JSONLDConfig{
		Domain:       h.siteConfig.Meta.Domain,
		SiteName:     h.siteConfig.Meta.Title,
		DefaultIndex: h.siteConfig.DefaultIndex,
		HasSearch:    h.siteConfig.HasSearch,
	}

	// Build breadcrumbs with full URLs
	var breadcrumbs []seo.BreadcrumbItem
	if h.breadcrumbGen != nil {
		for _, bc := range h.breadcrumbGen.Generate(page.Path) {
			breadcrumbs = append(breadcrumbs, seo.BreadcrumbItem{
				Name: bc.Label,
				URL:  seo.PageURL(cfg.Domain, bc.Path, cfg.DefaultIndex),
			})
		}
	}

	// Detect index page
	isIndex := page.Path == "/" || path.Base(page.Path) == h.siteConfig.DefaultIndex

	p := seo.JSONLDPage{
		Path:        page.Path,
		Breadcrumbs: breadcrumbs,
		IsIndex:     isIndex,
	}

	// Extract metadata fields
	if title, ok := page.Meta["title"].(string); ok {
		p.Title = title
	}
	if desc, ok := page.Meta["description"].(string); ok {
		p.Description = desc
	}
	if author, ok := page.Meta["author"].(string); ok {
		p.Author = author
	}

	raw := seo.GenerateJSONLD(cfg, p)
	if raw == "" {
		return ""
	}
	return template.HTML(`<script type="application/ld+json">` + raw + `</script>`) // #nosec G203 -- trusted JSON-LD output
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

// generateBreadcrumbs generates breadcrumbs for the given path
func (h *HTMLRenderer) generateBreadcrumbs(path string) []breadcrumb.Breadcrumb {
	if h.breadcrumbGen == nil {
		return nil
	}
	return h.breadcrumbGen.Generate(path)
}

// filterTOC returns a filtered list of TOCNode children for template rendering.
// Nodes outside the min/max level range are pruned: nodes below min are traversed
// transparently (their children promoted), nodes above max are dropped entirely.
// Default levels: 1–2.
func filterTOC(toc *enricher.TOCNode, levels ...int) []*enricher.TOCNode {
	if toc == nil || len(toc.Children) == 0 {
		return nil
	}

	minLevel := 1
	maxLevel := 2
	if len(levels) > 0 {
		minLevel = levels[0]
	}
	if len(levels) > 1 {
		maxLevel = levels[1]
	}

	return filterTOCNodes(toc.Children, minLevel, maxLevel)
}

// filterTOCNodes recursively filters a slice of TOCNode by heading level range.
// Nodes within [min, max] are kept with their children filtered recursively.
// Nodes below min are skipped but their children are promoted (transparent traversal).
// Nodes above max are dropped entirely.
func filterTOCNodes(nodes []*enricher.TOCNode, min, max int) []*enricher.TOCNode {
	var result []*enricher.TOCNode
	for _, node := range nodes {
		if node.Level > max {
			continue
		}
		if node.Level >= min {
			result = append(result, &enricher.TOCNode{
				Level:    node.Level,
				Text:     node.Text,
				ID:       node.ID,
				Children: filterTOCNodes(node.Children, min, max),
			})
		} else {
			// Below min level — promote children (transparent traversal)
			result = append(result, filterTOCNodes(node.Children, min, max)...)
		}
	}
	return result
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
