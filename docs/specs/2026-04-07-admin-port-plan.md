# Admin Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate metrics and pprof endpoints onto a dedicated admin port for production isolation.

**Architecture:** A new `AdminServer` struct in `internal/server/` wraps an `http.Server` with a minimal mux serving health, metrics, and optionally pprof via a `RouteGroup`. The main `NewHTTPServer` conditionally skips admin endpoint registration when an admin port is configured. The orchestration layer in `cmd/gomddoc/pipeline.go` manages the admin server lifecycle alongside the main server.

**Tech Stack:** Go stdlib `net/http`, existing `RouteGroup`, `prometheus/client_golang/prometheus/promhttp`

---

### Task 1: Add `AdminPort` to config

**Files:**
- Modify: `internal/config/config.go:54-61` (ServerConfig struct)

- [ ] **Step 1: Add the field**

In `internal/config/config.go`, add `AdminPort` to `ServerConfig`:

```go
type ServerConfig struct {
	Port      string     `env:"PORT"`
	AdminPort string     `env:"ADMIN_PORT"`
	DevMode   bool       `env:"DEV_MODE"`
	Dir       string     `env:"DIR"`
	GitSSHKey string     `env:"GIT_SSH_KEY"`
	Pprof     bool       `env:"PPROF"`
	HTTP      HTTPConfig `env:"HTTP"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "config: add AdminPort field to ServerConfig"
```

---

### Task 2: Add `--admin-port` flag to serve command

**Files:**
- Modify: `cmd/gomddoc/serve.go:20-28` (ServeCmd struct)
- Modify: `cmd/gomddoc/pipeline.go:160-171` (ServerSetupOptions struct)
- Modify: `cmd/gomddoc/pipeline.go:180-268` (setupServer function)

- [ ] **Step 1: Add flag to ServeCmd**

In `cmd/gomddoc/serve.go`, add the `AdminPort` field to `ServeCmd`:

```go
type ServeCmd struct {
	Dir           string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory or Git URL."`
	Port          string `name:"port" short:"p" default:":8080" env:"GOMDDOC_SERVER_PORT" help:"HTTP listen address (host:port). Use ':auto' for automatic port assignment."`
	AdminPort     string `name:"admin-port" default:"" env:"GOMDDOC_SERVER_ADMIN_PORT" help:"Listen address for admin endpoints (metrics, pprof, health)."`
	Domain        string `name:"domain" short:"d" default:"" env:"GOMDDOC_DOMAIN" help:"Override site domain for canonical URLs, sitemap, and SEO tags."`
	GitSSHKey     string `name:"git-key-file" default:"" env:"GOMDDOC_SERVER_GIT_SSH_KEY" help:"Path to SSH private key file for Git authentication."`
	GitStorageDir string `name:"git-storage-dir" default:"" env:"GOMDDOC_SERVER_GIT_STORAGE_DIR" help:"Directory for disk-based Git clone storage (default: in-memory)."`
	Pprof         bool   `name:"pprof" default:"false" env:"GOMDDOC_SERVER_PPROF" help:"Enable pprof profiling endpoints at /debug/pprof/."`
	BasicAuthFile string `name:"basic-auth-file" default:"" env:"GOMDDOC_SERVER_BASIC_AUTH_FILE" help:"Path to htpasswd file for HTTP Basic Auth (bcrypt hashes only)."`
}
```

- [ ] **Step 2: Add AdminPort to ServerSetupOptions**

In `cmd/gomddoc/pipeline.go`, add `AdminPort string` to `ServerSetupOptions`:

```go
type ServerSetupOptions struct {
	Dir       string
	Port      string
	AdminPort string
	Domain    string
	DevMode   bool
	DirIndex  bool
	GitSSHKey string
	Pprof     bool
	GitCfg    provider.GitProviderConfig
	AuthStore *server.CredentialStore
}
```

- [ ] **Step 3: Pass AdminPort through setup**

In `cmd/gomddoc/serve.go`, in the `setup()` method, pass `AdminPort` to `setupServer`:

```go
return setupServer(ServerSetupOptions{
	Dir:       s.Dir,
	Port:      s.Port,
	AdminPort: s.AdminPort,
	Domain:    s.Domain,
	GitSSHKey: s.GitSSHKey,
	Pprof:     s.Pprof,
	GitCfg:    gitCfg,
	AuthStore: authStore,
})
```

- [ ] **Step 4: Set AdminPort on config in setupServer**

In `cmd/gomddoc/pipeline.go`, in `setupServer`, after `cfg.Server.Pprof = opts.Pprof`, add:

```go
cfg.Server.AdminPort = opts.AdminPort
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/serve.go cmd/gomddoc/pipeline.go
git commit -m "serve: add --admin-port flag and plumb through setup"
```

---

### Task 3: Create `AdminServer` with tests (TDD)

**Files:**
- Create: `internal/server/admin.go`
- Create: `internal/server/admin_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/server/admin_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/server/ -run "TestAdminServer" -v`
Expected: FAIL — `NewAdminServer` and `AdminServerConfig` undefined

- [ ] **Step 3: Implement AdminServer**

Create `internal/server/admin.go`:

```go
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"

	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AdminServerConfig holds dependencies for creating an AdminServer.
