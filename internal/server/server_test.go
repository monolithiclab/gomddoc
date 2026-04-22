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
	cfg := &config.Config{
		Dir:             ".",
		Port:            ":8080",
		ShutdownTimeout: 1 * time.Second,
		Server: &config.ServerConfig{
			DefaultIndex: "README.test.md",
			DirIndex:     false,
		},
		Site: siteConfig,
	}

	// Create dependencies
	prov, err := provider.NewFilesystemProvider(cfg.Dir, cfg.Server.DefaultIndex, cfg.Server.DirIndex)
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

	rend := template.NewHTMLRenderer(siteConfig, testFS)

	server := NewHTTPServer(cfg, prov, registry, rend)
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	if server.server.Addr != ":8080" {
		t.Errorf("Expected server address ':8080', got %q", server.server.Addr)
	}

	if server.server.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("Expected read header timeout 5s, got %v", server.server.ReadHeaderTimeout)
	}
}
