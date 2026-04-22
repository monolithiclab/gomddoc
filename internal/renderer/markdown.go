package renderer

import (
	"context"
	"mime"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

func init() {
	// Register markdown MIME types with the standard library.
	// This ensures mime.TypeByExtension() returns "text/markdown"
	// for .md and .markdown files.
	//
	// Co-located with MarkdownRenderer as it owns this mapping.
	// Errors are ignored as these are standard MIME types that should always succeed.
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
	_ = mime.AddExtensionType(".markdown", "text/markdown; charset=utf-8")
}

// MarkdownRenderer transforms markdown content to HTML.
// It uses the gomarkdown library with CommonExtensions and AutoHeadingIDs.
//
// The renderer is stateless and thread-safe. Each Render() call creates
// fresh parser and HTML renderer instances to avoid shared state.
type MarkdownRenderer struct {
	extensions parser.Extensions
	htmlFlags  html.Flags
	htmlOpts   html.RendererOptions
}

// NewMarkdownRenderer creates a new markdown renderer with standard settings.
//
// Parser configuration:
//   - CommonExtensions: Tables, fenced code blocks, autolinks, strikethrough
//   - AutoHeadingIDs: Automatic ID generation for headings
//   - NoEmptyLineBeforeBlock: Allows lists/blocks without blank lines
//
// HTML renderer configuration:
//   - CommonFlags: Standard HTML output flags
func NewMarkdownRenderer() *MarkdownRenderer {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}

	return &MarkdownRenderer{
		extensions: extensions,
		htmlFlags:  htmlFlags,
		htmlOpts:   opts,
	}
}

// SupportedMimeTypes returns the MIME types this renderer handles.
func (m *MarkdownRenderer) SupportedMimeTypes() []string {
	return []string{"text/markdown"}
}

// Render converts markdown content to HTML.
//
// Context handling:
//   - Checks ctx.Err() before parsing (expensive operation)
//   - Checks ctx.Err() after parsing, before rendering
//   - Returns context.Canceled or context.DeadlineExceeded if cancelled
//
// Thread safety: Creates fresh parser and HTML renderer for each call.
// Both components maintain internal state and must not be shared across
// concurrent renders. This design ensures the MarkdownRenderer itself
// is stateless and thread-safe.
func (m *MarkdownRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
	// Check context before expensive parsing operation
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	// Create new parser for each render (library constraint - DO NOT pool)
	p := parser.NewWithExtensions(m.extensions)
	doc := p.Parse(content)

	// Check context after parsing, before rendering
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	// Create new HTML renderer for each render (thread safety - has internal state)
	htmlRenderer := html.NewRenderer(m.htmlOpts)
	htmlOutput := markdown.Render(doc, htmlRenderer)
	return htmlOutput, "text/html; charset=utf-8", nil
}
