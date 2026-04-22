package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// PreviewCmd holds all flags for the preview subcommand.
type PreviewCmd struct {
	Dir    string `arg:"" optional:"" default:"." help:"Markdown directory to preview."`
	Port   string `name:"port" short:"p" default:":auto" help:"HTTP listen address (host:port). Defaults to auto-assigned port."`
	NoOpen bool   `name:"no-open" default:"false" help:"Do not open the browser automatically."`
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

	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownPassthroughRenderer())
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
	)

	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		return err
	}

	metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot)
	if err != nil {
		slog.Warn("Failed to build metadata index", slog.Any("error", err))
	}

	enricherRegistry := enricher.NewDefaultEnricherRegistry()
	enricherRegistry.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{
		MetaIndex:  metaIndex,
		NavBuilder: navBuilderAdapter(navGen),
	}))

	staticFS := assets.BuildStaticFS(assetsFS, cfg.Site.Theme.Name)

	redirectFinder := redirectFinderAdapter(navGen)
	httpServer := server.NewHTTPServer(cfg, prov, registry, enricherRegistry, templateRenderer, metaIndex, redirectFinder, staticFS)

	url := server.ListenURL(cfg.Server.Port)
	fmt.Printf("Preview: %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	if !p.NoOpen {
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
