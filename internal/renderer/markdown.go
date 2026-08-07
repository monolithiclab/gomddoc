package renderer

import (
	"bytes"
	"context"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
)

// MarkdownOptions configures the MarkdownRenderer.
type MarkdownOptions struct {
	// HighlightTheme controls the Chroma syntax highlighting style for fenced
	// code blocks. Defaults to "github" when empty.
	HighlightTheme string

	// Features controls which markdown rendering features are enabled.
	// If nil, all features default to enabled.
	// Per-page frontmatter can override site-level settings.
	Features map[string]bool
}

// MarkdownRenderer transforms markdown content to HTML.
// It uses the goldmark library with GitHub Flavored Markdown (GFM) and Front Matter support.
//
// The renderer is stateless and thread-safe. The underlying goldmark instance
// is shared across renders.
type MarkdownRenderer struct {
	md       goldmark.Markdown
	features map[string]bool
}

// NewMarkdownRenderer creates a new markdown renderer with the given options.
//
// Configuration:
//   - extension.GFM: Tables, strikethrough, linkify, task lists
//   - meta.Meta: Strips YAML frontmatter from rendered output (metadata extraction is in enricher)
//   - highlighting: Syntax highlighting via Chroma with the specified theme
//   - HeadingAnchorExtension: Appends anchor links to headings with IDs
//   - AdmonitionExtension: Transforms [!TYPE] blockquotes to admonition divs
//   - ColorChipExtension: Transforms hex color code spans to <gmd-color-chip> elements
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
			&HeadingAnchorExtension{},
			&AdmonitionExtension{},
			&ColorChipExtension{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML to match common expectations for doc sites
		),
	)

	return &MarkdownRenderer{
		md:       md,
		features: opts.Features,
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

	// Merge site-level and per-page feature flags.
	var pageFeatures map[string]bool
	if enrichment != nil {
		pageFeatures = enrichment.Features
	}
	merged := config.MergeFeatures(m.features, pageFeatures)

	// Set features on parser context for AST transformers.
	pCtx := parser.NewContext()
	pCtx.Set(featuresContextKey, merged)

	reader := text.NewReader(content)
	doc := m.md.Parser().Parse(reader, parser.WithContext(pCtx))

	// Set features on document node for node renderers.
	setDocFeatures(doc, merged)

	var buf bytes.Buffer
	buf.Grow(len(content) * 2)
	if err := m.md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, err
	}

	return &RenderResult{
		Content:  buf.Bytes(),
		MimeType: "text/html; charset=utf-8",
	}, nil
}
