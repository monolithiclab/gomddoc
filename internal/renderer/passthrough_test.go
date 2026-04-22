package renderer

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestPassthroughRenderer_SupportedMimeTypes(t *testing.T) {
	renderer := NewPassthroughRenderer()
	mimeTypes := renderer.SupportedMimeTypes()

	if len(mimeTypes) != 1 {
		t.Errorf("SupportedMimeTypes() returned %d types, want 1", len(mimeTypes))
	}

	if mimeTypes[0] != "*/*" {
		t.Errorf("SupportedMimeTypes() = %v, want [*/*]", mimeTypes)
	}
}

func TestPassthroughRenderer_Render(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		wantOutput   []byte
		wantMimeType string
	}{
		{
			name:         "text content",
			input:        []byte("Hello, World!"),
			wantOutput:   []byte("Hello, World!"),
			wantMimeType: "",
		},
		{
			name:         "binary content",
			input:        []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
			wantOutput:   []byte{0x89, 0x50, 0x4E, 0x47},
			wantMimeType: "",
		},
		{
			name:         "empty content",
			input:        []byte{},
			wantOutput:   []byte{},
			wantMimeType: "",
		},
		{
			name:         "HTML content",
			input:        []byte("<html><body>Test</body></html>"),
			wantOutput:   []byte("<html><body>Test</body></html>"),
			wantMimeType: "",
		},
		{
			name:         "JSON content",
			input:        []byte(`{"key": "value"}`),
			wantOutput:   []byte(`{"key": "value"}`),
			wantMimeType: "",
		},
		{
			name:         "large content",
			input:        bytes.Repeat([]byte("x"), 10000),
			wantOutput:   bytes.Repeat([]byte("x"), 10000),
			wantMimeType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewPassthroughRenderer()
			output, mimeType, err := renderer.Render(context.Background(), tt.input)

			if err != nil {
				t.Fatalf("Render() error = %v, want nil", err)
			}

			if mimeType != tt.wantMimeType {
				t.Errorf("Render() mimeType = %q, want %q", mimeType, tt.wantMimeType)
			}

			if !bytes.Equal(output, tt.wantOutput) {
				t.Errorf("Render() output = %v, want %v", output, tt.wantOutput)
			}
		})
	}
}

func TestPassthroughRenderer_ContextCancellation(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		wantError bool
	}{
		{
			name: "already cancelled context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx
			},
			wantError: true,
		},
		{
			name: "timeout context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(2 * time.Millisecond) // Ensure timeout
				return ctx
			},
			wantError: true,
		},
		{
			name:      "valid context",
			setupCtx:  context.Background,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewPassthroughRenderer()
			ctx := tt.setupCtx()

			_, _, err := renderer.Render(ctx, []byte("test content"))

			if tt.wantError && err == nil {
				t.Error("Render() error = nil, want error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Render() error = %v, want nil", err)
			}

			if tt.wantError && err != nil {
				// Verify it's a context error
				if err != context.Canceled && err != context.DeadlineExceeded {
					t.Errorf("Render() error = %v, want context.Canceled or context.DeadlineExceeded", err)
				}
			}
		})
	}
}

func TestPassthroughRenderer_ConcurrentRenders(t *testing.T) {
	renderer := NewPassthroughRenderer()
	ctx := context.Background()

	// Test that renderer can be used concurrently
	const concurrency = 100
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(n int) {
			input := []byte("Content " + string(rune('A'+(n%26))))
			output, mimeType, err := renderer.Render(ctx, input)
			if err != nil {
				t.Errorf("Concurrent render failed: %v", err)
			}
			if mimeType != "" {
				t.Errorf("Concurrent render returned mimeType %q, want empty", mimeType)
			}
			if !bytes.Equal(output, input) {
				t.Errorf("Concurrent render modified content: got %v, want %v", output, input)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

func TestPassthroughRenderer_NilInput(t *testing.T) {
	renderer := NewPassthroughRenderer()

	output, mimeType, err := renderer.Render(context.Background(), nil)

	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}

	if mimeType != "" {
		t.Errorf("Render() mimeType = %q, want empty", mimeType)
	}

	if output != nil {
		t.Errorf("Render() output = %v, want nil", output)
	}
}
