package renderer

import (
	"bytes"
	"context"
	"mime"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"

	"github.com/monolithiclab/gomddoc/internal/enricher"
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
//   - meta.Meta: Strips YAML frontmatter from rendered output (metadata extraction is in enricher)
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
			meta.Meta, // Strips YAML frontmatter from rendered output
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
// Metadata and TOC come from the enrichment data, not from parsing here.
func (m *MarkdownRenderer) Render(ctx context.Context, content []byte, enrichment *enricher.EnrichmentData) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pCtx := parser.NewContext()
	reader := text.NewReader(content)
	doc := m.md.Parser().Parse(reader, parser.WithContext(pCtx))

	var buf bytes.Buffer
	if err := m.md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, err
	}

	// Post-process: add anchor links to headings, transform admonitions, then color chips
	rendered := addHeadingAnchors(buf.Bytes())
	rendered = transformAdmonitions(rendered)

	// Color chips: check enrichment metadata for per-page override
	var metadata map[string]any
	if enrichment != nil {
		metadata = enrichment.Metadata
	}
	if colorChipsEnabled(m.colorChips, metadata) {
		rendered = transformColorChips(rendered)
	}

	return &RenderResult{
		Content:  rendered,
		MimeType: "text/html; charset=utf-8",
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
