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
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/text"
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
	Content     template.HTML
	Breadcrumbs map[string]string
}

const (
	HomePath  = "/"
	HomeLabel = "Home"
	RootDir   = "."
)

// bufferPool is a sync.Pool for reusing bytes.Buffer objects
// This reduces GC pressure in high-traffic scenarios by reusing buffers
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GenerateBreadcrumbs creates breadcrumb navigation from a file path using provider to determine file vs directory
// Returns a map where keys are URL paths and values are capitalized display names
// Directories get trailing slashes in URLs, files don't
// Example: "/howtos/core/test.md" → {"/": "Home", "/howtos/": "Howtos", "/howtos/core/": "Core", "/howtos/core/test.md": "Test"}
func GenerateBreadcrumbs(p provider.Provider, filepath string) map[string]string {
	const (
		initialCapacity = 8
		maxPathLength   = 2048
	)
	breadcrumbs := make(map[string]string, initialCapacity) // Pre-allocate with reasonable capacity

	// Always include root
	breadcrumbs[HomePath] = HomeLabel

	if len(filepath) > maxPathLength {
		slog.Warn("url path too long, skipping breadcrumbs")
		return breadcrumbs
	}

	// Clean the path but keep the full path including filename
	filepath = strings.Trim(path.Clean(filepath), "/")

	// If we're at root, return just the root breadcrumb
	if filepath == RootDir || filepath == "" {
		return breadcrumbs
	}

	// Use provider to determine if the target path is a file or directory
	targetIsDir := false
	if stat, err := p.Stat("/" + filepath); err == nil {
		targetIsDir = stat.IsDir()
	}

	// Split the full path into segments
	segments := strings.Split(filepath, "/")

	// Build cumulative paths efficiently with strings.Builder
	var pathBuilder strings.Builder
	pathBuilder.Grow(len(filepath)) // Pre-allocate capacity

	for i, segment := range segments {
		pathBuilder.WriteByte('/')
		pathBuilder.WriteString(segment)

		currentPath := pathBuilder.String()
		isLastSegment := i == len(segments)-1

		// Determine if this segment should have a trailing slash
		if isLastSegment {
			// For the last segment, use the actual stat result
			if targetIsDir {
				currentPath += "/"
			}
			// If it's a file, no trailing slash
		} else {
			// All intermediate segments are directories - add trailing slash
			currentPath += "/"
		}

		breadcrumbs[currentPath] = titleCase(basename(segment))
	}

	return breadcrumbs
}

// basename strips the extension from a filename
func basename(s string) string {
	return strings.TrimSuffix(s, path.Ext(s))
}

// titleCase is a convenience wrapper around text.TitleCase
func titleCase(s string) string {
	return text.TitleCase(s)
}

// HTMLRenderer implements Renderer for HTML templates
type HTMLRenderer struct {
	assetsFS   fs.FS
	siteConfig *config.SiteConfig // For theme name (NOT full Config - security)
	cache      TemplateCache      // Injected dependency (strategy pattern)
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
		bufferPool.Put(buf)
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
	tmpl, err := template.New(templateName).ParseFS(h.assetsFS, templatePath)
	if err != nil && h.siteConfig.Theme.Name != config.DefaultThemeName {
		// Fallback to default theme
		slog.Warn("Theme template not found, falling back to default",
			slog.String("theme", h.siteConfig.Theme.Name),
			slog.String("template", templateName))
		templatePath = path.Join("assets/themes/", config.DefaultThemeName, templateName)
		tmpl, err = template.New(templateName).ParseFS(h.assetsFS, templatePath)
	}
	return tmpl, err
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
	if _, err := h.assetsFS.Open(defaultTemplate); err != nil {
		return fmt.Errorf("default theme not found: %w (this is a fatal error)", err)
	}
	slog.Debug("Default theme validated", slog.String("template", defaultTemplate))
	return nil
}
