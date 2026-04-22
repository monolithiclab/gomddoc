package enricher

import (
	"context"
	"sync"
	"testing"
)

// mockEnricher is a test enricher implementation.
type mockEnricher struct {
	name      string
	mimeTypes []string
}

func (m *mockEnricher) SupportedMimeTypes() []string {
	return m.mimeTypes
}

func (m *mockEnricher) Enrich(_ context.Context, _ []byte, _ string) (*EnrichmentData, error) {
	return &EnrichmentData{
		Metadata: map[string]any{"enricher": m.name},
	}, nil
}

func TestDefaultEnricherRegistry_Get_Registered(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()
	reg.Register(&mockEnricher{name: "markdown", mimeTypes: []string{"text/markdown"}})

	e := reg.Get("text/markdown")
	result, err := e.Enrich(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Metadata["enricher"] != "markdown" {
		t.Errorf("Got enricher %v, want 'markdown'", result.Metadata["enricher"])
	}
}

func TestDefaultEnricherRegistry_Get_Fallback(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()
	reg.Register(&mockEnricher{name: "markdown", mimeTypes: []string{"text/markdown"}})

	// Request an unregistered type — should get NoOp
	e := reg.Get("application/json")
	result, err := e.Enrich(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	// NoOp returns empty metadata
	if result.Metadata != nil {
		t.Errorf("Metadata = %v, want nil (NoOp)", result.Metadata)
	}
}

func TestDefaultEnricherRegistry_Get_NormalizesCharset(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()
	reg.Register(&mockEnricher{name: "markdown", mimeTypes: []string{"text/markdown"}})

	// Lookup with charset parameter — should still match
	e := reg.Get("text/markdown; charset=utf-8")
	result, err := e.Enrich(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Metadata["enricher"] != "markdown" {
		t.Errorf("Got enricher %v, want 'markdown'", result.Metadata["enricher"])
	}
}

func TestDefaultEnricherRegistry_Get_OverwritesOnDuplicate(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()
	reg.Register(&mockEnricher{name: "old", mimeTypes: []string{"text/markdown"}})
	reg.Register(&mockEnricher{name: "new", mimeTypes: []string{"text/markdown"}})

	e := reg.Get("text/markdown")
	result, err := e.Enrich(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result.Metadata["enricher"] != "new" {
		t.Errorf("Got enricher %v, want 'new' (latest registration)", result.Metadata["enricher"])
	}
}

func TestDefaultEnricherRegistry_Get_MultipleMimeTypes(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()
	reg.Register(&mockEnricher{name: "text", mimeTypes: []string{"text/markdown", "text/plain"}})

	for _, mime := range []string{"text/markdown", "text/plain"} {
		e := reg.Get(mime)
		result, err := e.Enrich(context.Background(), nil, "")
		if err != nil {
			t.Fatalf("Enrich() error for %s = %v", mime, err)
		}
		if result.Metadata["enricher"] != "text" {
			t.Errorf("Got enricher %v for %s, want 'text'", result.Metadata["enricher"], mime)
		}
	}
}

func TestDefaultEnricherRegistry_Get_EmptyRegistry(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()

	// Should return NoOp, never nil
	e := reg.Get("text/markdown")
	if e == nil {
		t.Fatal("Get() returned nil, should return NoOp fallback")
	}

	result, err := e.Enrich(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("NoOp Enrich() error = %v", err)
	}
	if result == nil {
		t.Fatal("NoOp Enrich() returned nil result")
	}
}

func TestDefaultEnricherRegistry_ThreadSafety(t *testing.T) {
	t.Parallel()
	reg := NewDefaultEnricherRegistry()

	var wg sync.WaitGroup
	concurrency := 100

	// Concurrent registrations
	for i := range concurrency {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			reg.Register(&mockEnricher{
				name:      string(rune('A' + (n % 26))),
				mimeTypes: []string{"application/test"},
			})
		}(i)
	}

	// Concurrent lookups
	for range concurrency {
		wg.Go(func() {
			e := reg.Get("application/test")
			_, _ = e.Enrich(context.Background(), nil, "")
		})
	}

	wg.Wait()

	// Registry should still be functional
	e := reg.Get("application/test")
	if e == nil {
		t.Fatal("Registry not functional after concurrent access")
	}
}
