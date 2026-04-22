package server

import (
	"context"
	"net"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/template"
)

func TestNewHTTPServer(t *testing.T) {
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
	prov, err := provider.NewFilesystemProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	// Create registry with renderers
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer(""))
	registry.Register(renderer.NewPassthroughRenderer())

	// Create test renderer with template
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Site.Meta.Title}}</title></head>
<body>{{.Page.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layout.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	rend := template.NewHTMLRenderer(&siteConfig, testFS)

	server := NewHTTPServer(cfg, prov, registry, rend)
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
			Port:    ":" + string(rune(port)),
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
	// Use port string properly
	cfg.Server.Port = ":" + netPortToString(port)

	// Create dependencies
	prov, err := provider.NewFilesystemProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer prov.Close()

	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer(""))
	registry.Register(renderer.NewPassthroughRenderer())

	templateContent := `<!DOCTYPE html><html><body>{{.Page.Content}}</body></html>`
	testFS := fstest.MapFS{
		"assets/themes/default/layout.html.tmpl": {Data: []byte(templateContent)},
	}
	rend := template.NewHTMLRenderer(&siteConfig, testFS)

	server := NewHTTPServer(cfg, prov, registry, rend)

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

func netPortToString(port int) string {
	return strconv.Itoa(port)
}