type AdminServerConfig struct {
	Addr     string
	Provider provider.Provider
	Pprof    bool
}

// AdminServer serves admin endpoints (metrics, health, pprof) on a dedicated port.
type AdminServer struct {
	server *http.Server
}

// NewAdminServer creates a new admin server with health, metrics, and optionally pprof.
func NewAdminServer(cfg AdminServerConfig) *AdminServer {
	mux := http.NewServeMux()
	admin := NewGroup(mux, "")

	// Health endpoints
	healthHandler := NewHealthHandler(cfg.Provider)
	health := admin.Subgroup("/health")
	health.HandleFunc("GET /live", healthHandler.LiveHandler)
	health.HandleFunc("GET /ready", healthHandler.ReadyHandler)

	// Metrics
	admin.Handle("/metrics", promhttp.Handler())

	// Pprof
	if cfg.Pprof {
		slog.Warn("pprof profiling enabled on admin port — do not expose publicly")
		debug := admin.Subgroup("/debug/pprof")
		debug.HandleFunc("GET /", pprof.Index)
		debug.HandleFunc("GET /cmdline", pprof.Cmdline)
		debug.HandleFunc("GET /profile", pprof.Profile)
		debug.HandleFunc("GET /symbol", pprof.Symbol)
		debug.HandleFunc("GET /trace", pprof.Trace)
	}

	return &AdminServer{
		server: &http.Server{
			Addr:    cfg.Addr,
			Handler: mux,
		},
	}
}

// Handler returns the admin server's HTTP handler for testing.
func (s *AdminServer) Handler() http.Handler {
	return s.server.Handler
}

