package renderer

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"
)

// MarkdownPassthroughRenderer returns raw markdown content with metadata and
// TOC extracted. It enables LLM-friendly API access via Accept: text/markdown.
type MarkdownPassthroughRenderer struct {
	md goldmark.Markdown
}

// NewMarkdownPassthroughRenderer creates a new markdown passthrough renderer.
func NewMarkdownPassthroughRenderer() *MarkdownPassthroughRenderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
	return &MarkdownPassthroughRenderer{md: md}
}

// InputMimeTypes returns the MIME types this renderer accepts.
func (m *MarkdownPassthroughRenderer) InputMimeTypes() []string {
	return []string{"text/markdown"}
}

// OutputMimeTypes returns the MIME types this renderer produces.
func (m *MarkdownPassthroughRenderer) OutputMimeTypes() []string {
	return []string{"text/markdown"}
}

// Render extracts metadata and TOC from markdown but returns the raw content.
// The content is returned with frontmatter stripped.
func (m *MarkdownPassthroughRenderer) Render(ctx context.Context, content []byte) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pCtx := parser.NewContext()
	reader := text.NewReader(content)
	doc := m.md.Parser().Parse(reader, parser.WithContext(pCtx))

	// Extract TOC
	tocItems, err := toc.Inspect(doc, content)
	if err != nil {
		slog.Warn("Failed to extract TOC", slog.Any("error", err))
	}

	metadata := meta.Get(pCtx)

	// Strip frontmatter: find the end of the second "---" delimiter
	body := stripFrontmatter(content)

	var tocNode *TOCNode
	if tocItems != nil {
		tocNode = convertTOC(tocItems.Items)
	}

	return &RenderResult{
		Content:  body,
		MimeType: "text/markdown; charset=utf-8",
		Metadata: metadata,
		TOC:      tocNode,
	}, nil
}

// stripFrontmatter removes YAML frontmatter delimited by --- from markdown content.
func stripFrontmatter(content []byte) []byte {
	if !bytes.HasPrefix(content, []byte("---")) {
		return content
	}

	// Find end of frontmatter (second "---")
	rest := content[3:]
	_, after, ok := bytes.Cut(rest, []byte("\n---"))
	if !ok {
		return content
	}

	// Skip past the closing delimiter and any following newline
	body := after
	body = bytes.TrimLeft(body, "\r\n")
	return body
}
