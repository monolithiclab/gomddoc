package enricher

import (
	"sync"

	"github.com/monolithiclab/gomddoc/internal/common"
)

// DefaultEnricherRegistry is a MIME-type-keyed registry of enrichers.
// It falls back to a NoOpEnricher for types without a dedicated enricher.
type DefaultEnricherRegistry struct {
	enrichers map[string]Enricher
	fallback  Enricher
	mu        sync.RWMutex
}

// NewDefaultEnricherRegistry creates a new enricher registry with a NoOp fallback.
func NewDefaultEnricherRegistry() *DefaultEnricherRegistry {
	return &DefaultEnricherRegistry{
		enrichers: make(map[string]Enricher),
		fallback:  &NoOpEnricher{},
	}
}

// Register adds an enricher to the registry for each of its supported MIME types.
func (r *DefaultEnricherRegistry) Register(e Enricher) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, mimeType := range e.SupportedMimeTypes() {
		r.enrichers[common.NormalizeMimeType(mimeType)] = e
	}
}

// Get returns the enricher for the given MIME type.
// Returns the NoOp fallback for unregistered types — never returns nil.
func (r *DefaultEnricherRegistry) Get(mimeType string) Enricher {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := common.NormalizeMimeType(mimeType)
	if e, ok := r.enrichers[normalized]; ok {
		return e
	}
	return r.fallback
}
