// Package registry constructs the renderer and enricher registries a test
// needs, so a test exercising something else does not have to restate the
// production wiring — and does not quietly drift from it.
package registry

import (
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/renderer"
)

// NewRendererRegistry creates a renderer registry with the default markdown,
// markdown passthrough, and passthrough renderers, suitable for unit tests.
func NewRendererRegistry() renderer.RendererRegistry {
	reg := renderer.NewDefaultRegistry()
	reg.Register(renderer.NewMarkdownPassthroughRenderer())
	reg.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{}))
	reg.Register(renderer.NewPassthroughRenderer())
	return reg
}

// NewEnricherRegistry creates an enricher registry with the default markdown
// enricher, suitable for unit tests.
func NewEnricherRegistry() enricher.EnricherRegistry {
	reg := enricher.NewDefaultEnricherRegistry()
	reg.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{}))
	return reg
}
