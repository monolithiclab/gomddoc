package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/locale"
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

// LanguagePipeline holds per-language pipeline instances.
type LanguagePipeline struct {
	Default   *Pipeline            // default language pipeline (root content)
	ByLang    map[string]*Pipeline // non-default language pipelines keyed by BCP 47 code
	Bundle    *locale.Bundle       // shared locale bundle
	Languages []string             // all non-default language codes
}

// warnTagsContentCollision logs a warning when the user has authored content
// at paths that collide with the auto-generated tag pages (/tags/, /tags/{tag}).
func warnTagsContentCollision(contentRoot fs.FS, lang string) {
	for _, candidate := range []string{"tags.md", "tags"} {
		info, err := fs.Stat(contentRoot, candidate)
		if err != nil {
			continue
		}
		kind := "file"
		if info.IsDir() {
			kind = "directory"
		}
		slog.Warn("Content path collides with auto-generated tag pages",
			slog.String("path", candidate),
			slog.String("kind", kind),
			slog.String("lang", lang),
			slog.String("hint", "rename to avoid being shadowed by /tags routes"))
	}
}

// setupLanguagePipelines builds the default pipeline and per-language pipelines
// for each BCP 47 directory found in the content root. It also loads the locale
// bundle from embedded assets and merges site-level overrides from .gomddoc/locales/.
func setupLanguagePipelines(cfg *config.Config, prov provider.Provider, opts PipelineOptions) (*LanguagePipeline, error) {
	// Build default pipeline.
	defaultPipeline, err := setupPipeline(cfg, prov, opts)
	if err != nil {
		return nil, fmt.Errorf("default pipeline: %w", err)
	}

	// Load locale bundle from embedded assets.
	contentRoot, err := prov.RootFS(context.Background())
	if err != nil {
		return nil, fmt.Errorf("content root: %w", err)
	}
	assetsFS := assets.BuildFS(contentRoot, embeddedAssets)

	// Warn if user content shadows auto-generated /tags routes.
	warnTagsContentCollision(contentRoot, cfg.Site.Language)

	// Embedded files keep their `assets/` prefix from the //go:embed directive,
	// so the locale dir lives at `assets/locales` rather than `locales`. The
	// site-level overrides below (MergeFrom contentRoot) use the .gomddoc/locales
	// path directly because contentRoot has no prefix.
	bundle, err := locale.LoadBundle(cfg.Site.Language, assetsFS, "assets/locales")
	if err != nil {
		return nil, fmt.Errorf("load locale bundle: %w", err)
	}

	// Merge site-level locale overrides from .gomddoc/locales/.
	if err := bundle.MergeFrom(contentRoot, config.ConfigDirName+"/locales"); err != nil {
		slog.Warn("Failed to merge site locale overrides", slog.Any("error", err))
	}

	// Detect BCP 47 directories in the content root.
	langs := locale.DetectLanguages(contentRoot)

	lp := &LanguagePipeline{
		Default:   defaultPipeline,
		ByLang:    make(map[string]*Pipeline, len(langs)),
		Bundle:    bundle,
		Languages: langs,
	}

	// Build a pipeline for each detected language directory.
	for _, lang := range langs {
		subFS, err := fs.Sub(contentRoot, lang)
		if err != nil {
			slog.Warn("Failed to create sub-FS for language", slog.String("lang", lang), slog.Any("error", err))
			continue
		}

		langProv, err := provider.NewFilesystemProviderFromFS(subFS, cfg.Site.DefaultIndex, cfg.Site.DirIndex, cfg.Site.Exclude)
		if err != nil {
			slog.Warn("Failed to create provider for language", slog.String("lang", lang), slog.Any("error", err))
			continue
		}

		langPipeline, err := setupPipeline(cfg, langProv, opts)
		if err != nil {
			_ = langProv.Close()
			slog.Warn("Failed to build pipeline for language", slog.String("lang", lang), slog.Any("error", err))
			continue
		}

		// Warn if user content in this language directory shadows auto-generated /tags routes.
		warnTagsContentCollision(subFS, lang)

		lp.ByLang[lang] = langPipeline
		slog.Info("Built language pipeline", slog.String("lang", lang))
	}

	return lp, nil
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

	resolver := resolve.Build(contentRoot, resolve.BuildOptions{
		StripExtensions: cfg.Site.StripExtensions,
		Exclude:         cfg.Site.Exclude,
		HasRenderer: func(mimeType string) bool {
			_, _, err := registry.Get(mimeType, []negotiate.MediaType{{Type: "text", Subtype: "html", Q: 1.0}})
			return err == nil
		},
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

	// Build the metadata index before navigation so the nav generator can label
	// leaf pages from indexed titles instead of opening every file.
	if opts.EnableMetadata {
		metaIndex, err := metadata.BuildIndex(context.Background(), contentRoot, cfg.Site.Exclude)
		if err != nil {
			slog.Warn("Failed to build metadata index", slog.Any("error", err))
		}
		enricherOpts.MetaIndex = metaIndex
		p.MetaIndex = metaIndex
		p.URLRedirects = server.BuildRedirectMap(metaIndex, p.Resolver)
	}

	if opts.EnableNavigation {
		navGen := navigation.NewGenerator(contentRoot, cfg.Site.DefaultIndex, cfg.Site.Exclude, resolver)
		if p.MetaIndex != nil {
			metaIndex := p.MetaIndex
			navGen.SetTitleLookup(func(filePath string) string {
				if pg := metaIndex.ByPath("/" + filePath); pg != nil {
					return pg.Title
				}
				return ""
			})
		}
		enricherOpts.NavBuilder = navBuilderAdapter(navGen)
		enricherOpts.PrevNextBuilder = prevNextBuilderAdapter(navGen)
		p.RedirectFinder = redirectFinderAdapter(navGen)
	}

	if opts.EnableSearch && cfg.Site.Search.Index {
		searchIdx, searchErr := search.BuildIndex(context.Background(), contentRoot, p.MetaIndex, cfg.Site.Exclude)
		if searchErr != nil {
			slog.Warn("Failed to build search index", slog.Any("error", searchErr))
		} else {
			p.SearchIndex = searchIdx
			templateRenderer.Configure(template.WithSearchIndex())
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
	return func(string) string {
		return navigation.FindFirstPage(navGen.Tree())
	}
}

// navBuilderAdapter wraps a navigation.Generator into an enricher.NavBuilder.
func navBuilderAdapter(navGen *navigation.Generator) enricher.NavBuilder {
	return func(currentPath string) []enricher.NavItem {
		tree := navGen.Tree()
		if tree == nil {
			return nil
		}
		return buildNavItems(tree.Children, navigation.NormalizeRequestPath(currentPath))
	}
}

// buildNavItems converts NavNodes into NavItems, marking the leaf matching
// currentPath as Active and ancestor directories as Open. currentPath must be
// pre-normalized via navigation.NormalizeRequestPath.
func buildNavItems(nodes []*navigation.NavNode, currentPath string) []enricher.NavItem {
	items := make([]enricher.NavItem, len(nodes))
	for i, node := range nodes {
		children := buildNavItems(node.Children, currentPath)
		active := !node.IsDir && node.CompareClean(currentPath)
		open := active
		if !open {
			for _, c := range children {
				if c.Active || c.Open {
					open = true
					break
				}
			}
		}
		items[i] = enricher.NavItem{
			Title:    node.Label,
			Path:     node.Path,
			IsDir:    node.IsDir,
			Active:   active,
			Open:     open,
			Children: children,
		}
	}
	return items
}

// buildGitConfig constructs a GitProviderConfig from CLI flags.
// The storageDir, when non-empty, creates a unique subdirectory based on the
// SHA-256 hash of the source URL to isolate clones for different repositories.
func buildGitConfig(sshKeyFile, storageDir, sourceURL string) provider.GitProviderConfig {
	cfg := provider.GitProviderConfig{
		SSHKeyFile: sshKeyFile,
	}
	if storageDir != "" {
		h := sha256.Sum256([]byte(sourceURL))
		subdir := filepath.Join(storageDir, hex.EncodeToString(h[:8]))
		cfg.StorageFactory = provider.DiskStorageFactory(subdir)
	}
	return cfg
}

// ServerSetupOptions configures server creation for both serve and preview commands.
type ServerSetupOptions struct {
	Dir       string
	Port      string
	AdminPort string
	Domain    string
	DevMode   bool
	DirIndex  bool
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

	cfg, err := config.NewFromServeArgs(config.ServeArgs{
		Dir:       opts.Dir,
		Port:      port,
		AdminPort: opts.AdminPort,
		DevMode:   opts.DevMode,
		DirIndex:  opts.DirIndex,
		Pprof:     opts.Pprof,
	})
	if err != nil {
		return nil, err
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

	lp, err := setupLanguagePipelines(cfg, prov, PipelineOptions{
		EnableCache:      !cfg.Server.DevMode,
		EnableNavigation: true,
		EnableMetadata:   true,
		EnableSearch:     true,
	})
	if err != nil {
		_ = prov.Close()
		return nil, err
	}

	pipeline := lp.Default

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

	// Build per-language pipeline configs for the server.
	langPipelineConfigs := make(map[string]server.LangPipelineConfig, len(lp.ByLang))
	for lang, langPipe := range lp.ByLang {
		langPipelineConfigs[lang] = server.LangPipelineConfig{
			SearchIndex:      langPipe.SearchIndex,
			MetaIndex:        langPipe.MetaIndex,
			Provider:         langPipe.Provider,
			Resolver:         langPipe.Resolver,
			RedirectFinder:   langPipe.RedirectFinder,
			EnricherRegistry: langPipe.EnricherRegistry,
		}
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
		URLRedirects:     pipeline.URLRedirects,
		Resolver:         pipeline.Resolver,
		StaticFS:         pipeline.StaticFS,
		AuthStore:        opts.AuthStore,
		MCPHandler:       mcpServer.HTTPHandler(),
		LangPipelines:    langPipelineConfigs,
		DefaultLang:      cfg.Site.Language,
		LocaleBundle:     lp.Bundle,
		AllLanguages:     lp.Languages,
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

	if adminServer != nil {
		g.Go(func() error {
			return adminServer.Start(gCtx)
		})
	}

	return g.Wait()
}

// prevNextBuilderAdapter wraps a navigation.Generator into an enricher.PrevNextBuilder.
func prevNextBuilderAdapter(navGen *navigation.Generator) enricher.PrevNextBuilder {
	return func(currentPath string) (prev, next *enricher.PageLink) {
		prevEntry, nextEntry := navGen.PrevNext(currentPath)
		if prevEntry != nil {
			prev = &enricher.PageLink{Path: prevEntry.Path, Title: prevEntry.Label}
		}
		if nextEntry != nil {
			next = &enricher.PageLink{Path: nextEntry.Path, Title: nextEntry.Label}
		}
		return prev, next
	}
}
