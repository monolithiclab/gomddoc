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
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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

// HTTPServerConfig holds all dependencies for creating an HTTPServer.
type HTTPServerConfig struct {
	Config           *config.Config
	Provider         provider.Provider
	Registry         renderer.RendererRegistry
	EnricherRegistry enricher.EnricherRegistry
	TemplateRenderer template.Renderer
	MetaIndex        *metadata.Index // nil disables metadata API
	RedirectFinder   RedirectFinder  // nil disables redirect lookup
	StaticFS         fs.FS           // nil disables static asset serving
}

// NewHTTPServer creates a new HTTP server with the given dependencies.
func NewHTTPServer(opts HTTPServerConfig) *HTTPServer {
	cfg := opts.Config
	handler := NewHandler(opts.Provider, opts.Registry, opts.EnricherRegistry, opts.TemplateRenderer, &cfg.Site, opts.RedirectFinder)

	// Apply middleware chain (outermost first, innermost closest to handler)
	var h http.Handler = http.HandlerFunc(handler.ServeContent)
	h = Metrics(h)                                       // Innermost: measure actual handler time
	h = BlockHiddenPaths(h)                              // Block all hidden files/directories
	h = MethodFilter(http.MethodGet, http.MethodHead)(h) // Only allow GET and HEAD
	h = Compression(h)                                   // Gzip responses >= 1KB when client accepts
	h = RequestID(h)                                     // Assign unique request ID for tracing
	h = SecurityHeaders(h)                               // Must be outermost so headers are set first

	// Health and metrics endpoints bypass content middleware
	healthHandler := NewHealthHandler(opts.Provider)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", healthHandler.LiveHandler)
	mux.HandleFunc("GET /health/ready", healthHandler.ReadyHandler)
	mux.Handle("/metrics", promhttp.Handler())

	if opts.MetaIndex != nil {
		metaHandler := NewMetadataHandler(opts.MetaIndex)
		mux.HandleFunc("GET /api/tags", metaHandler.TagsHandler)
		mux.HandleFunc("GET /api/tags/{tag}", metaHandler.TagPagesHandler)
	}

	if opts.StaticFS != nil {
		assetsHandler := NewAssetsHandler(opts.StaticFS)
		mux.Handle("GET /_assets/", http.StripPrefix("/_assets/", assetsHandler))
	}

	if cfg.Server.Pprof {
		slog.Warn("pprof profiling enabled — do not use in production")
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	}

	mux.Handle("/", h)

	server := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           mux,
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

// Start starts the HTTP server
func (s *HTTPServer) Start(ctx context.Context) error {
	slog.Info("Server started",
		slog.String("url", ListenURL(s.server.Addr)),
		slog.String("dir", s.config.Server.Dir),
		slog.Bool("dev", s.config.Server.DevMode),
	)
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
