package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

// ServeCmd holds all flags for the serve subcommand.
type ServeCmd struct {
	Dir       string `name:"dir" short:"d" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory or Git URL."`
	Port      string `name:"port" short:"p" default:":8080" env:"GOMDDOC_SERVER_PORT" help:"HTTP listen address (host:port)."`
	DevMode   bool   `name:"dev" default:"false" env:"GOMDDOC_SERVER_DEV_MODE" help:"Enable development mode (no caching, verbose logging)."`
	GitSSHKey string `name:"git-key-file" default:"" env:"GOMDDOC_SERVER_GIT_SSH_KEY" help:"Path to SSH private key file for Git authentication."`
}

// Run executes the serve command.
func (s *ServeCmd) Run() error {
	cfg, err := config.NewFromServeArgs(s.Dir, s.Port, s.DevMode, s.GitSSHKey)
	if err != nil {
		return err
	}

	if cfg.Server.DevMode {
		slog.Info("Development mode enabled", slog.Any("config", cfg))
	}

	var providerOpts []provider.GitProviderOption
	if cfg.Server.GitSSHKey != "" {
		providerOpts = append(providerOpts, provider.WithSSHKeyFile(cfg.Server.GitSSHKey))
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, providerOpts...)
	if err != nil {
		return err
	}
	defer func() {
		if err := prov.Close(); err != nil {
			slog.Error("Failed to close provider", slog.Any("error", err))
		}
	}()

	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{
		HighlightTheme: cfg.Site.Highlighting.Theme,
		ColorChips:     cfg.Site.ColorChips,
	}))
	registry.Register(renderer.NewPassthroughRenderer())

	breadcrumbGen := breadcrumb.NewGenerator(&infoProviderAdapter{p: prov})

	contentRoot, err := prov.RootFS()
	if err != nil {
		return err
	}
	assetsFS := assets.BuildFS(contentRoot, embeddedAssets)

	templateRenderer := template.NewHTMLRenderer(&cfg.Site, assetsFS, template.WithBreadcrumbGenerator(breadcrumbGen))
	if !cfg.Server.DevMode {
		templateRenderer.Configure(template.WithCache(&template.CachedTemplateStore{}))
	}

	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		return err
	}

	httpServer := server.NewHTTPServer(cfg, prov, registry, templateRenderer)

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
