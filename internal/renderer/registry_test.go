package renderer

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// mockRenderer is a test renderer implementation
type mockRenderer struct {
	name          string
	supportedMime []string
}

func (m *mockRenderer) SupportedMimeTypes() []string {
	return m.supportedMime
}

func (m *mockRenderer) Render(ctx context.Context, content []byte) ([]byte, string, error) {
	return content, "", nil
}

func TestDefaultRegistry_Register(t *testing.T) {
	tests := []struct {
		name      string
		renderers []ContentRenderer
		lookupFor string
		wantFound bool
		wantName  string
	}{
		{
			name: "single renderer registration",
			renderers: []ContentRenderer{
				&mockRenderer{name: "markdown", supportedMime: []string{"text/markdown"}},
			},
			lookupFor: "text/markdown",
			wantFound: true,
			wantName:  "markdown",
		},
		{
			name: "multiple MIME types",
			renderers: []ContentRenderer{
				&mockRenderer{name: "html", supportedMime: []string{"text/html", "application/xhtml+xml"}},
			},
			lookupFor: "application/xhtml+xml",
			wantFound: true,
			wantName:  "html",
		},
		{
			name: "override existing renderer",
			renderers: []ContentRenderer{
				&mockRenderer{name: "old", supportedMime: []string{"text/plain"}},
				&mockRenderer{name: "new", supportedMime: []string{"text/plain"}},
			},
			lookupFor: "text/plain",
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

			renderer, err := registry.Get(tt.lookupFor)
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

func TestDefaultRegistry_Get_Wildcards(t *testing.T) {
	tests := []struct {
		name         string
		renderers    []mockRenderer
		lookupFor    string
		wantFound    bool
		wantRenderer string
	}{
		{
			name: "exact match",
			renderers: []mockRenderer{
				{name: "markdown", supportedMime: []string{"text/markdown"}},
			},
			lookupFor:    "text/markdown",
			wantFound:    true,
			wantRenderer: "markdown",
		},
		{
			name: "type wildcard match",
			renderers: []mockRenderer{
				{name: "text-wildcard", supportedMime: []string{"text/*"}},
			},
			lookupFor:    "text/plain",
			wantFound:    true,
			wantRenderer: "text-wildcard",
		},
		{
			name: "catch-all wildcard match",
			renderers: []mockRenderer{
				{name: "catch-all", supportedMime: []string{"*/*"}},
			},
			lookupFor:    "application/json",
			wantFound:    true,
			wantRenderer: "catch-all",
		},
		{
			name: "exact match wins over wildcard",
			renderers: []mockRenderer{
				{name: "exact", supportedMime: []string{"text/markdown"}},
				{name: "wildcard", supportedMime: []string{"text/*"}},
			},
			lookupFor:    "text/markdown",
			wantFound:    true,
			wantRenderer: "exact",
		},
		{
			name: "type wildcard wins over catch-all",
			renderers: []mockRenderer{
				{name: "type-wildcard", supportedMime: []string{"text/*"}},
				{name: "catch-all", supportedMime: []string{"*/*"}},
			},
			lookupFor:    "text/plain",
			wantFound:    true,
			wantRenderer: "type-wildcard",
		},
		{
			name: "MIME type with charset normalized",
			renderers: []mockRenderer{
				{name: "html", supportedMime: []string{"text/html"}},
			},
			lookupFor:    "text/html; charset=utf-8",
			wantFound:    true,
			wantRenderer: "html",
		},
		{
			name: "no match",
			renderers: []mockRenderer{
				{name: "markdown", supportedMime: []string{"text/markdown"}},
			},
			lookupFor: "application/json",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewDefaultRegistry()

			// Register all renderers for this test case
			for i := range tt.renderers {
				registry.Register(&tt.renderers[i])
			}

			renderer, err := registry.Get(tt.lookupFor)

			if tt.wantFound {
				if err != nil {
					t.Fatalf("Get() error = %v, want nil", err)
				}
				if mock, ok := renderer.(*mockRenderer); ok {
					if mock.name != tt.wantRenderer {
						t.Errorf("Get() returned renderer %q, want %q", mock.name, tt.wantRenderer)
					}
				}
			} else {
				if err == nil {
					t.Error("Get() error = nil, want error")
				}
				if !errors.Is(err, ErrNoRenderer) {
					t.Errorf("Get() error = %v, want ErrNoRenderer", err)
				}
			}
		})
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
				name:          string(rune('A' + (n % 26))),
				supportedMime: []string{"application/test"},
			}
			registry.Register(renderer)
		}(i)
	}

	// Concurrent lookups
	for range concurrency {
		wg.Go(func() {
			_, _ = registry.Get("application/test")
		})
	}

	wg.Wait()

	// Verify registry is still functional
	_, err := registry.Get("application/test")
	if err != nil {
		t.Errorf("Registry not functional after concurrent access: %v", err)
	}
}

func TestDefaultRegistry_EmptyRegistry(t *testing.T) {
	registry := NewDefaultRegistry()

	_, err := registry.Get("text/plain")
	if err == nil {
		t.Error("Get() on empty registry should return error")
	}
	if !errors.Is(err, ErrNoRenderer) {
		t.Errorf("Get() error = %v, want ErrNoRenderer", err)
	}
}
