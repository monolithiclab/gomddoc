package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRequestID_Generated(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("Expected X-Request-ID response header to be set")
	}

	// Verify format: timestamp-counter
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		t.Errorf("Expected request ID format 'timestamp-counter', got %q", id)
	}
}

func TestRequestID_PreservesIncoming(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the incoming ID is available in context
		id := GetRequestID(r.Context())
		if id != "upstream-123" {
			t.Errorf("Expected context request ID 'upstream-123', got %q", id)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "upstream-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id != "upstream-123" {
		t.Errorf("Expected X-Request-ID 'upstream-123', got %q", id)
	}
}

func TestGetRequestID_FromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey{}, "test-id-456")
	id := GetRequestID(ctx)
	if id != "test-id-456" {
		t.Errorf("Expected 'test-id-456', got %q", id)
	}
}

func TestGetRequestID_MissingContext(t *testing.T) {
	id := GetRequestID(context.Background())
	if id != "" {
		t.Errorf("Expected empty string for missing context value, got %q", id)
	}
}

func TestRequestID_UniqueForConcurrentRequests(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numRequests = 100
	ids := make([]string, numRequests)
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := range numRequests {
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			ids[idx] = w.Header().Get("X-Request-ID")
		}(i)
	}

	wg.Wait()

	// Check all IDs are unique
	seen := make(map[string]bool, numRequests)
	for i, id := range ids {
		if id == "" {
			t.Errorf("Request %d: empty request ID", i)
			continue
		}
		if seen[id] {
			t.Errorf("Duplicate request ID: %q", id)
		}
		seen[id] = true
	}
}
