package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestAdminServer_Metrics(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":9090",
		Provider: prov,
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /metrics status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAdminServer_Health(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":9090",
		Provider: prov,
	})

	paths := []string{"/health/live", "/health/ready"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET %s status = %d, want %d", path, w.Code, http.StatusOK)
			}
		})
	}
}

func TestAdminServer_PprofEnabled(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":9090",
		Provider: prov,
		Pprof:    true,
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /debug/pprof/ status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAdminServer_PprofDisabled(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":9090",
		Provider: prov,
		Pprof:    false,
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /debug/pprof/ status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAdminServer_NoContentRoutes(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{
		"README.md": {Data: []byte("# Hello")},
	}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":9090",
		Provider: prov,
	})

	paths := []string{"/", "/api/tags", "/_mcp/"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				t.Errorf("GET %s on admin server: got 200, want non-200 (should not serve content)", path)
			}
		})
	}
}
