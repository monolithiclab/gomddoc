package renderer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// mockRenderer is a test renderer implementation
type mockRenderer struct {
	name       string
	inputMime  []string
	outputMime []string
}

func (m *mockRenderer) InputMimeTypes() []string {
	return m.inputMime
}

func (m *mockRenderer) OutputMimeTypes() []string {
	return m.outputMime
}

func (m *mockRenderer) Render(ctx context.Context, content []byte) (*RenderResult, error) {
	return &RenderResult{
		Content:  content,
		MimeType: "",
		Metadata: nil,
	}, nil
}

// acceptAll is a convenience for tests that don't care about output negotiation.
var acceptAll = []negotiate.MediaType{{Type: "*", Subtype: "*", Q: 1.0}}

func acceptOnly(mimeType string) []negotiate.MediaType {
	mt := negotiate.ParseAccept(mimeType)
	return mt
}

func TestDefaultRegistry_Register(t *testing.T) {
	tests := []struct {
		name      string
		renderers []ContentRenderer
		lookupFor string
		accepted  []negotiate.MediaType
		wantFound bool
		wantName  string
	}{
		{
			name: "single renderer registration",
			renderers: []ContentRenderer{
				&mockRenderer{name: "markdown", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor: "text/markdown",
			accepted:  acceptAll,
			wantFound: true,
			wantName:  "markdown",
		},
		{
			name: "multiple input MIME types",
			renderers: []ContentRenderer{
				&mockRenderer{name: "html", inputMime: []string{"text/html", "application/xhtml+xml"}, outputMime: []string{"text/html"}},
			},
			lookupFor: "application/xhtml+xml",
			accepted:  acceptAll,
			wantFound: true,
			wantName:  "html",
		},
		{
			name: "later registration wins on tie",
			renderers: []ContentRenderer{
				&mockRenderer{name: "old", inputMime: []string{"text/plain"}, outputMime: []string{"text/html"}},
				&mockRenderer{name: "new", inputMime: []string{"text/plain"}, outputMime: []string{"text/html"}},
			},
			lookupFor: "text/plain",
			accepted:  acceptAll,
			wantFound: true,
			wantName:  "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewDefaultRegistry()

			for _, r := range tt.renderers {
				registry.Register(r)
			}

			renderer, _, err := registry.Get(tt.lookupFor, tt.accepted)
			if tt.wantFound {
				if err != nil {
					t.Fatalf("Get() error = %v, want nil", err)
				}
				if mock, ok := renderer.(*mockRenderer); ok {
					if mock.name != tt.wantName {
						t.Errorf("Get() returned renderer %q, want %q", mock.name, tt.wantName)
					}
				} else {
					t.Error("Get() returned non-mock renderer")
				}
			} else if err == nil {
				t.Error("Get() error = nil, want error")
			}
		})
	}
}

func TestDefaultRegistry_Get_2D(t *testing.T) {
	tests := []struct {
		name           string
		renderers      []mockRenderer
		lookupFor      string
		accepted       []negotiate.MediaType
		wantFound      bool
		wantRenderer   string
		wantOutputType string
	}{
		{
			name: "exact input and output match",
			renderers: []mockRenderer{
				{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/markdown",
			accepted:       acceptOnly("text/html"),
			wantFound:      true,
			wantRenderer:   "md-html",
			wantOutputType: "text/html",
		},
		{
			name: "accept text/markdown selects passthrough",
			renderers: []mockRenderer{
				{name: "md-passthrough", inputMime: []string{"text/markdown"}, outputMime: []string{"text/markdown"}},
				{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/markdown",
			accepted:       acceptOnly("text/markdown"),
			wantFound:      true,
			wantRenderer:   "md-passthrough",
			wantOutputType: "text/markdown",
		},
		{
			name: "accept */* prefers later registered (HTML over passthrough)",
			renderers: []mockRenderer{
				{name: "md-passthrough", inputMime: []string{"text/markdown"}, outputMime: []string{"text/markdown"}},
				{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/markdown",
			accepted:       acceptAll,
			wantFound:      true,
			wantRenderer:   "md-html",
			wantOutputType: "text/html",
		},
		{
			name: "input wildcard catch-all",
			renderers: []mockRenderer{
				{name: "catch-all", inputMime: []string{"*/*"}, outputMime: []string{"*/*"}},
			},
			lookupFor:      "application/json",
			accepted:       acceptAll,
			wantFound:      true,
			wantRenderer:   "catch-all",
			wantOutputType: "application/json",
		},
		{
			name: "exact input wins over wildcard input",
			renderers: []mockRenderer{
				{name: "catch-all", inputMime: []string{"*/*"}, outputMime: []string{"*/*"}},
				{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/markdown",
			accepted:       acceptAll,
			wantFound:      true,
			wantRenderer:   "md-html",
			wantOutputType: "text/html",
		},
		{
			name: "type wildcard input match",
			renderers: []mockRenderer{
				{name: "text-wildcard", inputMime: []string{"text/*"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/plain",
			accepted:       acceptAll,
			wantFound:      true,
			wantRenderer:   "text-wildcard",
			wantOutputType: "text/html",
		},
		{
			name: "MIME type with charset normalized",
			renderers: []mockRenderer{
				{name: "html", inputMime: []string{"text/html"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/html; charset=utf-8",
			accepted:       acceptAll,
			wantFound:      true,
			wantRenderer:   "html",
			wantOutputType: "text/html",
		},
		{
			name: "no input match",
			renderers: []mockRenderer{
				{name: "markdown", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor: "application/json",
			accepted:  acceptAll,
			wantFound: false,
		},
		{
			name: "no output match returns error",
			renderers: []mockRenderer{
				{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor: "text/markdown",
			accepted:  acceptOnly("application/json"),
			wantFound: false,
		},
		{
			name: "q-value priority: prefer higher q",
			renderers: []mockRenderer{
				{name: "md-passthrough", inputMime: []string{"text/markdown"}, outputMime: []string{"text/markdown"}},
				{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}},
			},
			lookupFor:      "text/markdown",
			accepted:       negotiate.ParseAccept("text/markdown;q=0.9, text/html;q=1.0"),
			wantFound:      true,
			wantRenderer:   "md-html",
			wantOutputType: "text/html",
		},
		{
			name:         "empty accepted defaults to */*",
			renderers:    []mockRenderer{{name: "a", inputMime: []string{"text/plain"}, outputMime: []string{"text/html"}}},
			lookupFor:    "text/plain",
			accepted:     nil,
			wantFound:    true,
			wantRenderer: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewDefaultRegistry()

			for i := range tt.renderers {
				registry.Register(&tt.renderers[i])
			}

			renderer, outputType, err := registry.Get(tt.lookupFor, tt.accepted)

			if tt.wantFound {
				if err != nil {
					t.Fatalf("Get() error = %v, want nil", err)
				}
				if mock, ok := renderer.(*mockRenderer); ok {
					if mock.name != tt.wantRenderer {
						t.Errorf("Get() returned renderer %q, want %q", mock.name, tt.wantRenderer)
					}
				}
				if tt.wantOutputType != "" && outputType != tt.wantOutputType {
					t.Errorf("Get() outputType = %q, want %q", outputType, tt.wantOutputType)
				}
			} else if err == nil {
				t.Error("Get() error = nil, want error")
			}
		})
	}
}

func TestDefaultRegistry_AvailableOutputTypes(t *testing.T) {
	registry := NewDefaultRegistry()
	registry.Register(&mockRenderer{name: "md-pt", inputMime: []string{"text/markdown"}, outputMime: []string{"text/markdown"}})
	registry.Register(&mockRenderer{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}})
	registry.Register(&mockRenderer{name: "catch-all", inputMime: []string{"*/*"}, outputMime: []string{"*/*"}})

	types := registry.AvailableOutputTypes("text/markdown")
	// text/markdown (from md-pt), text/html (from md-html), text/markdown (from catch-all */* resolved — deduped)
	if len(types) != 2 {
		t.Fatalf("AvailableOutputTypes() returned %d types, want 2; got %v", len(types), types)
	}

	want := map[string]bool{"text/markdown": true, "text/html": true}
	for _, typ := range types {
		delete(want, typ)
	}
	if len(want) > 0 {
		t.Errorf("AvailableOutputTypes() missing: %v", want)
	}
}

func TestDefaultRegistry_ThreadSafety(t *testing.T) {
	registry := NewDefaultRegistry()

	var wg sync.WaitGroup
	concurrency := 100

	// Concurrent registrations
	for i := range concurrency {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			renderer := &mockRenderer{
				name:       string(rune('A' + (n % 26))),
				inputMime:  []string{"application/test"},
				outputMime: []string{"text/html"},
			}
			registry.Register(renderer)
		}(i)
	}

	// Concurrent lookups
	for range concurrency {
		wg.Go(func() {
			_, _, _ = registry.Get("application/test", acceptAll)
		})
	}

	wg.Wait()

	// Verify registry is still functional
	_, _, err := registry.Get("application/test", acceptAll)
	if err != nil {
		t.Errorf("Registry not functional after concurrent access: %v", err)
	}
}

func TestDefaultRegistry_EmptyRegistry(t *testing.T) {
	registry := NewDefaultRegistry()

	_, _, err := registry.Get("text/plain", acceptAll)
	if err == nil {
		t.Error("Get() on empty registry should return error")
	}
	if !errors.Is(err, ErrNoRenderer) {
		t.Errorf("Get() error = %v, want ErrNoRenderer", err)
	}
}

func TestDefaultRegistry_NoMatchingOutput(t *testing.T) {
	registry := NewDefaultRegistry()
	registry.Register(&mockRenderer{name: "md-html", inputMime: []string{"text/markdown"}, outputMime: []string{"text/html"}})

	_, _, err := registry.Get("text/markdown", acceptOnly("application/json"))
	if err == nil {
		t.Error("Get() should return error when no output matches")
	}
	if !errors.Is(err, ErrNoMatchingRenderer) {
		t.Errorf("Get() error = %v, want ErrNoMatchingRenderer", err)
	}
}
