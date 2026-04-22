package server

import (
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
	registry.Register(renderer.NewMarkdownRenderer())
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
