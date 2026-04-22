package processor

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// MarkdownProcessor implements Processor for Markdown to HTML conversion
type MarkdownProcessor struct {
	extensions parser.Extensions
	renderer   *html.Renderer
}

// NewMarkdownProcessor creates a new markdown processor with the same configuration as the original
func NewMarkdownProcessor() *MarkdownProcessor {
	// Store extensions configuration (same as original)
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock

	// Create HTML renderer with extensions (same as original)
	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return &MarkdownProcessor{
		extensions: extensions,
		renderer:   renderer,
	}
}

// Process converts markdown content to HTML
func (m *MarkdownProcessor) Process(content []byte) ([]byte, error) {
	// Create a new parser for each process call (parser is not reusable)
	p := parser.NewWithExtensions(m.extensions)
	doc := p.Parse(content)
	return markdown.Render(doc, m.renderer), nil
}

// ContentType returns the MIME type for HTML content
func (m *MarkdownProcessor) ContentType() string {
	return "text/html; charset=utf-8"
}
