package enricher

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/toc"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	txt "github.com/monolithiclab/gomddoc/internal/text"
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
		Features: config.ExtractPageFeatures(mdMeta),
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

	// Normalize the .md suffix so a request whose URL has been
	// extension-stripped (/foo) still matches the metadata index's raw path
	// (/foo.md) for self-exclusion.
	currentKey := strings.TrimSuffix(currentPath, ".md")

	// related is kept sorted and never grows past maxRelatedDocs, so its last
	// element is the worst candidate admitted so far. Anything ordered after it
	// would be truncated anyway, so it is dropped on sight rather than
	// collected: this runs on every markdown request and once per file in a
	// static build, and one site-wide tag gives every page the whole corpus as
	// candidates.
	related := make([]RelatedDoc, 0, maxRelatedDocs)

	for _, tagRaw := range tags {
		tag, ok := tagRaw.(string)
		if !ok {
			continue
		}

		for page := range m.metaIndex.PagesByTag(tag) {
			if strings.TrimSuffix(page.Path, ".md") == currentKey {
				continue
			}

			doc := RelatedDoc{Path: page.Path, Title: page.Title}
			full := len(related) == maxRelatedDocs
			if full && compareRelatedDocs(doc, related[maxRelatedDocs-1]) >= 0 {
				continue
			}
			// A page carrying two of this page's tags is visited twice. The
			// window is the only place a duplicate can land — a page evicted
			// from it is, by definition, worse than the current worst and gets
			// rejected above — so dedup costs a scan of ten entries rather than
			// a set over the whole corpus.
			if slices.ContainsFunc(related, func(d RelatedDoc) bool { return d.Path == doc.Path }) {
				continue
			}

			if full {
				related[maxRelatedDocs-1] = doc
			} else {
				related = append(related, doc)
			}

			// Sort deterministically so both serve and build render the
			// see-also section in the same, stable order (tag-iteration order
			// is otherwise an implementation detail). Title first, path as a
			// stable tiebreaker. Bounded to maxRelatedDocs elements, and only
			// reached by a candidate that improves the window.
			slices.SortFunc(related, compareRelatedDocs)
		}
	}

	if len(related) == 0 {
		return nil
	}
	return related
}

// maxRelatedDocs bounds the see-also section so a page with a very common tag
// does not render an unbounded list.
const maxRelatedDocs = 10

// compareRelatedDocs orders related docs by title (case-insensitive) with path
// as a stable tiebreaker, mirroring metadata.CompareTitles for PageInfo.
func compareRelatedDocs(a, b RelatedDoc) int {
	if c := txt.CompareTitles(a.Title, b.Title); c != 0 {
		return c
	}
	return cmp.Compare(a.Path, b.Path)
}
