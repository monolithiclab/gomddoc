package renderer

import (
	"context"

	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/text"
	"gopkg.in/yaml.v3"
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

// Render returns raw markdown with frontmatter replaced by enriched metadata.
// The original frontmatter is merged with enrichment data (related docs,
// prev/next navigation) to produce a self-contained markdown document.
func (m *MarkdownPassthroughRenderer) Render(ctx context.Context, content []byte, enrichment *enricher.EnrichmentData) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	body := text.StripFrontmatter(content)

	fm := buildEnrichedFrontmatter(enrichment)
	if fm == nil {
		return &RenderResult{
			Content:  body,
			MimeType: "text/markdown; charset=utf-8",
		}, nil
	}

	out, err := yaml.Marshal(fm)
	if err != nil {
		// Fall back to body without frontmatter on marshal error.
		return &RenderResult{
			Content:  body,
			MimeType: "text/markdown; charset=utf-8",
		}, nil
	}

	buf := make([]byte, 0, len(out)+len(body)+8) // 8 = two "---\n" delimiters
	buf = append(buf, "---\n"...)
	buf = append(buf, out...)
	buf = append(buf, "---\n"...)
	buf = append(buf, body...)

	return &RenderResult{
		Content:  buf,
		MimeType: "text/markdown; charset=utf-8",
	}, nil
}

// enrichedFrontmatter is the YAML structure emitted in passthrough markdown.
// Metadata is under its own key (not inline) to avoid collisions with
// enrichment fields like related_docs or prev_page.
type enrichedFrontmatter struct {
	Metadata    map[string]any        `yaml:"metadata,omitempty"`
	RelatedDocs []enricher.RelatedDoc `yaml:"related_docs,omitempty"`
	PrevPage    *enricher.PageLink    `yaml:"prev_page,omitempty"`
	NextPage    *enricher.PageLink    `yaml:"next_page,omitempty"`
}

// buildEnrichedFrontmatter constructs frontmatter from enrichment data.
// Returns nil if there is nothing to include.
func buildEnrichedFrontmatter(e *enricher.EnrichmentData) *enrichedFrontmatter {
	if e == nil {
		return nil
	}

	if len(e.Metadata) == 0 && len(e.RelatedDocs) == 0 && e.PrevPage == nil && e.NextPage == nil {
		return nil
	}

	return &enrichedFrontmatter{
		Metadata:    e.Metadata,
		RelatedDocs: e.RelatedDocs,
		PrevPage:    e.PrevPage,
		NextPage:    e.NextPage,
	}
}
