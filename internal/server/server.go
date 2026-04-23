package server

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/search"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// maxMCPBodyBytes is the maximum request body size for MCP endpoints (1 MB).
const maxMCPBodyBytes = 1 << 20

// maxBodySize wraps an HTTP handler to limit the request body size.
func maxBodySize(next http.Handler, n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}

// Server defines the interface for HTTP servers
type Server interface {
	// Start starts the HTTP server
	Start(ctx context.Context) error
	// Shutdown gracefully shuts down the server
	Shutdown(ctx context.Context) error
}

// HTTPServer implements Server for HTTP/HTTPS serving
type HTTPServer struct {
	server  *http.Server
	config  *config.Config
	handler *Handler
}

// LangPipelineConfig holds per-language pipeline dependencies.
type LangPipelineConfig struct {
	SearchIndex *search.Index
	MetaIndex   *metadata.Index
	Provider    provider.Provider
}

// HTTPServerConfig holds all dependencies for creating an HTTPServer.
type HTTPServerConfig struct {
	Config           *config.Config
	Provider         provider.Provider
	Registry         renderer.RendererRegistry
	EnricherRegistry enricher.EnricherRegistry
	TemplateRenderer template.Renderer
	MetaIndex        *metadata.Index       // nil disables metadata API
	SearchIndex      *search.Index         // nil disables search API
	RedirectFinder   RedirectFinder        // nil disables redirect lookup
	URLRedirects     URLRedirectMap        // nil disables URL redirects
	StaticFS         fs.FS                 // nil disables static asset serving
	AuthStore        *CredentialStore      // nil disables basic auth
	MCPHandler       http.Handler          // nil disables MCP endpoint at /_mcp/
	Resolver         *resolve.PathResolver // nil disables extension stripping

	// Multi-language support
	LangPipelines map[string]LangPipelineConfig // per-language pipelines keyed by BCP 47 code
	DefaultLang   string                        // BCP 47 default language code
	LocaleBundle  *locale.Bundle                // shared locale bundle
	AllLanguages  []string                      // all non-default language codes
}

