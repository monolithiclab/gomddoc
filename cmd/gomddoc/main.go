package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/processor"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template"
)

//go:embed assets
var assets embed.FS

func main() {
	// Parse configuration
	cfg := config.New()
	cfg.ParseFlags()

	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize components
	provider, err := provider.NewFilesystemProvider(cfg.Dir, cfg.DefaultIndex)
	if err != nil {
		slog.Error("Cannot create filesystem provider", slog.Any("error", err))
		os.Exit(1)
	}

	processor := processor.NewMarkdownProcessor()
	renderer := template.NewHTMLRenderer(assets)

	// Create and configure server
	httpServer := server.NewHTTPServer(cfg, provider, processor, renderer)

	// Setup graceful shutdown
	sigChan, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	g, gCtx := errgroup.WithContext(sigChan)

	// Start server in a goroutine
	g.Go(func() error {
		return httpServer.Start(gCtx)
	})

	// Handle graceful shutdown
	g.Go(func() error {
		<-gCtx.Done()
		return httpServer.Shutdown(context.Background())
	})

	// Wait for completion
	if err := g.Wait(); err != nil {
		slog.Error("Server error", slog.Any("error", err))
		sigCancel()
		os.Exit(1)
	}
	sigCancel()
}
