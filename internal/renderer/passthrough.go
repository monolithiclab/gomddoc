package renderer

import (
	"context"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

// PassthroughRenderer is a no-op renderer that returns content unchanged.
// It registers with the wildcard MIME type "*/*" to act as a catch-all
// fallback when no specific renderer is available.
//
// The renderer is stateless and thread-safe.
type PassthroughRenderer struct{}

// NewPassthroughRenderer creates a new passthrough renderer.
func NewPassthroughRenderer() *PassthroughRenderer {
	return &PassthroughRenderer{}
}

// InputMimeTypes returns the wildcard MIME type "*/*" to match any content.
func (p *PassthroughRenderer) InputMimeTypes() []string {
	return []string{"*/*"}
}

// OutputMimeTypes returns "*/*" — output type mirrors the input type.
func (p *PassthroughRenderer) OutputMimeTypes() []string {
	return []string{"*/*"}
}

// Render returns the content unchanged with an empty output MIME type.
//
// Context handling:
//   - Checks ctx.Err() before processing to support cancellation
//   - Returns context.Canceled or context.DeadlineExceeded if cancelled
//
// The empty output MIME type signals to the handler to use the provider's
// detected MIME type for the HTTP response, enabling proper content type
// headers for binary files, images, etc.
func (p *PassthroughRenderer) Render(ctx context.Context, content []byte, _ *enricher.EnrichmentData) (*RenderResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &RenderResult{
		Content:  content,
		MimeType: "",
	}, nil
}
