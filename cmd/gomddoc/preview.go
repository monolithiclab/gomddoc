package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/server"
)

// PreviewCmd holds all flags for the preview subcommand.
type PreviewCmd struct {
	Dir  string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory to preview."`
	Port string `name:"port" short:"p" default:":auto" env:"GOMDDOC_SERVER_PORT" help:"HTTP listen address (host:port). Defaults to auto-assigned port."`
	Open bool   `name:"open" default:"false" env:"GOMDDOC_PREVIEW_OPEN" help:"Open the browser automatically on startup."`
}

// previewSetupResult holds the assembled preview server and cleanup function.
type previewSetupResult struct {
	httpServer *server.HTTPServer
	cfg        *config.Config
	cleanup    func()
}

// setup creates the provider, pipeline, and HTTP server without starting it.
func (p *PreviewCmd) setup() (*previewSetupResult, error) {
	port, err := resolvePort(p.Port)
	if err != nil {
		return nil, err
	}

	cfg, err := config.NewFromServeArgs(p.Dir, port, true, "")
	if err != nil {
		return nil, err
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex)
	if err != nil {
		return nil, err
	}

	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{
		EnableCache:      false,
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     true,
	})
	if err != nil {
		_ = prov.Close()
		return nil, err
	}

	serverConfig := server.HTTPServerConfig{
		Config:           cfg,
		Provider:         pipeline.Provider,
		Registry:         pipeline.Registry,
		EnricherRegistry: pipeline.EnricherRegistry,
		TemplateRenderer: pipeline.TemplateRenderer,
		MetaIndex:        pipeline.MetaIndex,
		SearchIndex:      pipeline.SearchIndex,
		RedirectFinder:   pipeline.RedirectFinder,
		StaticFS:         pipeline.StaticFS,
	}

	httpServer := server.NewHTTPServer(serverConfig)

	return &previewSetupResult{
		httpServer: httpServer,
		cfg:        cfg,
		cleanup: func() {
			if closeErr := prov.Close(); closeErr != nil {
				slog.Error("Failed to close provider", slog.Any("error", closeErr))
			}
		},
	}, nil
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
