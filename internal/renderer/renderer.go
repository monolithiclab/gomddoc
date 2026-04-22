package renderer

import (
	"context"

	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// RenderResult holds the output of a content rendering operation.
// Metadata and TOC are provided by the enricher, not the renderer.
type RenderResult struct {
	// Content is the transformed content bytes.
	Content []byte

	// MimeType is the MIME type of the output content.
	// Empty string means the input MIME type should be preserved (passthrough).
	MimeType string
}

// ContentRenderer transforms content from input MIME type to output MIME type.
// Renderers are stateless and can be used concurrently.
//
// The interface follows the self-declaration pattern where renderers expose
// which MIME types they support, enabling automatic registry mapping without
// hardcoded configuration. Both input and output types are declared, enabling
// two-dimensional content negotiation (input type + Accept header).
type ContentRenderer interface {
	// InputMimeTypes returns the MIME types this renderer can accept as input.
	InputMimeTypes() []string

	// OutputMimeTypes returns the MIME types this renderer can produce as output.
	OutputMimeTypes() []string

	// Render processes content and returns the transformed output.
	// The enrichment parameter provides pre-extracted metadata, TOC, navigation,
	// and related documents. Renderers may use or ignore enrichment data.
	//
	// The renderer should check ctx.Err() before expensive operations to support
	// request cancellation and timeouts.
	Render(ctx context.Context, content []byte, enrichment *enricher.EnrichmentData) (*RenderResult, error)
}

// RendererRegistry manages content renderer mappings with two-dimensional
// lookup: input MIME type + accepted output types (from Accept header).
// The registry is thread-safe and supports concurrent access.
type RendererRegistry interface {
	// Register adds a renderer to the registry.
	Register(renderer ContentRenderer)

	// Get finds the best renderer for the given input MIME type and accepted
	// output types. Returns the renderer, the selected output MIME type, and
	// an error if no match is found.
	//
	// The accepted parameter comes from parsing the HTTP Accept header.
	// If accepted is empty, it defaults to accepting anything (*/*).
	//
	// Returns ErrNoMatchingRenderer if no renderer can produce an acceptable
	// output type for the given input.
	Get(inputMimeType string, accepted []negotiate.MediaType) (ContentRenderer, string, error)

	// AvailableOutputTypes returns all output MIME types that registered
	// renderers can produce for the given input MIME type. Used for 406
	// Not Acceptable responses.
	AvailableOutputTypes(inputMimeType string) []string
}
