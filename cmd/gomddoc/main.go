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
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

//go:embed assets
var assets embed.FS

const (
	ExitSuccess     = 0
	ExitError       = 1
	ExitConfigError = 2
)

// infoProviderAdapter adapts provider.Provider to breadcrumb.InfoProvider
type infoProviderAdapter struct {
	p provider.Provider
}

func (pa *infoProviderAdapter) IsDir(path string) bool {
	info, err := pa.p.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func startCmd() int {
	// Load configuration (Defaults -> Env -> Flags -> File -> Env -> Validate)
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", slog.Any("error", err))
		return ExitConfigError
	}

	// Log config in dev mode
	if cfg.Server.DevMode {
		slog.Info("Development mode enabled", slog.Any("config", cfg))
	}

	// Initialize content provider (filesystem or Git based on Dir)
	var providerOpts []provider.GitProviderOption
	if cfg.Server.GitSSHKey != "" {
		providerOpts = append(providerOpts, provider.WithSSHKeyFile(cfg.Server.GitSSHKey))
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, providerOpts...)
	if err != nil {
		slog.Error("Cannot create content provider", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		// Cleanup provider on normal exit
		if err := prov.Close(); err != nil {
			slog.Error("Failed to close provider", slog.Any("error", err))
		}
	}()

	// Create renderer registry with markdown and passthrough renderers
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer())
	registry.Register(renderer.NewPassthroughRenderer())

	// Create breadcrumb generator
	breadcrumbGen := breadcrumb.NewGenerator(&infoProviderAdapter{p: prov})

	// Create template cache based on dev mode (factory pattern)

	// Create template renderer with injected dependencies (using functional options for cache)
	templateRenderer := template.NewHTMLRenderer(&cfg.Site, assets, template.WithBreadcrumbGenerator(breadcrumbGen))
	if !cfg.Server.DevMode {
		templateRenderer.Configure(template.WithCache(&template.CachedTemplateStore{}))
	}

	// Validate that default theme exists (fatal error if missing)
	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		slog.Error("Default theme missing", slog.Any("error", err))
		return ExitConfigError
	}

	// Create and configure server
	httpServer := server.NewHTTPServer(cfg, prov, registry, templateRenderer)

	// Setup graceful shutdown
	sigChan, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

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
		return ExitError
	}

	return ExitSuccess
}

func main() {
	statusCode := startCmd()
	os.Exit(statusCode)
}
