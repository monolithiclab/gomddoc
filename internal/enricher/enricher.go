package enricher

import "context"

// TOCNode represents a node in the table of contents.
// Moved from the renderer package to break the dependency cycle
// between enricher and renderer.
type TOCNode struct {
	Level    int
	Text     string
	ID       string
	Children []*TOCNode
}

// NavTree holds a format-agnostic navigation tree.
// Renderers decide how to present it (HTML sidebar, markdown links, etc.).
type NavTree struct {
	Items []NavItem
}

// NavItem represents a single node in the navigation tree.
type NavItem struct {
	Title    string
	Path     string
	IsDir    bool
	Active   bool // on the current page's path
	Open     bool // this node or a descendant is active
	Children []NavItem
}

// RelatedDoc represents a document related to the current page,
// typically linked by shared tags.
type RelatedDoc struct {
	Path  string `yaml:"path"`
	Title string `yaml:"title"`
}

// PageLink represents a link to a page with its display title.
// Used for previous/next navigation.
type PageLink struct {
	Path  string `yaml:"path"`
	Title string `yaml:"title"`
}

// EnrichmentData holds structured data extracted from content before rendering.
// The enricher runs before the renderer, producing format-agnostic data that
// renderers and templates can use.
type EnrichmentData struct {
	Metadata    map[string]any
	Features    map[string]bool // Page-level feature overrides from frontmatter
	TOC         *TOCNode
	Navigation  *NavTree
	RelatedDocs []RelatedDoc
	PrevPage    *PageLink // Previous page in navigation order
	NextPage    *PageLink // Next page in navigation order
}

// Enricher extracts structured data from content before rendering.
// Unlike renderers (which are stateless), enrichers may hold dependencies
// such as the metadata index and navigation builder.
type Enricher interface {
	// SupportedMimeTypes returns the MIME types this enricher handles.
	SupportedMimeTypes() []string

	// Enrich extracts structured data from content at the given path.
	Enrich(ctx context.Context, content []byte, path string) (*EnrichmentData, error)
}

// EnricherRegistry manages enricher mappings by input MIME type.
// Unlike the renderer registry, it always returns an enricher (falling back
// to a no-op enricher for unregistered types).
type EnricherRegistry interface {
	// Register adds an enricher to the registry.
	Register(enricher Enricher)

	// Get returns the enricher for the given MIME type.
	// Never returns nil — returns a NoOpEnricher for unregistered types.
	Get(mimeType string) Enricher
}
