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
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// Renderer defines the interface for template rendering
type Renderer interface {
	// Render renders a template with the given data
	// The context can be used for cancellation, timeouts, and request-scoped values
	Render(ctx context.Context, templateName string, data any) ([]byte, error)
}

// TemplateContext holds the data passed to templates
type TemplateContext struct {
	// Site configuration (public, safe to expose)
	Site *config.SiteConfig

	// Page-specific data (namespaced for extensibility)
	Page PageContext
}

type PageContext struct {
	Content template.HTML
	Path    string            // Current request path
	Meta    map[string]any    // Extracted metadata (e.g., front matter)
	TOC     *renderer.TOCNode // Table of Contents
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
	siteConfig    *config.SiteConfig    // For theme name (NOT full Config - security)
	cache         TemplateCache         // Injected dependency (strategy pattern)
	breadcrumbGen breadcrumb.Generator  // Optional breadcrumb generator
	navGen        *navigation.Generator // Optional navigation generator
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

// WithNavigationGenerator sets the navigation tree generator for the renderer
func WithNavigationGenerator(gen *navigation.Generator) RendererOption {
	return func(r *HTMLRenderer) {
		r.navGen = gen
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
	templatePath := fmt.Sprintf("assets/themes/%s/%s", theme, templateName)

	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try cache first (returns nil if PassthroughTemplateStore)
	tmpl := h.cache.Get(templatePath)

	var err error
	if tmpl == nil {
		// Check context again before expensive template parsing
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Cache miss or dev mode - parse template
		tmpl, err = h.parseTemplate(templateName, templatePath)
		if err != nil {
			return nil, fmt.Errorf("parse template: %w", err)
		}

		// Store in cache (no-op if PassthroughTemplateStore)
		h.cache.Set(templatePath, tmpl)
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

// parseTemplate parses a template with automatic fallback to default theme
func (h *HTMLRenderer) parseTemplate(templateName, templatePath string) (*template.Template, error) {
	tmpl, err := template.New(templateName).Funcs(h.funcMap()).ParseFS(h.assetsFS, templatePath)
	if err != nil && h.siteConfig.Theme.Name != config.DefaultThemeName {
		// Fallback to default theme
		slog.Warn("Theme template not found, falling back to default",
			slog.String("theme", h.siteConfig.Theme.Name),
			slog.String("template", templateName))
		templatePath = path.Join("assets/themes/", config.DefaultThemeName, templateName)
		tmpl, err = template.New(templateName).Funcs(h.funcMap()).ParseFS(h.assetsFS, templatePath)
	}
	return tmpl, err
}

// funcMap returns the map of functions available in templates
func (h *HTMLRenderer) funcMap() template.FuncMap {
	themeDir := path.Join("assets/themes", h.siteConfig.Theme.Name)
	return template.FuncMap{
		"breadcrumbs": h.generateBreadcrumbs,
		"toc":         h.generateTOC,
		"editURL":     h.generateEditURL,
		"navigation":  h.generateNavigation,
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

// generateNavigation generates the navigation tree HTML for the given path.
// Returns empty HTML if no navigation generator is configured.
func (h *HTMLRenderer) generateNavigation(currentPath string) template.HTML {
	if h.navGen == nil {
		return ""
	}
	root := h.navGen.Generate(currentPath)
	return template.HTML(navigation.RenderNavTree(root)) // #nosec G203
}

// generateBreadcrumbs generates breadcrumbs for the given path
func (h *HTMLRenderer) generateBreadcrumbs(path string) []breadcrumb.Breadcrumb {
	if h.breadcrumbGen == nil {
		return nil
	}
	return h.breadcrumbGen.Generate(path)
}

// generateTOC generates the Table of Contents HTML
func (h *HTMLRenderer) generateTOC(toc *renderer.TOCNode, levels ...int) template.HTML {
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

func (h *HTMLRenderer) renderTOCNode(buf *bytes.Buffer, node *renderer.TOCNode, minLevel, maxLevel int) {
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

func hasVisibleDescendants(node *renderer.TOCNode, min, max int) bool {
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

// ClearCache clears the template cache (used in dev mode hot reload)
// Delegates to cache implementation (no-op for PassthroughTemplateStore)
func (h *HTMLRenderer) ClearCache() {
	h.cache.Clear()
	slog.Info("[DEV] Template cache cleared")
}

// ValidateDefaultTheme checks that the default theme exists in the asset filesystem
// This is a fatal error if missing, as the application cannot function without it
func (h *HTMLRenderer) ValidateDefaultTheme() error {
	defaultTemplate := path.Join("assets/themes/", config.DefaultThemeName, "layout.html.tmpl")
	f, err := h.assetsFS.Open(defaultTemplate)
	if err != nil {
		return fmt.Errorf("default theme not found: %w (this is a fatal error)", err)
	}
	_ = f.Close()
	slog.Debug("Default theme validated", slog.String("template", defaultTemplate))
	return nil
}
