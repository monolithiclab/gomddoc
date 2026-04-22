package main

import (
	"context"
	"io/fs"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/mcp"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/server"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
	"github.com/monolithiclab/gomddoc/internal/template/navigation"
)

// PipelineOptions configures which pipeline features to enable.
type PipelineOptions struct {
	EnableCache      bool // enable template caching (production mode)
	EnableNavigation bool // enable navigation tree and redirect finder
	EnableMetadata   bool // enable metadata index for tag API
	EnableSearch     bool // enable full-text search index
}

// Pipeline holds the assembled rendering pipeline components.
type Pipeline struct {
	Registry         renderer.RendererRegistry
	EnricherRegistry enricher.EnricherRegistry
	TemplateRenderer *template.HTMLRenderer
	MetaIndex        *metadata.Index
	SearchIndex      *search.Index
	RedirectFinder   server.RedirectFinder
	StaticFS         fs.FS
	Provider         provider.Provider
}

// setupPipeline assembles the shared rendering pipeline from config and provider.
func setupPipeline(cfg *config.Config, prov provider.Provider, opts PipelineOptions) (*Pipeline, error) {
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
		return nil, err
	}
	assetsFS := assets.BuildFS(contentRoot, embeddedAssets)

	templateOpts := []template.RendererOption{
		template.WithBreadcrumbGenerator(breadcrumbGen),
	}
	if opts.EnableCache {
		templateOpts = append(templateOpts, template.WithCache(&template.CachedTemplateStore{}))
	}
	templateRenderer := template.NewHTMLRenderer(&cfg.Site, assetsFS, templateOpts...)

	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		return nil, err
	}

	staticFS := assets.BuildStaticFS(assetsFS, cfg.Site.Theme.Name)

	p := &Pipeline{
		Registry:         registry,
		TemplateRenderer: templateRenderer,
		StaticFS:         staticFS,
		Provider:         provider.NewOverlayProvider(prov, staticFS),
	}

	// Enricher options — navigation is optional (build doesn't use it).
	enricherOpts := enricher.MarkdownEnricherOptions{}

	if opts.EnableNavigation {
		navGen := navigation.NewGenerator(contentRoot, cfg.Site.DefaultIndex)
		enricherOpts.NavBuilder = navBuilderAdapter(navGen)
		p.RedirectFinder = redirectFinderAdapter(navGen)
	}

	if opts.EnableMetadata {
		metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot)
		if err != nil {
			slog.Warn("Failed to build metadata index", slog.Any("error", err))
		}
		enricherOpts.MetaIndex = metaIndex
		p.MetaIndex = metaIndex
	}

	if opts.EnableSearch {
		searchIdx, searchErr := search.BuildIndex(context.Background(), contentRoot, p.MetaIndex)
		if searchErr != nil {
			slog.Warn("Failed to build search index", slog.Any("error", searchErr))
		} else {
			p.SearchIndex = searchIdx
		}
	}

	enricherRegistry := enricher.NewDefaultEnricherRegistry()
	enricherRegistry.Register(enricher.NewMarkdownEnricher(enricherOpts))
	p.EnricherRegistry = enricherRegistry

	return p, nil
}

// redirectFinderAdapter wraps a navigation.Generator into a server.RedirectFinder,
// finding the first page under a directory for redirect when no index exists.
func redirectFinderAdapter(navGen *navigation.Generator) server.RedirectFinder {
	return func(dirPath string) string {
		tree := navGen.Generate(dirPath)
		return navigation.FindFirstPage(tree)
	}
}

// navBuilderAdapter wraps a navigation.Generator into an enricher.NavBuilder,
// converting NavNode trees to enricher.NavItem slices.
func navBuilderAdapter(navGen *navigation.Generator) enricher.NavBuilder {
	return func(currentPath string) []enricher.NavItem {
		root := navGen.Generate(currentPath)
		if root == nil {
			return nil
		}
		return convertNavNodes(root.Children)
	}
}

// ServerSetupOptions configures server creation for both serve and preview commands.
type ServerSetupOptions struct {
	Dir       string
	Port      string
	DevMode   bool
	GitSSHKey string
	Pprof     bool
	GitCfg    provider.GitProviderConfig
	AuthStore *server.CredentialStore
}

// setupResult holds the assembled server, config, and cleanup function.
type setupResult struct {
	httpServer *server.HTTPServer
	cfg        *config.Config
	cleanup    func()
}

// setupServer creates the provider, pipeline, and HTTP server from options.
// Shared by serve and preview commands.
func setupServer(opts ServerSetupOptions) (*setupResult, error) {
	port, err := resolvePort(opts.Port)
	if err != nil {
		return nil, err
	}

	cfg, err := config.NewFromServeArgs(opts.Dir, port, opts.DevMode, opts.GitSSHKey)
	if err != nil {
		return nil, err
	}

	cfg.Server.Pprof = opts.Pprof

	if cfg.Server.DevMode {
		slog.Info("Development mode enabled",
			slog.String("dir", cfg.Server.Dir),
			slog.String("port", cfg.Server.Port),
			slog.String("theme", cfg.Site.Theme.Name),
		)
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, opts.GitCfg)
	if err != nil {
		return nil, err
	}

	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{
		EnableCache:      !cfg.Server.DevMode,
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     true,
	})
	if err != nil {
		_ = prov.Close()
		return nil, err
	}

	mcpServer := mcp.NewServer(mcp.ServerDeps{
		Provider:     prov,
		MetaIndex:    pipeline.MetaIndex,
		SearchIndex:  pipeline.SearchIndex,
		DefaultIndex: cfg.Site.DefaultIndex,
		SiteName:     cfg.Site.Meta.Title,
		Version:      version,
	})

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
		AuthStore:        opts.AuthStore,
		MCPHandler:       mcpServer.HTTPHandler(),
	}

	httpServer := server.NewHTTPServer(serverConfig)

	return &setupResult{
		httpServer: httpServer,
		cfg:        cfg,
		cleanup: func() {
			if closeErr := prov.Close(); closeErr != nil {
				slog.Error("Failed to close provider", slog.Any("error", closeErr))
			}
		},
	}, nil
}

// runUntilCancelled starts the HTTP server and blocks until the context is cancelled,
// then performs a graceful shutdown. Used by serve and preview commands.
func runUntilCancelled(ctx context.Context, httpServer *server.HTTPServer) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return httpServer.Start(gCtx)
	})

	g.Go(func() error {
		<-gCtx.Done()
		return httpServer.Shutdown(context.Background())
	})

	return g.Wait()
}

// convertNavNodes converts navigation.NavNode children to enricher.NavItem slices.
func convertNavNodes(nodes []*navigation.NavNode) []enricher.NavItem {
	items := make([]enricher.NavItem, len(nodes))
	for i, node := range nodes {
		items[i] = enricher.NavItem{
			Title:    node.Label,
			Path:     node.Path,
			IsDir:    node.IsDir,
			Active:   node.IsActive,
			Open:     node.IsOpen,
			Children: convertNavNodes(node.Children),
		}
	}
	return items
}
