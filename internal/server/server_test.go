package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/template"
)

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()

	// Create test config
	siteConfig := config.NewSiteConfig(".")
	siteConfig.DefaultIndex = "README.test.md"
	siteConfig.DirIndex = false

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:      ":8080",
			DevMode:   false,
			Dir:       ".",
			GitSSHKey: "",
			HTTP: config.HTTPConfig{
				ShutdownTimeout:   1 * time.Second,
				ReadHeaderTimeout: config.DefaultReadHeaderTimeout,
				WriteTimeout:      config.DefaultWriteTimeout,
				IdleTimeout:       config.DefaultIdleTimeout,
				MaxHeaderMB:       config.DefaultMaxHeaderMB,
			},
		},
		Site: siteConfig,
	}

	// Create dependencies
	prov, err := provider.NewFilesystemProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	// Create registry with renderers
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{}))
	registry.Register(renderer.NewPassthroughRenderer())

	// Create test renderer with template
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Site.Meta.Title}}</title></head>
<body>{{.Page.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	rend := template.NewHTMLRenderer(&siteConfig, testFS)

	server := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
	})
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	if server.server.Addr != ":8080" {
		t.Errorf("Expected server address ':8080', got %q", server.server.Addr)
	}

	if server.server.ReadHeaderTimeout != config.DefaultReadHeaderTimeout {
		t.Errorf("Expected read header timeout %v, got %v", config.DefaultReadHeaderTimeout, server.server.ReadHeaderTimeout)
	}

	if server.server.WriteTimeout != config.DefaultWriteTimeout {
		t.Errorf("Expected write timeout %v, got %v", config.DefaultWriteTimeout, server.server.WriteTimeout)
	}

	if server.server.IdleTimeout != config.DefaultIdleTimeout {
		t.Errorf("Expected idle timeout %v, got %v", config.DefaultIdleTimeout, server.server.IdleTimeout)
	}

	expectedMaxHeaderBytes := config.DefaultMaxHeaderMB << 20
	if server.server.MaxHeaderBytes != expectedMaxHeaderBytes {
		t.Errorf("Expected max header bytes %v, got %v", expectedMaxHeaderBytes, server.server.MaxHeaderBytes)
	}
}

func TestHTTPServer_StartAndShutdown(t *testing.T) {
	t.Parallel()

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create test config with the available port
	siteConfig := config.NewSiteConfig(".")
	siteConfig.DefaultIndex = "README.md"
	siteConfig.DirIndex = false

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:    ":" + strconv.Itoa(port),
			DevMode: false,
			Dir:     ".",
			HTTP: config.HTTPConfig{
				ShutdownTimeout:   1 * time.Second,
				ReadHeaderTimeout: 1 * time.Second,
				WriteTimeout:      1 * time.Second,
				IdleTimeout:       1 * time.Second,
				MaxHeaderMB:       1,
			},
		},
		Site: siteConfig,
	}

	// Create dependencies
	prov, err := provider.NewFilesystemProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, nil)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{}))
	registry.Register(renderer.NewPassthroughRenderer())

	templateContent := `<!DOCTYPE html><html><body>{{.Page.Content}}</body></html>`
	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(templateContent)},
	}
	rend := template.NewHTMLRenderer(&siteConfig, testFS)

	server := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         prov,
		Registry:         registry,
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: rend,
	})

	// Start server in goroutine
	startErr := make(chan error, 1)
	go func() {
		startErr <- server.Start(context.Background())
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		t.Errorf("Shutdown failed: %v", shutdownErr)
	}

	// Check that Start returned (with nil error after graceful shutdown)
	select {
	case err := <-startErr:
		if err != nil {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after shutdown")
	}
}

func TestPprofEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pprof      bool
		wantStatus int
	}{
		{
			name:       "pprof enabled returns 200",
			pprof:      true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "pprof disabled returns 404",
			pprof:      false,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Server: config.ServerConfig{
					Port:  ":8080",
					Dir:   ".",
					Pprof: tt.pprof,
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

			prov := newMemoryProvider(fstest.MapFS{}, "README.md", false)

			srv := NewHTTPServer(HTTPServerConfig{
				Config:           cfg,
				Provider:         prov,
				Registry:         setupTestRegistry(),
				EnricherRegistry: setupTestEnricherRegistry(),
				TemplateRenderer: setupTestRenderer(),
			})

			req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GET /debug/pprof/ status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHTTPServer_AuthProtectsAllEndpoints(t *testing.T) {
	t.Parallel()

	store, err := ParseHTPasswd(strings.NewReader("admin:" + testHash(t, "secret")))
	if err != nil {
		t.Fatal(err)
	}

	testFS := fstest.MapFS{
		"README.md": {Data: []byte("---\ntitle: Test\ntags: [go]\n---\n# Test")},
	}
	idx, err := metadata.BuildIndex(context.Background(), testFS, nil)
	if err != nil {
		t.Fatal(err)
	}
	searchIdx, err := search.BuildIndex(context.Background(), testFS, idx, nil)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:  ":8080",
			Dir:   ".",
			Pprof: true,
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
	cfg.Site.Meta.Domain = "example.com"

	staticFS := fstest.MapFS{
		"style.css": {Data: []byte("body{}")},
	}

	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := NewHTTPServer(HTTPServerConfig{
		Config:           cfg,
		Provider:         newMemoryProvider(fstest.MapFS{}, "README.md", false),
		Registry:         setupTestRegistry(),
		EnricherRegistry: setupTestEnricherRegistry(),
		TemplateRenderer: setupTestRenderer(),
		MetaIndex:        idx,
		SearchIndex:      searchIdx,
		StaticFS:         staticFS,
		AuthStore:        store,
		MCPHandler:       mcpHandler,
	})

	// All these endpoints should require auth
	protectedPaths := []string{
		"/",
		"/api/search?q=test",
		"/api/tags",
		"/debug/pprof/",
		"/metrics",
		"/sitemap.xml",
		"/_mcp/",
	}

	for _, path := range protectedPaths {
		t.Run("unauthenticated"+path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("GET %s without auth: status = %d, want %d", path, w.Code, http.StatusUnauthorized)
			}
		})

		t.Run("authenticated "+path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.SetBasicAuth("admin", "secret")
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code == http.StatusUnauthorized {
				t.Errorf("GET %s with valid auth: got 401, want non-401", path)
			}
		})
	}

	// Unprotected endpoints should NOT require auth
	unprotectedPaths := []string{
		"/_assets/style.css",
		"/health/live",
		"/health/ready",
		"/robots.txt",
	}
	for _, path := range unprotectedPaths {
		t.Run("unauthenticated "+path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code == http.StatusUnauthorized {
				t.Errorf("GET %s without auth: got 401, health should not require auth", path)
			}
		})
	}
}

func TestHTTPServer_SecurityHeadersOnAllRoutes(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: ":8080",
			Dir:  ".",
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

	paths := []string{"/", "/health/live", "/health/ready", "/metrics"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if h := w.Header().Get("X-Content-Type-Options"); h != "nosniff" {
				t.Errorf("GET %s: X-Content-Type-Options = %q, want %q", path, h, "nosniff")
			}
			if h := w.Header().Get("X-Frame-Options"); h != "DENY" {
				t.Errorf("GET %s: X-Frame-Options = %q, want %q", path, h, "DENY")
			}
		})
	}
}

func TestMCPEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.Handler
		wantStatus int
	}{
		{
			name: "MCP enabled returns 200",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus: http.StatusOK,
		},
		{
			name:       "MCP disabled returns 404",
			handler:    nil,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Server: config.ServerConfig{
					Port: ":8080",
					Dir:  ".",
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
				MCPHandler:       tt.handler,
			})

			req := httptest.NewRequest(http.MethodGet, "/_mcp/", nil)
			w := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("GET /_mcp/ status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestMCPEndpoint_BodySizeLimited(t *testing.T) {
	t.Parallel()

	// MCP handler that reads the full body — should fail on oversized requests
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: ":8080",
			Dir:  ".",
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
		MCPHandler:       mcpHandler,
	})

	// Send a body larger than maxMCPBodyBytes
	oversizedBody := strings.NewReader(strings.Repeat("x", int(maxMCPBodyBytes)+1))
	req := httptest.NewRequest(http.MethodPost, "/_mcp/", oversizedBody)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("POST /_mcp/ with oversized body: status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}
