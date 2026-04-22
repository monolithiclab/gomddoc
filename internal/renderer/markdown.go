package renderer

import (
	"bytes"
	"context"
	"mime"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
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
// It uses the goldmark library with GitHub Flavored Markdown (GFM) and Front Matter support.
//
// The renderer is stateless and thread-safe. The underlying goldmark instance
// is shared across renders.
type MarkdownRenderer struct {
	md goldmark.Markdown
}

// NewMarkdownRenderer creates a new markdown renderer with standard settings.
//
// Configuration:
//   - extension.GFM: Tables, strikethrough, linkify, task lists
//   - meta.Meta: YAML front matter support
//   - parser.WithAutoHeadingID: Automatic ID generation for headings
//   - html.WithUnsafe: Allow raw HTML (matches previous gomarkdown behavior)
func NewMarkdownRenderer() *MarkdownRenderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML to match common expectations for doc sites
		),
	)

	return &MarkdownRenderer{
		md: md,
	}
}

// SupportedMimeTypes returns the MIME types this renderer handles.
func (m *MarkdownRenderer) SupportedMimeTypes() []string {
	return []string{"text/markdown"}
}

// Render converts markdown content to HTML.
func (m *MarkdownRenderer) Render(ctx context.Context, content []byte) (*RenderResult, error) {
	// Check context before expensive operations
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	pCtx := parser.NewContext()

	// Convert markdown to HTML
	if err := m.md.Convert(content, &buf, parser.WithContext(pCtx)); err != nil {
		return nil, err
	}

	// Extract metadata
	metadata := meta.Get(pCtx)

	// Check context after rendering
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &RenderResult{
		Content:  buf.Bytes(),
		MimeType: "text/html; charset=utf-8",
		Metadata: metadata,
	}, nil
}
