package template

import (
	"bytes"
	"html/template"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// Renderer defines the interface for template rendering
type Renderer interface {
	// Render renders a template with the given data
	Render(templateName string, data any) ([]byte, error)
}

// ThemeOptions holds theme-specific configuration
type ThemeOptions struct {
	BaseURL string
}

// TemplateContext holds the data passed to templates
type TemplateContext struct {
	Title       string
	Description string
	Content     template.HTML
	Theme       ThemeOptions
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
	assetsFS  fs.FS
	templates sync.Map // Cache for parsed templates
}

// NewHTMLRenderer creates a new HTML template renderer
func NewHTMLRenderer(assetsFS fs.FS) *HTMLRenderer {
	return &HTMLRenderer{
		assetsFS: assetsFS,
	}
}

// Render renders an HTML template with the given data
func (h *HTMLRenderer) Render(templateName string, data any) ([]byte, error) {
	templatePath := "assets/themes/default/" + templateName

	var tmpl *template.Template
	var err error

	// Check cache first
	cached, ok := h.templates.Load(templatePath)
	if ok {
		tmpl = cached.(*template.Template)
	} else {
		// Parse and cache template
		tmpl, err = template.New(templateName).ParseFS(h.assetsFS, templatePath)
		if err != nil {
			return nil, err
		}
		h.templates.Store(templatePath, tmpl)
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
