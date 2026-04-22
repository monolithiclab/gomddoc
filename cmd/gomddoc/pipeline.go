package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/mcp"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/resolve"
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
	URLRedirects     server.URLRedirectMap
	Resolver         *resolve.PathResolver
	StaticFS         fs.FS
	Provider         provider.Provider
}

// setupPipeline assembles the shared rendering pipeline from config and provider.
func setupPipeline(cfg *config.Config, prov provider.Provider, opts PipelineOptions) (*Pipeline, error) {
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownPassthroughRenderer())
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{
		HighlightTheme: cfg.Site.Highlighting.Theme,
		Features:       cfg.Site.Theme.Features,
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

	resolver := resolve.Build(contentRoot, cfg.Site.StripExtensions, func(mimeType string) bool {
		_, _, err := registry.Get(mimeType, []negotiate.MediaType{{Type: "text", Subtype: "html", Q: 1.0}})
		return err == nil
	})

	templateRenderer.Configure(template.WithResolver(resolver))

	p := &Pipeline{
		Registry:         registry,
		TemplateRenderer: templateRenderer,
		Resolver:         resolver,
		StaticFS:         staticFS,
		Provider:         provider.NewOverlayProvider(prov, staticFS),
	}

	// Enricher options — navigation is optional (build doesn't use it).
	enricherOpts := enricher.MarkdownEnricherOptions{}

	if opts.EnableNavigation {
		navGen := navigation.NewGenerator(contentRoot, cfg.Site.DefaultIndex, cfg.Site.Exclude, resolver)
		enricherOpts.NavBuilder = navBuilderAdapter(navGen)
		enricherOpts.PrevNextBuilder = prevNextBuilderAdapter(navGen)
		p.RedirectFinder = redirectFinderAdapter(navGen)
	}

	if opts.EnableMetadata {
		metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot, cfg.Site.Exclude)
		if err != nil {
			slog.Warn("Failed to build metadata index", slog.Any("error", err))
		}
		enricherOpts.MetaIndex = metaIndex
		p.MetaIndex = metaIndex
		p.URLRedirects = server.BuildRedirectMap(metaIndex, p.Resolver)
	}

	if opts.EnableSearch && cfg.Site.Search.Index {
		searchIdx, searchErr := search.BuildIndex(context.Background(), contentRoot, p.MetaIndex, cfg.Site.Exclude)
		if searchErr != nil {
			slog.Warn("Failed to build search index", slog.Any("error", searchErr))
		} else {
			p.SearchIndex = searchIdx
			if cfg.Site.Theme.Features == nil {
				cfg.Site.Theme.Features = make(map[string]bool)
			}
			cfg.Site.Theme.Features["search"] = true
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
	AdminPort string
	Domain    string
	DevMode   bool
	DirIndex  bool
	GitSSHKey string
	Pprof     bool
	GitCfg    provider.GitProviderConfig
	AuthStore *server.CredentialStore
}

// setupResult holds the assembled server, config, and cleanup function.
type setupResult struct {
	httpServer  *server.HTTPServer
	adminServer *server.AdminServer // nil when admin port not set or same as main port
	cfg         *config.Config
	cleanup     func()
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
	cfg.Server.AdminPort = opts.AdminPort

	if opts.DirIndex {
		cfg.Site.DirIndex = true
	}

	if opts.Domain != "" {
		cfg.Site.Meta.Domain = opts.Domain
		if err := cfg.Site.Validate(); err != nil {
			return nil, fmt.Errorf("domain flag: %w", err)
		}
	}

	if cfg.Server.DevMode {
		slog.Info("Development mode enabled",
			slog.String("dir", cfg.Server.Dir),
			slog.String("port", cfg.Server.Port),
			slog.String("theme", cfg.Site.Theme.Name),
		)
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, cfg.Site.Exclude, opts.GitCfg)
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
		Provider:        prov,
		MetaIndex:       pipeline.MetaIndex,
		SearchIndex:     pipeline.SearchIndex,
		DefaultIndex:    cfg.Site.DefaultIndex,
		ExcludePatterns: cfg.Site.Exclude,
		Resolver:        pipeline.Resolver,
		SiteName:        cfg.Site.Meta.Title,
		Version:         version,
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
		URLRedirects:     pipeline.URLRedirects,
		Resolver:         pipeline.Resolver,
		StaticFS:         pipeline.StaticFS,
		AuthStore:        opts.AuthStore,
		MCPHandler:       mcpServer.HTTPHandler(),
	}

	httpServer := server.NewHTTPServer(serverConfig)

	var adminServer *server.AdminServer
	if cfg.Server.AdminPort != "" && cfg.Server.AdminPort != cfg.Server.Port {
		adminServer = server.NewAdminServer(server.AdminServerConfig{
			Addr:     cfg.Server.AdminPort,
			Provider: prov,
			Pprof:    cfg.Server.Pprof,
		})
	}

	if cfg.Server.AdminPort == "" && !cfg.Server.DevMode {
		slog.Warn("admin endpoints on main port — use --admin-port for production")
	}

	return &setupResult{
		httpServer:  httpServer,
		adminServer: adminServer,
		cfg:         cfg,
		cleanup: func() {
			if closeErr := prov.Close(); closeErr != nil {
				slog.Error("Failed to close provider", slog.Any("error", closeErr))
			}
		},
	}, nil
}

// runUntilCancelled starts the HTTP server (and optional admin server) and blocks
// until the context is cancelled, then performs a graceful shutdown.
func runUntilCancelled(ctx context.Context, httpServer *server.HTTPServer, adminServer *server.AdminServer) error {
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return httpServer.Start(gCtx)
	})

	g.Go(func() error {
		<-gCtx.Done()
		return httpServer.Shutdown(context.Background())
	})

	if adminServer != nil {
		g.Go(func() error {
			return adminServer.Start(gCtx)
		})

		g.Go(func() error {
			<-gCtx.Done()
			return adminServer.Shutdown(context.Background())
		})
	}

	return g.Wait()
}

// prevNextBuilderAdapter wraps a navigation.Generator into an enricher.PrevNextBuilder,
// computing previous/next page links from the navigation tree.
func prevNextBuilderAdapter(navGen *navigation.Generator) enricher.PrevNextBuilder {
	return func(currentPath string) (prev, next *enricher.PageLink) {
		root := navGen.Generate(currentPath)
		if root == nil {
			return nil, nil
		}
		prevPath, nextPath := navigation.FindPrevNext(root, currentPath)
		if prevPath != "" {
			prev = &enricher.PageLink{
				Path:  prevPath,
				Title: findNodeLabel(root, prevPath),
			}
		}
		if nextPath != "" {
			next = &enricher.PageLink{
				Path:  nextPath,
				Title: findNodeLabel(root, nextPath),
			}
		}
		return prev, next
	}
}

// findNodeLabel searches the navigation tree for a node matching path
// and returns its label. Returns "" if not found.
func findNodeLabel(node *navigation.NavNode, path string) string {
	if !node.IsDir && node.Path == path {
		return node.Label
	}
	for _, child := range node.Children {
		if label := findNodeLabel(child, path); label != "" {
			return label
		}
	}
	return ""
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
