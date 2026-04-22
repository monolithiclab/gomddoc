package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/monolithiclab/gomddoc/internal/server"
)

// PreviewCmd holds all flags for the preview subcommand.
type PreviewCmd struct {
	Dir  string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory to preview."`
	Port string `name:"port" short:"p" default:":auto" env:"GOMDDOC_SERVER_PORT" help:"HTTP listen address (host:port). Defaults to auto-assigned port."`
	Open bool   `name:"open" default:"false" env:"GOMDDOC_PREVIEW_OPEN" help:"Open the browser automatically on startup."`
}

// setup creates the provider, pipeline, and HTTP server without starting it.
func (p *PreviewCmd) setup() (*setupResult, error) {
	return setupServer(ServerSetupOptions{
		Dir:     p.Dir,
		Port:    p.Port,
		DevMode: true,
	})
}

// Run executes the preview command.
func (p *PreviewCmd) Run() error {
	result, err := p.setup()
	if err != nil {
		return err
	}
	defer result.cleanup()

	url := server.ListenURL(result.cfg.Server.Port)
	fmt.Printf("Preview: %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	if p.Open {
		if err := server.OpenBrowser(url); err != nil {
			slog.Debug("Failed to open browser", slog.Any("error", err))
		}
	}

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	return runUntilCancelled(sigCtx, result.httpServer)
}
