package renderer

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// ErrNoRenderer is returned when no renderer is found for a given input MIME type.
var ErrNoRenderer = errors.New("no renderer found")

// ErrNoMatchingRenderer is returned when a renderer exists for the input type
// but cannot produce any of the accepted output types.
var ErrNoMatchingRenderer = errors.New("no matching renderer for accepted output types")

// registryEntry pairs a renderer with its registration order for tie-breaking.
type registryEntry struct {
	renderer ContentRenderer
	order    int
}

// DefaultRegistry implements RendererRegistry with two-dimensional lookup:
// input MIME type + accepted output types.
//
// Thread-safe: All methods use sync.RWMutex for concurrent access.
type DefaultRegistry struct {
	entries []registryEntry
	mu      sync.RWMutex
}

// NewDefaultRegistry creates a new empty registry.
func NewDefaultRegistry() *DefaultRegistry {
	return &DefaultRegistry{}
}

// Register adds a renderer to the registry. Later registrations take
// precedence over earlier ones when multiple renderers match equally.
func (r *DefaultRegistry) Register(renderer ContentRenderer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, registryEntry{
		renderer: renderer,
		order:    len(r.entries),
	})
}

// Get finds the best renderer for the given input MIME type and accepted
// output types using two-dimensional matching.
//
// Algorithm:
//  1. Filter entries whose InputMimeTypes() match the input
//     (exact match > type/* > */*)
//  2. For each accepted type (sorted by q-value, highest first):
//     score each candidate's output types by specificity
//     (exact=3, type/*=2, */*=1). On tie, latest registered wins.
//  3. Return best match or ErrNoMatchingRenderer.
func (r *DefaultRegistry) Get(inputMimeType string, accepted []negotiate.MediaType) (ContentRenderer, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := negotiate.NormalizeMimeType(inputMimeType)

	if len(accepted) == 0 {
		accepted = []negotiate.MediaType{{Type: "*", Subtype: "*", Q: 1.0}}
	}

	// Step 1: Find all entries that match the input MIME type with their scores.
	type candidate struct {
		entry      registryEntry
		inputScore int // exact=3, type/*=2, */*=1
	}
	// Stack-allocated array avoids heap allocation for typical registries (≤8 renderers).
	var candidateBuf [8]candidate
	candidates := candidateBuf[:0]
	for _, entry := range r.entries {
		score := inputMatchScore(entry.renderer.InputMimeTypes(), normalized)
		if score > 0 {
			candidates = append(candidates, candidate{entry: entry, inputScore: score})
		}
	}

	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("%w: %s", ErrNoRenderer, normalized)
	}

	// Step 2: For each accepted type (by q-value priority), find best candidate.
	var bestRenderer ContentRenderer
	var bestOutputType string
	bestScore := 0
	bestOrder := -1

	for _, acc := range accepted {
		for _, cand := range candidates {
			for _, outType := range cand.entry.renderer.OutputMimeTypes() {
				// Resolve wildcard output to the actual input type before matching
				resolved := resolveOutputType(outType, normalized)
				score := outputMatchScore(acc, resolved)
				if score == 0 {
					continue
				}

				// Combine input specificity + output specificity
				totalScore := cand.inputScore*10 + score
				if totalScore > bestScore || (totalScore == bestScore && cand.entry.order > bestOrder) {
					bestScore = totalScore
					bestOrder = cand.entry.order
					bestRenderer = cand.entry.renderer
					bestOutputType = resolved
				}
			}
		}

		// If we found a match at this q-level, stop (higher q takes priority)
		if bestRenderer != nil {
			return bestRenderer, bestOutputType, nil
		}
	}

	return nil, "", fmt.Errorf("%w: input=%s", ErrNoMatchingRenderer, normalized)
}

// AvailableOutputTypes returns all output MIME types that registered
// renderers can produce for the given input MIME type.
func (r *DefaultRegistry) AvailableOutputTypes(inputMimeType string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := negotiate.NormalizeMimeType(inputMimeType)
	seen := make(map[string]bool)
	var types []string

	for _, entry := range r.entries {
		if inputMatchScore(entry.renderer.InputMimeTypes(), normalized) > 0 {
			for _, outType := range entry.renderer.OutputMimeTypes() {
				resolved := resolveOutputType(outType, normalized)
				if !seen[resolved] {
					seen[resolved] = true
					types = append(types, resolved)
				}
			}
		}
	}

	return types
}

// inputMatchScore returns how well the renderer's input types match the
// given MIME type. Returns 0 for no match.
//
//	exact=3, type/*=2, */*=1
func inputMatchScore(inputTypes []string, mimeType string) int {
	best := 0
	slash := strings.IndexByte(mimeType, '/')
	for _, it := range inputTypes {
		switch {
		case it == mimeType:
			return 3 // Exact match — best possible
		case it == "*/*" && best < 1:
			best = 1
		case slash > 0 && best < 2 && len(it) == slash+2 &&
			it[slash] == '/' && it[slash+1] == '*' &&
			it[:slash] == mimeType[:slash]:
			best = 2
		}
	}
	return best
}

// outputMatchScore returns how well an accepted MediaType matches a resolved
// output type. The outputType must already be resolved (no wildcards).
// Returns 0 for no match.
//
//	exact=3, type/*=2, */*=1
func outputMatchScore(accepted negotiate.MediaType, outputType string) int {
	slash := strings.IndexByte(outputType, '/')
	if slash <= 0 {
		return 0
	}

	if accepted.Type == "*" {
		return 1
	}
	if accepted.Type != outputType[:slash] {
		return 0
	}
	if accepted.Subtype == "*" {
		return 2
	}
	if accepted.Subtype == outputType[slash+1:] {
		return 3
	}
	return 0
}

// resolveOutputType resolves wildcard output types to concrete types.
// If the output type is "*/*", it resolves to the input MIME type.
func resolveOutputType(outputType, inputMimeType string) string {
	if outputType == "*/*" {
		return inputMimeType
	}
	return outputType
}
