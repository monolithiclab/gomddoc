package server

import (
	"testing"
	"time"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/processor"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/template"
)

func TestNewHTTPServer(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		DefaultIndex:    "README.test.md",
		Dir:             ".",
		Port:            ":8080",
		ShutdownTimeout: 1 * time.Second,
	}

	// Create dependencies
	prov, err := provider.NewFilesystemProvider(cfg.Dir, cfg.DefaultIndex)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	proc := processor.NewMarkdownProcessor()
	rend := template.NewHTMLRenderer(nil)

	server := NewHTTPServer(cfg, prov, proc, rend)
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
