package renderer

import (
	"context"

	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// MarkdownPassthroughRenderer returns raw markdown content with frontmatter
// stripped. Metadata and TOC are provided by the enricher, not extracted here.
// It enables LLM-friendly API access via Accept: text/markdown.
type MarkdownPassthroughRenderer struct{}

// NewMarkdownPassthroughRenderer creates a new markdown passthrough renderer.
func NewMarkdownPassthroughRenderer() *MarkdownPassthroughRenderer {
	return &MarkdownPassthroughRenderer{}
}

// InputMimeTypes returns the MIME types this renderer accepts.
func (m *MarkdownPassthroughRenderer) InputMimeTypes() []string {
	return []string{"text/markdown"}
}

// OutputMimeTypes returns the MIME types this renderer produces.
func (m *MarkdownPassthroughRenderer) OutputMimeTypes() []string {
	return []string{"text/markdown"}
}

// Render returns raw markdown with frontmatter stripped.
func (m *MarkdownPassthroughRenderer) Render(ctx context.Context, content []byte, _ *enricher.EnrichmentData) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	body := text.StripFrontmatter(content)

	return &RenderResult{
		Content:  body,
		MimeType: "text/markdown; charset=utf-8",
	}, nil
}
