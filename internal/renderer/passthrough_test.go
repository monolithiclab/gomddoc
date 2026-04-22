package renderer

import (
	"bytes"
	"context"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestPassthroughRenderer_MimeTypes(t *testing.T) {
	renderer := NewPassthroughRenderer()

	inputTypes := renderer.InputMimeTypes()
	if len(inputTypes) != 1 || inputTypes[0] != "*/*" {
		t.Errorf("InputMimeTypes() = %v, want [*/*]", inputTypes)
	}

	outputTypes := renderer.OutputMimeTypes()
	if len(outputTypes) != 1 || outputTypes[0] != "*/*" {
		t.Errorf("OutputMimeTypes() = %v, want [*/*]", outputTypes)
	}
}

func TestPassthroughRenderer_Render(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "text content",
			input: []byte("Hello, World!"),
		},
		{
			name:  "binary content",
			input: []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
		},
		{
			name:  "empty content",
			input: []byte{},
		},
		{
			name:  "HTML content",
			input: []byte("<html><body>Test</body></html>"),
		},
		{
			name:  "JSON content",
			input: []byte(`{"key": "value"}`),
		},
		{
			name:  "large content",
			input: bytes.Repeat([]byte("x"), 10000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewPassthroughRenderer()
			result, err := renderer.Render(context.Background(), tt.input, &enricher.EnrichmentData{})

			if err != nil {
				t.Fatalf("Render() error = %v, want nil", err)
			}

			if !bytes.Equal(result.Content, tt.input) {
				t.Errorf("Render() output = %v, want %v", result.Content, tt.input)
			}

		})
	}
}

func TestPassthroughRenderer_ContextCancellation(t *testing.T) {
	t.Parallel()

	renderer := NewPassthroughRenderer()
	testContextCancellation(t, func(ctx context.Context) error {
		_, err := renderer.Render(ctx, []byte("test content"), &enricher.EnrichmentData{})
		return err
	})
}

func TestPassthroughRenderer_ConcurrentRenders(t *testing.T) {
	renderer := NewPassthroughRenderer()
	ctx := context.Background()

	// Test that renderer can be used concurrently
	const concurrency = 100
	done := make(chan bool, concurrency)

	for i := range concurrency {
		go func(n int) {
			input := []byte("Content " + string(rune('A'+(n%26))))
			result, err := renderer.Render(ctx, input, &enricher.EnrichmentData{})
			if err != nil {
				t.Errorf("Concurrent render failed: %v", err)
			}
			if result.MimeType != "" {
				t.Errorf("Concurrent render returned mimeType %q, want empty", result.MimeType)
			}
			if !bytes.Equal(result.Content, input) {
				t.Errorf("Concurrent render modified content: got %v, want %v", result.Content, input)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range concurrency {
		<-done
	}
}

func TestPassthroughRenderer_NilInput(t *testing.T) {
	renderer := NewPassthroughRenderer()

	result, err := renderer.Render(context.Background(), nil, &enricher.EnrichmentData{})

	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}

	if result.MimeType != "" {
		t.Errorf("Render() mimeType = %q, want empty", result.MimeType)
	}

	if result.Content != nil {
		t.Errorf("Render() output = %v, want nil", result.Content)
	}
}
