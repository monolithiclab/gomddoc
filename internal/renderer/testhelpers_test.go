package renderer

import (
	"context"
	"testing"
	"time"
)

// testContextCancellation is a shared helper for testing context cancellation
// in renderers. It tests that renderers properly handle cancelled/timed-out contexts.
func testContextCancellation(t *testing.T, renderFunc func(context.Context) error) {
	t.Helper()

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
			t.Parallel()

			ctx := tt.setupCtx()
			err := renderFunc(ctx)

			if tt.wantError && err == nil {
				t.Error("expected error from cancelled/timeout context, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
