package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

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

func TestContentExclusion_HiddenFiles(t *testing.T) {
	t.Parallel()

	handler := ContentExclusion(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		description    string
	}{
		{".gomddoc/config.yml", "/.gomddoc/config.yml", http.StatusNotFound, "Block .gomddoc directory"},
		{".git/config", "/.git/config", http.StatusNotFound, "Block .git directory"},
		{".env", "/.env", http.StatusNotFound, "Block .env file"},
		{".htaccess", "/.htaccess", http.StatusNotFound, "Block .htaccess file"},
		{".DS_Store", "/.DS_Store", http.StatusNotFound, "Block .DS_Store file"},
		{"nested .hidden", "/docs/.hidden/file.txt", http.StatusNotFound, "Block nested hidden directory"},
		{".well-known/acme-challenge", "/.well-known/acme-challenge/token", http.StatusOK, "Allow .well-known (RFC 8615)"},
		{"normal file", "/docs/file.txt", http.StatusOK, "Allow normal files"},
		{"file with dot in name", "/file.name.txt", http.StatusOK, "Allow files with dots not at start"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("%s: Status = %d, want %d", tt.description, w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestNewMethodFilterMiddleware(t *testing.T) {
	t.Parallel()

	handler := NewMethodFilterMiddleware(http.MethodGet, http.MethodHead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{"GET allowed", http.MethodGet, http.StatusOK},
		{"HEAD allowed", http.MethodHead, http.StatusOK},
		{"POST blocked", http.MethodPost, http.StatusMethodNotAllowed},
		{"PUT blocked", http.MethodPut, http.StatusMethodNotAllowed},
		{"DELETE blocked", http.MethodDelete, http.StatusMethodNotAllowed},
		{"PATCH blocked", http.MethodPatch, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Method %s: Status = %d, want %d", tt.method, w.Code, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusMethodNotAllowed {
				allow := w.Header().Get("Allow")
				if allow != "GET, HEAD" {
					t.Errorf("Allow header = %q, want %q", allow, "GET, HEAD")
				}
			}
		})
	}
}

func TestNewBasicAuthMiddleware(t *testing.T) {
	t.Parallel()

	store, err := ParseHTPasswd(strings.NewReader("admin:" + testHash(t, "secret")))
	if err != nil {
		t.Fatalf("failed to create credential store: %v", err)
	}

	handler := NewBasicAuthMiddleware(store, "test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	tests := []struct {
		name           string
		user           string
		pass           string
		setAuth        bool
		expectedStatus int
	}{
		{"no credentials", "", "", false, http.StatusUnauthorized},
		{"valid credentials", "admin", "secret", true, http.StatusOK},
		{"wrong password", "admin", "wrong", true, http.StatusUnauthorized},
		{"wrong username", "wrong", "secret", true, http.StatusUnauthorized},
		{"wrong both", "wrong", "wrong", true, http.StatusUnauthorized},
		{"empty username", "", "secret", true, http.StatusUnauthorized},
		{"empty password", "admin", "", true, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusUnauthorized {
				auth := w.Header().Get("WWW-Authenticate")
				if auth != `Basic realm="test"` {
					t.Errorf("WWW-Authenticate = %q, want %q", auth, `Basic realm="test"`)
				}
				// Verify response body is present
				if w.Body.Len() == 0 {
					t.Error("401 response should have a body")
				}
			}
		})
	}
}

func TestNewBasicAuthMiddleware_MultipleUsers(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("admin:" + testHash(t, "adminpass") + "\nviewer:" + testHash(t, "viewerpass"))
	store, err := ParseHTPasswd(input)
	if err != nil {
		t.Fatalf("failed to parse htpasswd: %v", err)
	}

	handler := NewBasicAuthMiddleware(store, "test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		user           string
		pass           string
		expectedStatus int
	}{
		{"admin valid", "admin", "adminpass", http.StatusOK},
		{"viewer valid", "viewer", "viewerpass", http.StatusOK},
		{"admin wrong pass", "admin", "viewerpass", http.StatusUnauthorized},
		{"viewer wrong pass", "viewer", "adminpass", http.StatusUnauthorized},
		{"unknown user", "unknown", "adminpass", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/test", nil)
			req.SetBasicAuth(tt.user, tt.pass)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestContentExclusion_ResponseFormat(t *testing.T) {
	t.Parallel()

	handler := ContentExclusion(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/.gomddoc/config.yml", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}

	body := w.Body.String()
	if body != "File not found" {
		t.Errorf("Body = %q, want %q", body, "File not found")
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "text/plain; charset=utf-8")
	}
}
