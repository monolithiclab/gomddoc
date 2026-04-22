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
)

//go:embed assets
var assets embed.FS

const (
	ExitSuccess     = 0
	ExitError       = 1
	ExitConfigError = 2
)

func startCmd() int {
	// 1. Create application config with defaults (includes temporary SiteConfig)
	cfg := config.New()

	// 2. Apply environment variable overrides to Config (before CLI flags)
	cfg.ApplyEnvOverrides()

	// 3. Parse CLI flags (affects Config, CLI flags take precedence over env vars)
	cfg.ParseFlags()

	// 4. Initialize SiteConfig with final Dir value (for correct default title)
	// This ensures the title defaults to the basename of the actual directory being served
	// All config initialization must complete before any goroutines start
	siteConfig := config.NewSiteConfig(cfg.Dir)

	// 5. Load site config from .gomddoc/config.yml (affects SiteConfig only)
	if err := siteConfig.LoadFromFile(cfg.Dir); err != nil {
		slog.Error("Cannot load site config", slog.Any("error", err))
		os.Exit(1)
	}

	// 6. Apply environment variable overrides to SiteConfig (after YAML)
	siteConfig.ApplyEnvOverrides()

	// 7. Validate site config (with auto-fixes)
	if err := siteConfig.Validate(); err != nil {
		slog.Error("Invalid site configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// 8. Assign finalized SiteConfig to Config (single assignment, no mutation)
	cfg.Site = siteConfig

	// 9. Validate application config
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize content provider (filesystem or Git based on cfg.Dir)
	var providerOpts []provider.GitProviderOption
	if cfg.GitSSHKeyFile != "" {
		providerOpts = append(providerOpts, provider.WithSSHKeyFile(cfg.GitSSHKeyFile))
	}

	prov, err := provider.NewProvider(cfg.Dir, cfg.Server.DefaultIndex, cfg.Server.DirIndex, providerOpts...)
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

	// Create template cache based on dev mode (factory pattern)

	// Create template renderer with injected dependencies (using functional options for cache)
	templateRenderer := template.NewHTMLRenderer(cfg.Site, assets)
	if !cfg.DevMode {
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
