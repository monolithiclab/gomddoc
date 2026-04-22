package renderer

import (
	"testing"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// setupBenchRegistry creates a registry with mock renderers mimicking
// production MIME type configurations:
//   - markdown renderer: text/markdown -> text/html
//   - markdown passthrough: text/markdown -> text/markdown
//   - catch-all passthrough: */* -> */*
func setupBenchRegistry() *DefaultRegistry {
	reg := NewDefaultRegistry()
	reg.Register(&mockRenderer{
		name:       "markdown",
		inputMime:  []string{"text/markdown"},
		outputMime: []string{"text/html"},
	})
	reg.Register(&mockRenderer{
		name:       "markdown-passthrough",
		inputMime:  []string{"text/markdown"},
		outputMime: []string{"text/markdown"},
	})
	reg.Register(&mockRenderer{
		name:       "passthrough",
		inputMime:  []string{"*/*"},
		outputMime: []string{"*/*"},
	})
	return reg
}

func BenchmarkRegistryGet_ExactMatch(b *testing.B) {
	b.ReportAllocs()

	reg := setupBenchRegistry()
	accepted := negotiate.ParseAccept("text/html")

	for b.Loop() {
		_, _, _ = reg.Get("text/markdown", accepted)
	}
}

func BenchmarkRegistryGet_WildcardFallback(b *testing.B) {
	b.ReportAllocs()

	reg := setupBenchRegistry()
	accepted := negotiate.ParseAccept("*/*")

	for b.Loop() {
		_, _, _ = reg.Get("application/json", accepted)
	}
}

func BenchmarkRegistryGet_MultipleAcceptTypes(b *testing.B) {
	b.ReportAllocs()

	reg := setupBenchRegistry()
	accepted := negotiate.ParseAccept("text/html, text/markdown;q=0.9, */*;q=0.1")

	for b.Loop() {
		_, _, _ = reg.Get("text/markdown", accepted)
	}
}

func BenchmarkRegistryGet_NoMatch406(b *testing.B) {
	b.ReportAllocs()

	reg := setupBenchRegistry()
	accepted := negotiate.ParseAccept("application/xml")

	for b.Loop() {
		_, _, _ = reg.Get("text/markdown", accepted)
	}
}
