package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// ServeCmd holds all flags for the serve subcommand.
type ServeCmd struct {
	Dir           string `name:"dir" short:"d" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory or Git URL."`
	Port          string `name:"port" short:"p" default:":8080" env:"GOMDDOC_SERVER_PORT" help:"HTTP listen address (host:port). Use ':auto' for automatic port assignment."`
	DevMode       bool   `name:"dev" default:"false" env:"GOMDDOC_SERVER_DEV_MODE" help:"Enable development mode (no caching, verbose logging)."`
	GitSSHKey     string `name:"git-key-file" default:"" env:"GOMDDOC_SERVER_GIT_SSH_KEY" help:"Path to SSH private key file for Git authentication."`
	GitStorageDir string `name:"git-storage-dir" default:"" env:"GOMDDOC_SERVER_GIT_STORAGE_DIR" help:"Directory for disk-based Git clone storage (default: in-memory)."`
}

// resolvePort resolves the port, handling auto-port assignment.
// Returns the resolved port string (e.g., ":8081").
func resolvePort(port string) (string, error) {
	if !server.IsAutoPort(port) {
		return port, nil
	}
	resolved, err := server.FindAvailablePort(config.DefaultAutoPortStart)
	if err != nil {
		return "", fmt.Errorf("auto-port: %w", err)
	}
	slog.Info("Auto-assigned port", slog.String("port", resolved))
	return resolved, nil
}

// Run executes the serve command.
func (s *ServeCmd) Run() error {
	port, err := resolvePort(s.Port)
	if err != nil {
		return err
	}

	cfg, err := config.NewFromServeArgs(s.Dir, port, s.DevMode, s.GitSSHKey)
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
	if s.GitStorageDir != "" {
		// Hash the dir/URL to create a unique subdirectory per provider,
		// avoiding filesystem path issues with special characters.
		h := sha256.Sum256([]byte(s.Dir))
		subdir := filepath.Join(s.GitStorageDir, hex.EncodeToString(h[:8]))
		providerOpts = append(providerOpts, provider.WithStorageFactory(provider.DiskStorageFactory(subdir)))
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

	breadcrumbGen := breadcrumb.NewGenerator(func(path string) bool {
		info, err := prov.Stat(context.Background(), path)
		return err == nil && info.IsDir()
	})

	contentRoot, err := prov.RootFS(context.Background())
	if err != nil {
		return err
	}
	assetsFS := assets.BuildFS(contentRoot, embeddedAssets)

	navGen := navigation.NewGenerator(contentRoot, cfg.Site.DefaultIndex)

	templateRenderer := template.NewHTMLRenderer(&cfg.Site, assetsFS,
		template.WithBreadcrumbGenerator(breadcrumbGen),
		template.WithNavigationGenerator(navGen),
	)
	if !cfg.Server.DevMode {
		templateRenderer.Configure(template.WithCache(&template.CachedTemplateStore{}))
	}

	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		return err
	}

	metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot)
	if err != nil {
		slog.Warn("Failed to build metadata index", slog.Any("error", err))
	}

	httpServer := server.NewHTTPServer(cfg, prov, registry, templateRenderer, metaIndex, navGen)

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