// NewHTTPServer creates a new HTTP server with the given dependencies.
func NewHTTPServer(opts HTTPServerConfig) *HTTPServer {
	cfg := opts.Config

	mux := http.NewServeMux()

	// Health endpoints are unauthenticated (load balancer probes)
	healthHandler := NewHealthHandler(opts.Provider)
	healthGroup := NewGroup(mux, "/health")
	healthGroup.HandleFunc("GET /live", healthHandler.LiveHandler)
	healthGroup.HandleFunc("GET /ready", healthHandler.ReadyHandler)

	// Robots handler is unauthenticated
	robotsHandler := NewRobotsHandler(cfg.Site.Meta.Domain)
	mux.Handle("GET /robots.txt", robotsHandler)

	// Assets handler is unauthenticated
	if opts.StaticFS != nil {
		assetsHandler := NewAssetsHandler(opts.StaticFS)
		mux.Handle("GET /_assets/", http.StripPrefix("/_assets/", assetsHandler))
	}

	// All other endpoints require auth when configured
	auth := NewGroup(mux, "")
	if opts.AuthStore != nil {
		auth = NewGroup(mux, "", NewBasicAuthMiddleware(opts.AuthStore, "gomddoc"))
	}

	// Determine if admin endpoints should be on main mux or separate admin server
	adminOnMain := cfg.Server.AdminPort == "" || cfg.Server.AdminPort == cfg.Server.Port

	if adminOnMain {
		auth.Handle("/metrics", promhttp.Handler())
	}

	// API sub-group
	api := auth.Subgroup("/api")
	if opts.MetaIndex != nil {
		metaHandler := NewMetadataHandler(opts.MetaIndex)
		api.HandleFunc("GET /tags", metaHandler.TagsHandler)
		api.HandleFunc("GET /tags/{tag}", metaHandler.TagPagesHandler)
	}
	if len(opts.LangPipelines) > 0 {
		searchIndexes := make(map[string]*search.Index)
		if opts.SearchIndex != nil {
			searchIndexes[opts.DefaultLang] = opts.SearchIndex
		}
		for lang, lp := range opts.LangPipelines {
			if lp.SearchIndex != nil {
				searchIndexes[lang] = lp.SearchIndex
			}
		}
		searchHandler := NewMultiLangSearchHandler(searchIndexes, opts.DefaultLang)
		api.HandleFunc("GET /search", searchHandler.SearchEndpoint)
	} else if opts.SearchIndex != nil {
		searchHandler := NewSearchHandler(opts.SearchIndex)
		api.HandleFunc("GET /search", searchHandler.SearchEndpoint)
	}

	if opts.MCPHandler != nil {
		auth.Handle("/_mcp/", http.StripPrefix("/_mcp", maxBodySize(opts.MCPHandler, maxMCPBodyBytes)))
	}

	if opts.MetaIndex != nil && cfg.Site.Meta.Domain != "" {
		sitemapHandler := NewSitemapHandler(opts.MetaIndex, cfg.Site.Meta.Domain, cfg.Site.DefaultIndex, opts.Provider, opts.Resolver, "")
		auth.Handle("GET /sitemap.xml", sitemapHandler)

		feedHandler := NewFeedHandler(opts.MetaIndex, cfg.Site.Meta.Domain, cfg.Site.DefaultIndex, opts.Provider, cfg.Site.Meta.Title, opts.Resolver, "")
		auth.Handle("GET /feed.xml", feedHandler)
	}

	// Tag routes for default language
	if opts.MetaIndex != nil && opts.LocaleBundle != nil {
		defaultTagTFunc := opts.LocaleBundle.TFunc(opts.DefaultLang)
		auth.Handle("GET /tags/{tag}", NewTagPageHandler(opts.MetaIndex, opts.TemplateRenderer, defaultTagTFunc, ""))
		auth.Handle("GET /tags/", NewTagsIndexHandler(opts.MetaIndex, opts.TemplateRenderer, defaultTagTFunc, ""))
	}

	// Per-language sitemap and feed routes
	for lang, lp := range opts.LangPipelines {
		prefix := "/" + lang
		if lp.MetaIndex != nil && cfg.Site.Meta.Domain != "" {
			langSitemapHandler := NewSitemapHandler(lp.MetaIndex, cfg.Site.Meta.Domain, cfg.Site.DefaultIndex, lp.Provider, opts.Resolver, prefix)
			auth.Handle("GET "+prefix+"/sitemap.xml", langSitemapHandler)

			langFeedHandler := NewFeedHandler(lp.MetaIndex, cfg.Site.Meta.Domain, cfg.Site.DefaultIndex, lp.Provider, cfg.Site.Meta.Title, opts.Resolver, prefix)
			auth.Handle("GET "+prefix+"/feed.xml", langFeedHandler)
		}

		// Tag routes per language
		if lp.MetaIndex != nil && opts.LocaleBundle != nil {
			langTagTFunc := opts.LocaleBundle.TFunc(lang)
			auth.Handle("GET "+prefix+"/tags/{tag}", NewTagPageHandler(lp.MetaIndex, opts.TemplateRenderer, langTagTFunc, lang))
			auth.Handle("GET "+prefix+"/tags/", NewTagsIndexHandler(lp.MetaIndex, opts.TemplateRenderer, langTagTFunc, lang))
		}
	}

	if cfg.Server.Pprof && adminOnMain {
		slog.Warn("pprof profiling enabled — do not use in production")
		debug := auth.Subgroup("/debug/pprof")
		debug.HandleFunc("GET /", pprof.Index)
		debug.HandleFunc("GET /cmdline", pprof.Cmdline)
		debug.HandleFunc("GET /profile", pprof.Profile)
		debug.HandleFunc("GET /symbol", pprof.Symbol)
		debug.HandleFunc("GET /trace", pprof.Trace)
	}

	// Build language info for the language switcher (only when multiple languages exist)
	var allLanguageInfos []template.LanguageInfo
	if len(opts.AllLanguages) > 0 && opts.LocaleBundle != nil {
		allLanguageInfos = template.BuildLanguageInfos(opts.LocaleBundle, opts.DefaultLang, opts.AllLanguages)
	}

	// Per-language content handlers (must be registered before the default catch-all)
	for lang, lp := range opts.LangPipelines {
		langTFunc := opts.LocaleBundle.TFunc(lang)
		langInfos := template.WithActiveLang(allLanguageInfos, lang)

		langHandler := NewHandler(HandlerConfig{
			Provider:         lp.Provider,
			Registry:         opts.Registry,
			EnricherRegistry: opts.EnricherRegistry,
			TemplateRenderer: opts.TemplateRenderer,
			SiteConfig:       &cfg.Site,
			RedirectFinder:   opts.RedirectFinder,
			Resolver:         opts.Resolver,
			Lang:             lang,
			TFunc:            langTFunc,
			Languages:        langInfos,
		})

		langContent := auth.Subgroup("/"+lang,
			Compression,
			NewMethodFilterMiddleware(http.MethodGet, http.MethodHead),
			ContentExclusion(cfg.Site.Exclude),
			ExtensionRedirect(opts.Resolver, cfg.Site.StripExtensions),
			Metrics,
		)
		langContent.HandleFunc("/", langHandler.ServeContent)
	}

	var defaultTFunc func(string) string
	if opts.LocaleBundle != nil {
		defaultTFunc = opts.LocaleBundle.TFunc(opts.DefaultLang)
	}
	defaultLangInfos := template.WithActiveLang(allLanguageInfos, opts.DefaultLang)

	handler := NewHandler(HandlerConfig{
		Provider:         opts.Provider,
		Registry:         opts.Registry,
		EnricherRegistry: opts.EnricherRegistry,
		TemplateRenderer: opts.TemplateRenderer,
		SiteConfig:       &cfg.Site,
		RedirectFinder:   opts.RedirectFinder,
		URLRedirects:     opts.URLRedirects,
		Resolver:         opts.Resolver,
		Lang:             opts.DefaultLang,
		TFunc:            defaultTFunc,
		Languages:        defaultLangInfos,
	})

	// Content handler with content-specific middleware (outermost first)
	content := auth.Subgroup("",
		Compression, // Gzip responses >= 1KB when client accepts
		NewMethodFilterMiddleware(http.MethodGet, http.MethodHead), // Only allow GET and HEAD
		ContentExclusion(cfg.Site.Exclude),                         // Block hidden files and user-configured exclusions
		ExtensionRedirect(opts.Resolver, cfg.Site.StripExtensions), // Redirect .md URLs to clean URLs
		Metrics, // Innermost: measure actual handler time
	)
	content.HandleFunc("/", handler.ServeContent)

	// Shared middleware: all routes get security headers and request IDs
	var root http.Handler = mux
	root = RequestID(root)
	root = SecurityHeaders(root)

	server := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           root,
		ReadHeaderTimeout: cfg.Server.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.HTTP.WriteTimeout,
		IdleTimeout:       cfg.Server.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes(),
	}

	return &HTTPServer{
		server:  server,
		config:  cfg,
		handler: handler,
	}
}

// ListenURL returns a clickable URL for the server address.
func ListenURL(addr string) string {
	host, port, _ := net.SplitHostPort(addr)
	if host == "" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// Start starts the HTTP server. When ctx is cancelled, the server is
// gracefully shut down. Callers may also use Shutdown() directly.
func (s *HTTPServer) Start(ctx context.Context) error {
	slog.Info("Server started",
		slog.String("url", ListenURL(s.server.Addr)),
		slog.String("dir", s.config.Server.Dir),
		slog.Bool("dev", s.config.Server.DevMode),
	)

	go func() { // #nosec G118 -- shutdown context is intentionally independent of the cancelled parent
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.Server.HTTP.ShutdownTimeout)
	defer cancel()
	return s.server.Shutdown(shutdownCtx)
}
