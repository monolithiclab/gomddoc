package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/server"
)

// PreviewCmd holds all flags for the preview subcommand.
type PreviewCmd struct {
	Dir  string `arg:"" optional:"" default:"." help:"Markdown directory to preview."`
	Port string `name:"port" short:"p" default:":auto" help:"HTTP listen address (host:port). Defaults to auto-assigned port."`
	Open bool   `name:"open" default:"false" help:"Open the browser automatically on startup."`
}

// Run executes the preview command.
func (p *PreviewCmd) Run() error {
	port, err := resolvePort(p.Port)
	if err != nil {
		return err
	}

	cfg, err := config.NewFromServeArgs(p.Dir, port, true, "")
	if err != nil {
		return err
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := prov.Close(); closeErr != nil {
			slog.Error("Failed to close provider", slog.Any("error", closeErr))
		}
	}()

	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{
		EnableCache:      false, // preview = dev mode, no caching
		EnableNavigation: true,
		EnableMetadata:   true,
	})
	if err != nil {
		return err
	}

	httpServer := server.NewHTTPServer(server.HTTPServerConfig{
		Config:           cfg,
		Provider:         prov,
		Registry:         pipeline.Registry,
		EnricherRegistry: pipeline.EnricherRegistry,
		TemplateRenderer: pipeline.TemplateRenderer,
		MetaIndex:        pipeline.MetaIndex,
		RedirectFinder:   pipeline.RedirectFinder,
		StaticFS:         pipeline.StaticFS,
	})

	url := server.ListenURL(cfg.Server.Port)
	fmt.Printf("Preview: %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	if p.Open {
		if err := server.OpenBrowser(url); err != nil {
			slog.Debug("Failed to open browser", slog.Any("error", err))
		}
	}

	sigChan, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	g, gCtx := errgroup.WithContext(sigChan)

	g.Go(func() error {
		return httpServer.Start(gCtx)
	})

	g.Go(func() error {
		<-gCtx.Done()
		return httpServer.Shutdown(context.Background())
	})

	return g.Wait()
}
