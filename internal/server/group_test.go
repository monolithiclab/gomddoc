package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func headerMiddleware(key, value string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(key, value)
			next.ServeHTTP(w, r)
		})
	}
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRouteGroup_Handle(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		pattern     string
		requestPath string
		wantStatus  int
		wantHeaders map[string]string
		middleware  []func(http.Handler) http.Handler
	}{
		{
			name:        "prefix prepended with method",
			prefix:      "/api",
			pattern:     "GET /tags",
			requestPath: "/api/tags",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "prefix prepended without method",
			prefix:      "/api",
			pattern:     "/metrics",
			requestPath: "/api/metrics",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "bare path 404 when prefix set",
			prefix:      "/api",
			pattern:     "GET /tags",
			requestPath: "/tags",
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "empty prefix applies middleware only",
			prefix:      "",
			pattern:     "GET /health",
			requestPath: "/health",
			wantStatus:  http.StatusOK,
			middleware:  []func(http.Handler) http.Handler{headerMiddleware("X-Test", "applied")},
			wantHeaders: map[string]string{"X-Test": "applied"},
		},
		{
			name:        "middleware applied to handler",
			prefix:      "/api",
			pattern:     "GET /items",
			requestPath: "/api/items",
			wantStatus:  http.StatusOK,
			middleware:  []func(http.Handler) http.Handler{headerMiddleware("X-Auth", "yes")},
			wantHeaders: map[string]string{"X-Auth": "yes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			g := NewGroup(mux, tt.prefix, tt.middleware...)
			g.Handle(tt.pattern, okHandler)

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			for k, v := range tt.wantHeaders {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("header %s = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestRouteGroup_NestedGroups(t *testing.T) {
	mux := http.NewServeMux()

	root := NewGroup(mux, "/api", headerMiddleware("X-Root", "root"))
	v1 := root.Subgroup("/v1", headerMiddleware("X-V1", "v1"))
	v1.HandleFunc("GET /users", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-Root"); got != "root" {
		t.Errorf("X-Root = %q, want %q", got, "root")
	}
	if got := rec.Header().Get("X-V1"); got != "v1" {
		t.Errorf("X-V1 = %q, want %q", got, "v1")
	}
}

func TestRouteGroup_MiddlewareOrder(t *testing.T) {
	var order []string
	trackMiddleware := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	mux := http.NewServeMux()
	root := NewGroup(mux, "", trackMiddleware("A"))
	child := root.Subgroup("", trackMiddleware("B"), trackMiddleware("C"))
	child.HandleFunc("GET /test", func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	want := []string{"A", "B", "C", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i, v := range want {
		if order[i] != v {
			t.Fatalf("order[%d] = %q, want %q (full: %v)", i, order[i], v, order)
		}
	}
}
