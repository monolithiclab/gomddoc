package renderer

import (
	"bytes"
	"context"
	"log/slog"
	"mime"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"
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

// MarkdownOptions configures the MarkdownRenderer.
type MarkdownOptions struct {
	// HighlightTheme controls the Chroma syntax highlighting style for fenced
	// code blocks. Defaults to "github" when empty.
	HighlightTheme string

	// ColorChips enables inline color chip rendering for backtick-wrapped hex
	// codes (e.g. `#E91E63`). Can be overridden per page via frontmatter.
	ColorChips bool
}

// MarkdownRenderer transforms markdown content to HTML.
// It uses the goldmark library with GitHub Flavored Markdown (GFM) and Front Matter support.
//
// The renderer is stateless and thread-safe. The underlying goldmark instance
// is shared across renders.
type MarkdownRenderer struct {
	md         goldmark.Markdown
	colorChips bool
}

// NewMarkdownRenderer creates a new markdown renderer with the given options.
//
// Configuration:
//   - extension.GFM: Tables, strikethrough, linkify, task lists
//   - meta.Meta: YAML front matter support
//   - highlighting: Syntax highlighting via Chroma with the specified theme
//   - parser.WithAutoHeadingID: Automatic ID generation for headings
//   - html.WithUnsafe: Allow raw HTML (matches previous gomarkdown behavior)
func NewMarkdownRenderer(opts MarkdownOptions) *MarkdownRenderer {
	highlightTheme := opts.HighlightTheme
	if highlightTheme == "" {
		highlightTheme = "github"
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithStyle(highlightTheme),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML to match common expectations for doc sites
		),
	)

	return &MarkdownRenderer{
		md:         md,
		colorChips: opts.ColorChips,
	}
}

// InputMimeTypes returns the MIME types this renderer accepts as input.
func (m *MarkdownRenderer) InputMimeTypes() []string {
	return []string{"text/markdown"}
}

// OutputMimeTypes returns the MIME types this renderer produces.
func (m *MarkdownRenderer) OutputMimeTypes() []string {
	return []string{"text/html"}
}

// Render converts markdown content to HTML.
func (m *MarkdownRenderer) Render(ctx context.Context, content []byte) (*RenderResult, error) {
	// Check context before expensive operations
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Create parser context to extract metadata
	pCtx := parser.NewContext()

	// Parse the content
	reader := text.NewReader(content)
	doc := m.md.Parser().Parse(reader, parser.WithContext(pCtx))

	// Extract TOC using goldmark-toc
	tocItems, err := toc.Inspect(doc, content)
	if err != nil {
		slog.Warn("Failed to extract TOC", slog.Any("error", err))
	}
	// Extract metadata
	metadata := meta.Get(pCtx)

	// Render the document
	var buf bytes.Buffer
	if err := m.md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, err
	}

	// Post-process: add anchor links to headings, transform admonitions, then color chips
	rendered := addHeadingAnchors(buf.Bytes())
	rendered = TransformAdmonitions(rendered)

	if colorChipsEnabled(m.colorChips, metadata) {
		rendered = transformColorChips(rendered)
	}

	// Convert TOC
	var tocNode *TOCNode
	if tocItems != nil {
		tocNode = convertTOC(tocItems.Items)
	}

	return &RenderResult{
		Content:  rendered,
		MimeType: "text/html; charset=utf-8",
		Metadata: metadata,
		TOC:      tocNode,
	}, nil
}

// colorChipsEnabled returns whether color chips should be applied.
// Per-page frontmatter (color_chips: true/false) overrides the global default.
func colorChipsEnabled(globalDefault bool, metadata map[string]any) bool {
	if metadata != nil {
		if v, ok := metadata["color_chips"]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return globalDefault
}

// convertTOC converts goldmark-toc Items to our internal TOCNode structure.
func convertTOC(items toc.Items) *TOCNode {
	if len(items) == 0 {
		return nil
	}

	root := &TOCNode{
		Level:    0,
		Children: make([]*TOCNode, len(items)),
	}

	for i, item := range items {
		root.Children[i] = convertItem(item, 1)
	}

	return root
}

func convertItem(item *toc.Item, level int) *TOCNode {
	node := &TOCNode{
		Level: level,
		Text:  string(item.Title),
		ID:    string(item.ID),
	}

	// Lazy-allocate children only when needed
	if len(item.Items) > 0 {
		node.Children = make([]*TOCNode, 0, len(item.Items))
		for _, child := range item.Items {
			node.Children = append(node.Children, convertItem(child, level+1))
		}
	}

	return node
}