// Start starts the admin HTTP server.
func (s *AdminServer) Start(ctx context.Context) error {
	slog.Info("Admin server started", slog.String("url", ListenURL(s.server.Addr)))
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the admin server.
func (s *AdminServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/server/ -run "TestAdminServer" -v`
Expected: all 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/admin.go internal/server/admin_test.go
git commit -m "server: add AdminServer for dedicated admin port"
```

---

### Task 4: Conditionally skip admin endpoints on main mux (TDD)

**Files:**
- Modify: `internal/server/server.go:66-169` (NewHTTPServer)
- Modify: `internal/server/server.go:42-64` (HTTPServerConfig)
- Modify: `internal/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/server_test.go`:

```go
func TestHTTPServer_AdminPortSeparation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		adminPort    string
		port         string
		pprof        bool
		path         string
		wantStatus   int
	}{
		{
			name:       "admin port set — /metrics returns 404 on main",
			adminPort:  ":9090",
			port:       ":8080",
			path:       "/metrics",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "admin port set — /debug/pprof/ returns 404 on main",
			adminPort:  ":9090",
			port:       ":8080",
			pprof:      true,
			path:       "/debug/pprof/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "same port — /metrics stays on main",
			adminPort:  ":8080",
			port:       ":8080",
			path:       "/metrics",
			wantStatus: http.StatusOK,
		},
		{
			name:       "same port — /debug/pprof/ stays on main",
			adminPort:  ":8080",
			port:       ":8080",
			pprof:      true,
			path:       "/debug/pprof/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "no admin port — /metrics stays on main",
			adminPort:  "",
			port:       ":8080",
			path:       "/metrics",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Server: config.ServerConfig{
					Port:      tt.port,
					AdminPort: tt.adminPort,
					Dir:       ".",
					Pprof:     tt.pprof,
					HTTP: config.HTTPConfig{
						ShutdownTimeout:   1 * time.Second,
						ReadHeaderTimeout: config.DefaultReadHeaderTimeout,
						WriteTimeout:      config.DefaultWriteTimeout,
						IdleTimeout:       config.DefaultIdleTimeout,
						MaxHeaderMB:       config.DefaultMaxHeaderMB,
					},
				},
				Site: config.NewSiteConfig("."),
			}

			srv := NewHTTPServer(HTTPServerConfig{
				Config:           cfg,
				Provider:         newMemoryProvider(fstest.MapFS{}, "README.md", false),
				Registry:         setupTestRegistry(),
				EnricherRegistry: setupTestEnricherRegistry(),
				TemplateRenderer: setupTestRenderer(),
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GET %s status = %d, want %d", tt.path, w.Code, tt.wantStatus)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/server/ -run "TestHTTPServer_AdminPortSeparation" -v`
Expected: FAIL — metrics/pprof still registered on main mux when admin port is set

- [ ] **Step 3: Modify NewHTTPServer to conditionally register admin endpoints**

In `internal/server/server.go`, modify `NewHTTPServer`. The key change is wrapping the metrics and pprof registration in a condition. Replace the current metrics and pprof blocks:

```go
	// Admin endpoints: metrics and pprof
	// When AdminPort is set and differs from Port, these move to the admin server.
	// When AdminPort equals Port or is empty, register on main mux.
	adminOnMain := cfg.Server.AdminPort == "" || cfg.Server.AdminPort == cfg.Server.Port
	if adminOnMain {
		auth.Handle("/metrics", promhttp.Handler())
	}
```

And for pprof, change the existing block:

```go
	if cfg.Server.Pprof && adminOnMain {
		slog.Warn("pprof profiling enabled — do not use in production")
		debug := auth.Subgroup("/debug/pprof")
		debug.HandleFunc("GET /", pprof.Index)
		debug.HandleFunc("GET /cmdline", pprof.Cmdline)
		debug.HandleFunc("GET /profile", pprof.Profile)
		debug.HandleFunc("GET /symbol", pprof.Symbol)
		debug.HandleFunc("GET /trace", pprof.Trace)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/server/ -run "TestHTTPServer_AdminPortSeparation" -v`
Expected: all 5 subtests PASS

- [ ] **Step 5: Run full server test suite**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./internal/server/ -v`
Expected: all tests PASS (existing tests still work since they don't set AdminPort)

- [ ] **Step 6: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "server: skip metrics/pprof on main mux when admin port is set"
```

---

### Task 5: Wire admin server lifecycle in pipeline

**Files:**
- Modify: `cmd/gomddoc/pipeline.go:173-284` (setupResult, setupServer, runUntilCancelled)

- [ ] **Step 1: Add adminServer to setupResult**

In `cmd/gomddoc/pipeline.go`, add the field to `setupResult`:

```go
type setupResult struct {
	httpServer  *server.HTTPServer
	adminServer *server.AdminServer // nil when admin port not set or same as main port
	cfg         *config.Config
	cleanup     func()
}
```

- [ ] **Step 2: Create admin server in setupServer**

In `cmd/gomddoc/pipeline.go`, in `setupServer`, after `httpServer := server.NewHTTPServer(serverConfig)`, add admin server creation:

```go
	var adminServer *server.AdminServer
	if cfg.Server.AdminPort != "" && cfg.Server.AdminPort != cfg.Server.Port {
		adminServer = server.NewAdminServer(server.AdminServerConfig{
			Addr:     cfg.Server.AdminPort,
			Provider: prov,
			Pprof:    cfg.Server.Pprof,
		})
	}
```

Update the return to include `adminServer`:

```go
	return &setupResult{
		httpServer:  httpServer,
		adminServer: adminServer,
		cfg:         cfg,
		cleanup: func() {
			if closeErr := prov.Close(); closeErr != nil {
				slog.Error("Failed to close provider", slog.Any("error", closeErr))
			}
		},
	}, nil
```

- [ ] **Step 3: Update runUntilCancelled to manage admin server**

Change `runUntilCancelled` signature and add admin server goroutines:

```go
func runUntilCancelled(ctx context.Context, httpServer *server.HTTPServer, adminServer *server.AdminServer) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return httpServer.Start(gCtx)
	})

	g.Go(func() error {
		<-gCtx.Done()
		return httpServer.Shutdown(context.Background())
	})

	if adminServer != nil {
		g.Go(func() error {
			return adminServer.Start(gCtx)
		})

		g.Go(func() error {
			<-gCtx.Done()
			return adminServer.Shutdown(context.Background())
		})
	}

	return g.Wait()
}
```

- [ ] **Step 4: Update all callers of runUntilCancelled**

In `cmd/gomddoc/serve.go`, update the `Run` method:

```go
return runUntilCancelled(sigCtx, result.httpServer, result.adminServer)
```

Find and update any other callers (preview.go):

Run: `grep -rn "runUntilCancelled" cmd/gomddoc/`

Update each caller to pass `result.adminServer` (which will be nil for preview since it doesn't set AdminPort).

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go build ./...`
Expected: clean build

- [ ] **Step 6: Commit**

```bash
git add cmd/gomddoc/pipeline.go cmd/gomddoc/serve.go cmd/gomddoc/preview.go
git commit -m "pipeline: wire admin server lifecycle alongside main server"
```

---

### Task 6: Add startup warning

**Files:**
- Modify: `cmd/gomddoc/pipeline.go` (setupServer function)

- [ ] **Step 1: Add warning log**

In `cmd/gomddoc/pipeline.go`, in `setupServer`, after the admin server creation block (after `cfg.Server.AdminPort = opts.AdminPort`), add the warning for serve (non-dev, non-preview) mode:

```go
	if cfg.Server.AdminPort == "" && !cfg.Server.DevMode {
		slog.Warn("admin endpoints on main port — use --admin-port for production")
	}
```

This naturally won't fire for preview (which sets `DevMode: true`) or when admin port is set.

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go build ./...`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add cmd/gomddoc/pipeline.go
git commit -m "serve: warn when admin endpoints are on main port"
```

---

### Task 7: Run full CI and fix any issues

**Files:**
- Possibly: any file that needs adjustment

- [ ] **Step 1: Run make ci**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && make ci`
Expected: all checks pass (codefix, format, lint, tests)

- [ ] **Step 2: Fix any failures**

If lint or tests fail, fix the issues. Common things to watch for:
- The existing `TestHTTPServer_SecurityHeadersOnAllEndpoints` tests `/metrics` — it will still pass because it doesn't set `AdminPort`
- The existing `TestHTTPServer_AuthProtectsAllEndpoints` tests `/metrics` and `/debug/pprof/` — same, still passes
- Unused imports in `server.go` if pprof import is no longer always needed — the import is still needed because the `adminOnMain` path still uses it

- [ ] **Step 3: Commit any fixes**

```bash
git add -u
git commit -m "fix: address CI issues from admin port changes"
```

---

### Task 8: Verify preview command still works

**Files:**
- Check: `cmd/gomddoc/preview.go`

- [ ] **Step 1: Check preview passes nil admin server**

Run: `grep -n "runUntilCancelled" cmd/gomddoc/preview.go`

Verify the preview command passes `result.adminServer` (which will be nil since preview's `ServerSetupOptions` doesn't set `AdminPort`). The `runUntilCancelled` function handles nil gracefully.

- [ ] **Step 2: Run preview-related tests if any exist**

Run: `cd /Users/nicolasm/Work/Monolithic/repositories/gomddoc && go test ./cmd/gomddoc/ -v -run "Preview"`
Expected: PASS (or no matching tests)

- [ ] **Step 3: No commit needed if no changes**
