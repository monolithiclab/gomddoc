package renderer

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// ErrNoRenderer is returned when no renderer is found for a given MIME type.
// Callers can use errors.Is(err, ErrNoRenderer) to check for this condition.
var ErrNoRenderer = errors.New("no renderer found")

// DefaultRegistry implements RendererRegistry with support for wildcard MIME type matching.
//
// Thread-safe: All methods use sync.RWMutex for concurrent access.
// Register() acquires a write lock, Get() acquires a read lock.
// Safe for concurrent registration and lookup from multiple goroutines.
type DefaultRegistry struct {
	renderers map[string]ContentRenderer
	mu        sync.RWMutex
}

// NewDefaultRegistry creates a new empty registry.
func NewDefaultRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		renderers: make(map[string]ContentRenderer),
	}
}

// Register adds a renderer and automatically maps all its supported MIME types.
// If a MIME type is already registered, the new renderer overrides it and
// a warning is logged.
//
// This warn-and-override behavior enables:
//   - Testing with mock renderers
//   - Plugin systems that extend functionality
//   - Runtime renderer replacement
func (r *DefaultRegistry) Register(renderer ContentRenderer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, mimeType := range renderer.SupportedMimeTypes() {
		if existing, ok := r.renderers[mimeType]; ok {
			slog.Warn("Overriding renderer for MIME type",
				slog.String("mime_type", mimeType),
				slog.String("old_renderer", fmt.Sprintf("%T", existing)),
				slog.String("new_renderer", fmt.Sprintf("%T", renderer)),
			)
		}
		r.renderers[mimeType] = renderer
	}
}

// Get retrieves a renderer for the given MIME type with wildcard support.
//
// Lookup order:
//  1. Exact match: "text/markdown"
//  2. Type wildcard: "text/*" matches "text/plain"
//  3. Catch-all: "*/*" matches any type
//
// The MIME type is automatically normalized before lookup to handle
// types with parameters (e.g., "text/html; charset=utf-8" → "text/html").
func (r *DefaultRegistry) Get(mimeType string) (ContentRenderer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := NormalizeMimeType(mimeType)

	// Try exact match first
	if renderer, ok := r.renderers[normalized]; ok {
		return renderer, nil
	}

	// Try type/* wildcard (e.g., "text/*" for "text/plain")
	parts := strings.Split(normalized, "/")
	if len(parts) == 2 {
		typeWildcard := parts[0] + "/*"
		if renderer, ok := r.renderers[typeWildcard]; ok {
			return renderer, nil
		}
	}

	// Try */* wildcard (catch-all)
	if renderer, ok := r.renderers["*/*"]; ok {
		return renderer, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrNoRenderer, normalized)
}
