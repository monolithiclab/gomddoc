package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Wrap with security headers middleware
	secureHandler := SecurityHeaders(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	secureHandler.ServeHTTP(w, req)

	// Check that all security headers are present
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		// "Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'",
		"Referrer-Policy":    "strict-origin-when-cross-origin",
		"Permissions-Policy": "geolocation=(), microphone=(), camera=()",
	}

	for header, expected := range expectedHeaders {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("Expected %s: %q, got: %q", header, expected, got)
		}
	}

	// HSTS should NOT be set for non-HTTPS requests
	if hsts := w.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("Expected no HSTS header for HTTP request, got: %q", hsts)
	}

	// Check that the response is still successful
	if w.Code != 200 {
		t.Errorf("Expected status 200, got: %d", w.Code)
	}

	if w.Body.String() != "test response" {
		t.Errorf("Expected body 'test response', got: %q", w.Body.String())
	}
}
