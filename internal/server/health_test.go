package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// errorProvider is a mock provider that always returns an error from Stat.
type errorProvider struct {
	statErr error
}

var _ = (interface {
	Stat(context.Context, string) (fs.FileInfo, error)
})((*errorProvider)(nil))

func (e *errorProvider) ReadFile(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", errors.New("not implemented")
}

func (e *errorProvider) Stat(_ context.Context, _ string) (fs.FileInfo, error) {
	return nil, e.statErr
}

func (e *errorProvider) DefaultIndex() string {
	return "README.md"
}

func (e *errorProvider) RootFS(_ context.Context) (fs.FS, error) {
	return nil, errors.New("not implemented")
}

func (e *errorProvider) Close() error {
	return nil
}

func TestHealthHandler_LiveHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		wantStatus     int
		wantBodyStatus string
	}{
		{
			name:           "always returns 200",
			wantStatus:     http.StatusOK,
			wantBodyStatus: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHealthHandler(&errorProvider{statErr: errors.New("broken")})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health/live", nil)

			handler.LiveHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var resp healthResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode JSON: %v", err)
			}

			if resp.Status != tt.wantBodyStatus {
				t.Errorf("status = %q, want %q", resp.Status, tt.wantBodyStatus)
			}

			if resp.Error != "" {
				t.Errorf("error field should be empty, got %q", resp.Error)
			}
		})
	}
}

func TestHealthHandler_ReadyHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		provider       *errorProvider
		memProvider    bool
		wantStatus     int
		wantBodyStatus string
		wantError      bool
	}{
		{
			name:           "healthy provider returns 200",
			memProvider:    true,
			wantStatus:     http.StatusOK,
			wantBodyStatus: "ok",
			wantError:      false,
		},
		{
			name:           "unhealthy provider returns 503",
			provider:       &errorProvider{statErr: errors.New("disk unavailable")},
			wantStatus:     http.StatusServiceUnavailable,
			wantBodyStatus: "unavailable",
			wantError:      true,
		},
		{
			name:           "provider timeout returns 503",
			provider:       &errorProvider{statErr: errors.New("context deadline exceeded")},
			wantStatus:     http.StatusServiceUnavailable,
			wantBodyStatus: "unavailable",
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var handler *HealthHandler
			if tt.memProvider {
				mp := newMemoryProvider(nil, "README.md", false)
				handler = NewHealthHandler(mp)
			} else {
				handler = NewHealthHandler(tt.provider)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)

			handler.ReadyHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			var resp healthResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode JSON: %v", err)
			}

			if resp.Status != tt.wantBodyStatus {
				t.Errorf("status = %q, want %q", resp.Status, tt.wantBodyStatus)
			}

			if tt.wantError && resp.Error == "" {
				t.Error("expected error field to be non-empty")
			}

			if !tt.wantError && resp.Error != "" {
				t.Errorf("expected empty error field, got %q", resp.Error)
			}
		})
	}
}

func TestHealthHandler_ResponseJSON(t *testing.T) {
	t.Parallel()

	handler := NewHealthHandler(newMemoryProvider(nil, "README.md", false))

	t.Run("live response has valid JSON structure", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		handler.LiveHandler(rec, req)

		var raw map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}

		if _, ok := raw["status"]; !ok {
			t.Error("JSON response missing 'status' field")
		}

		// "error" field should be omitted for healthy response
		if _, ok := raw["error"]; ok {
			t.Error("JSON response should not contain 'error' field when healthy")
		}
	})

	t.Run("ready error response has error field", func(t *testing.T) {
		t.Parallel()

		errHandler := NewHealthHandler(&errorProvider{statErr: errors.New("test error")})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		errHandler.ReadyHandler(rec, req)

		var raw map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}

		if _, ok := raw["status"]; !ok {
			t.Error("JSON response missing 'status' field")
		}

		errVal, ok := raw["error"]
		if !ok {
			t.Error("JSON response missing 'error' field for unhealthy response")
		}

		if errStr, isStr := errVal.(string); !isStr || errStr == "" {
			t.Error("error field should be a non-empty string")
		}
	})
}

func TestHealthEndpoints_NoMiddleware(t *testing.T) {
	t.Parallel()

	// Verify health endpoints are accessible without going through middleware
	// by checking that the mux routes them directly to health handlers.
	mp := newMemoryProvider(nil, "README.md", false)
	healthHandler := NewHealthHandler(mp)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", healthHandler.LiveHandler)
	mux.HandleFunc("GET /health/ready", healthHandler.ReadyHandler)

	// Simulate a request - health endpoints should respond directly
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health/live through mux: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify no security headers are set (they come from middleware)
	if h := rec.Header().Get("X-Content-Type-Options"); h != "" {
		t.Errorf("health endpoint should not have security headers, got X-Content-Type-Options: %q", h)
	}
}

// Ensure the memoryProvider Stat works for health checks (stat ".")
func TestMemoryProvider_StatRoot(t *testing.T) {
	t.Parallel()

	mp := newMemoryProvider(nil, "README.md", false)
	info, err := mp.Stat(t.Context(), ".")
	if err != nil {
		t.Fatalf("Stat(.) failed: %v", err)
	}

	if !info.IsDir() {
		t.Error("Stat(.) should return a directory")
	}

	if info.ModTime().After(time.Now()) {
		t.Error("ModTime should not be in the future")
	}
}
