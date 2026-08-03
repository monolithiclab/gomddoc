package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/monolithiclab/gomddoc/internal/config"
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

func TestAdminServer_StartAndShutdown(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":0",
		Provider: prov,
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Cancel context to trigger graceful shutdown.
	cancel()

	if err := <-errCh; err != nil {
		t.Errorf("Start() returned error: %v", err)
	}
}

func TestAdminServer_Shutdown(t *testing.T) {
	t.Parallel()

	prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)
	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":0",
		Provider: prov,
	})

	// Shutdown without Start should succeed (server not listening).
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown() error = %v", err)
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

// TestAdminServer_PprofAuth pins the admin port to the same rule the main port
// follows: /debug/pprof/* sits behind the credential store when one exists,
// while metrics and health stay open for scrapers that carry no credentials.
// Unauthenticated, /debug/pprof/cmdline returns the process command line —
// including the --basic-auth-file path.
func TestAdminServer_PprofAuth(t *testing.T) {
	t.Parallel()

	srv := NewAdminServer(AdminServerConfig{
		Addr:      ":9090",
		Provider:  newMemoryProvider(fstest.MapFS{}, "README.md", false),
		Pprof:     true,
		AuthStore: newTestAuthStore(t),
	})

	tests := []struct {
		name string
		path string
		auth bool
		want int
	}{
		{"pprof anonymous", "/debug/pprof/cmdline", false, http.StatusUnauthorized},
		{"pprof authenticated", "/debug/pprof/cmdline", true, http.StatusOK},
		{"metrics stay open", "/metrics", false, http.StatusOK},
		{"health stays open", "/health/live", false, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.auth {
				req.SetBasicAuth("admin", testPassword)
			}
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("GET %s status = %d, want %d", tt.path, w.Code, tt.want)
			}
		})
	}
}

// TestAdminServer_Timeouts — previously only ReadHeaderTimeout was set, so a
// slow reader held a connection indefinitely and keep-alives were never reaped.
func TestAdminServer_Timeouts(t *testing.T) {
	t.Parallel()

	srv := NewAdminServer(AdminServerConfig{
		Addr:     ":9090",
		Provider: newMemoryProvider(fstest.MapFS{}, "README.md", false),
		HTTP: config.HTTPConfig{
			ReadHeaderTimeout: 3 * time.Second,
			WriteTimeout:      7 * time.Second,
			IdleTimeout:       11 * time.Second,
			MaxHeaderMB:       2,
		},
	})

	if got := srv.server.ReadHeaderTimeout; got != 3*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 3s", got)
	}
	if got := srv.server.WriteTimeout; got != 7*time.Second {
		t.Errorf("WriteTimeout = %v, want 7s", got)
	}
	if got := srv.server.IdleTimeout; got != 11*time.Second {
		t.Errorf("IdleTimeout = %v, want 11s", got)
	}
	if got := srv.server.MaxHeaderBytes; got != 2<<20 {
		t.Errorf("MaxHeaderBytes = %d, want %d", got, 2<<20)
	}
}
