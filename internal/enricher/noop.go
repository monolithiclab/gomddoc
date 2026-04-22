package enricher

import "context"

// NoOpEnricher returns empty enrichment data for content types
// that don't have a dedicated enricher. Used as the registry fallback.
type NoOpEnricher struct{}

// SupportedMimeTypes returns an empty list since the no-op enricher
// is not registered for specific types — it's the default fallback.
func (n *NoOpEnricher) SupportedMimeTypes() []string {
	return nil
}

// Enrich returns empty enrichment data.
func (n *NoOpEnricher) Enrich(_ context.Context, _ []byte, _ string) (*EnrichmentData, error) {
	return &EnrichmentData{}, nil
}
