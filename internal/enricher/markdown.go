package enricher

import (
	"context"
	"log/slog"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

// NavBuilder generates navigation items for a given path.
// This function type avoids importing the template/navigation package directly,
// preventing import cycles.
type NavBuilder func(currentPath string) []NavItem

// PrevNextBuilder computes the previous and next pages in navigation order.
type PrevNextBuilder func(currentPath string) (prev, next *PageLink)

// MarkdownEnricherOptions configures the MarkdownEnricher.
type MarkdownEnricherOptions struct {
	// MetaIndex provides access to the metadata index for related document lookup.
	// When nil, related documents are not resolved.
	MetaIndex *metadata.Index

	// NavBuilder generates navigation items for a given path.
	// When nil, navigation is not included in enrichment data.
	NavBuilder NavBuilder

	// PrevNextBuilder computes previous/next page links.
	// When nil, prev/next links are not included in enrichment data.
	PrevNextBuilder PrevNextBuilder
}

// MarkdownEnricher extracts structured data from markdown content.
// It parses YAML frontmatter, builds a table of contents from headings,
// generates navigation context, and finds related documents via shared tags.
type MarkdownEnricher struct {
	md              goldmark.Markdown
	metaIndex       *metadata.Index
	navBuilder      NavBuilder
	prevNextBuilder PrevNextBuilder
}

// NewMarkdownEnricher creates a new markdown enricher with the given options.
// Uses a lightweight goldmark instance (GFM + meta, no syntax highlighting
// or HTML rendering) since only parsing is needed.
func NewMarkdownEnricher(opts MarkdownEnricherOptions) *MarkdownEnricher {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
	return &MarkdownEnricher{
		md:              md,
		metaIndex:       opts.MetaIndex,
		navBuilder:      opts.NavBuilder,
		prevNextBuilder: opts.PrevNextBuilder,
	}
}

// SupportedMimeTypes returns the MIME types this enricher handles.
func (m *MarkdownEnricher) SupportedMimeTypes() []string {
	return []string{"text/markdown"}
}

// Enrich extracts metadata, TOC, navigation, and related documents from markdown content.
func (m *MarkdownEnricher) Enrich(ctx context.Context, content []byte, path string) (*EnrichmentData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pCtx := parser.NewContext()
	reader := text.NewReader(content)
	doc := m.md.Parser().Parse(reader, parser.WithContext(pCtx))

	// Extract TOC from parsed AST
	tocItems, err := toc.Inspect(doc, content)
	if err != nil {
		slog.Warn("Failed to extract TOC", slog.Any("error", err))
	}

	var tocNode *TOCNode
	if tocItems != nil {
		tocNode = ConvertTOC(tocItems.Items)
	}

	// Extract frontmatter metadata
	mdMeta := meta.Get(pCtx)

	enrichment := &EnrichmentData{
		Metadata: mdMeta,
		TOC:      tocNode,
	}

	// Build navigation if builder is available
	if m.navBuilder != nil {
		items := m.navBuilder(path)
		if len(items) > 0 {
			enrichment.Navigation = &NavTree{Items: items}
		}
	}

	// Compute previous/next page links
	if m.prevNextBuilder != nil {
		enrichment.PrevPage, enrichment.NextPage = m.prevNextBuilder(path)
	}

	// Find related documents via shared tags
	if m.metaIndex != nil && mdMeta != nil {
		enrichment.RelatedDocs = m.findRelatedDocs(path, mdMeta)
	}

	return enrichment, nil
}

// findRelatedDocs finds documents that share tags with the current page.
func (m *MarkdownEnricher) findRelatedDocs(currentPath string, mdMeta map[string]any) []RelatedDoc {
	tagsRaw, ok := mdMeta["tags"]
	if !ok {
		return nil
	}

	tags, ok := tagsRaw.([]any)
	if !ok {
		return nil
	}

	// Collect unique related docs by path
	seen := map[string]bool{currentPath: true}
	var related []RelatedDoc

	for _, tagRaw := range tags {
		tag, ok := tagRaw.(string)
		if !ok {
			continue
		}

		pages := m.metaIndex.ByTag(tag)
		for _, page := range pages {
			if seen[page.Path] {
				continue
			}
			seen[page.Path] = true
			related = append(related, RelatedDoc{
				Path:  page.Path,
				Title: page.Title,
			})
		}
	}

	return related
}
