package template

import (
	"bytes"
	"html/template"
	"io/fs"
	"sync"
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

	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.Bytes(), err
}
