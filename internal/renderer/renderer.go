package renderer

import "context"

// RenderResult holds the output of a content rendering operation.
type RenderResult struct {
	// Content is the transformed content bytes.
	Content []byte

	// MimeType is the MIME type of the output content.
	// Empty string means the input MIME type should be preserved (passthrough).
	MimeType string

	// Metadata is key-value pairs extracted from the content (e.g., front matter).
	// This can be used by templates for title, description, tags, etc.
	Metadata map[string]interface{}

	// TOC is the table of contents extracted from the content.
	TOC *TOCNode
}

// TOCNode represents a node in the table of contents.
type TOCNode struct {
	Level    int
	Text     string
	ID       string
	Children []*TOCNode
}

// ContentRenderer transforms content from input MIME type to output MIME type.
// Renderers are stateless and can be used concurrently.
//
// The interface follows the self-declaration pattern where renderers expose
// which MIME types they support, enabling automatic registry mapping without
// hardcoded configuration.
type ContentRenderer interface {
	// SupportedMimeTypes returns the list of MIME types this renderer can process.
	// MIME types should be normalized (without charset parameters).
	//
	// Examples:
	//   - "text/markdown"
	//   - "text/html"
	//   - "*/*" (wildcard for catch-all renderers)
	//
	// The registry uses these to automatically map MIME types to renderers.
	SupportedMimeTypes() []string

	// Render processes content and returns the transformed output.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout handling
	//   - content: Input content bytes to process
	//
	// Returns:
	//   - result: The rendering result containing content, MIME type, and metadata
	//   - error: Processing error, including context cancellation
	//
	// The renderer should check ctx.Err() before expensive operations to support
	// request cancellation and timeouts.
	Render(ctx context.Context, content []byte) (*RenderResult, error)
}

// RendererRegistry manages MIME type to renderer mappings.
// The registry is thread-safe and supports concurrent access.
type RendererRegistry interface {
	// Register adds a renderer and automatically maps all its supported MIME types.
	// If a MIME type is already registered, the new renderer overrides it and
	// a warning is logged via slog.Warn.
	//
	// This allows for:
	//   - Easy renderer registration without manual MIME type mapping
	//   - Testing with mock renderers that override built-in ones
	//   - Plugin systems that extend or replace default renderers
	Register(renderer ContentRenderer)

	// Get retrieves a renderer for the given MIME type.
	// The lookup supports wildcard matching in this order:
	//   1. Exact match (e.g., "text/markdown")
	//   2. Type wildcard (e.g., "text/*" matches "text/plain")
	//   3. Catch-all wildcard (e.g., "*/*" matches any type)
	//
	// Parameters:
	//   - mimeType: The MIME type to find a renderer for (will be normalized)
	//
	// Returns:
	//   - ContentRenderer: The matching renderer
	//   - error: ErrNoRenderer if no matching renderer is found
	//
	// The MIME type is automatically normalized before lookup to ensure
	// consistent matching (e.g., "text/html; charset=utf-8" → "text/html").
	Get(mimeType string) (ContentRenderer, error)
}
