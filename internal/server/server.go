package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/config"
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

// NewHTTPServer creates a new HTTP server with the given dependencies
func NewHTTPServer(
	cfg *config.Config,
	provider provider.Provider,
	registry renderer.RendererRegistry,
	templateRenderer template.Renderer,
	metaIndex *metadata.Index,
) *HTTPServer {
	// Create handler with new signature
	handler := NewHandler(provider, registry, templateRenderer, &cfg.Site)

	// Apply middleware chain (outermost first, innermost closest to handler)
	var h http.Handler = http.HandlerFunc(handler.ServeContent)
	h = Metrics(h)                                       // Innermost: measure actual handler time
	h = BlockHiddenPaths(h)                              // Block all hidden files/directories
	h = MethodFilter(http.MethodGet, http.MethodHead)(h) // Only allow GET and HEAD
	h = Compression(h)                                   // Gzip responses >= 1KB when client accepts
	h = RequestID(h)                                     // Assign unique request ID for tracing
	h = SecurityHeaders(h)                               // Must be outermost so headers are set first

	// Health and metrics endpoints bypass content middleware
	healthHandler := NewHealthHandler(provider)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", healthHandler.LiveHandler)
	mux.HandleFunc("GET /health/ready", healthHandler.ReadyHandler)
	mux.Handle("/metrics", promhttp.Handler())

	if metaIndex != nil {
		metaHandler := NewMetadataHandler(metaIndex)
		mux.HandleFunc("GET /api/tags", metaHandler.TagsHandler)
		mux.HandleFunc("GET /api/tags/{tag}", metaHandler.TagPagesHandler)
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

// listenURL returns a clickable URL for the server address.
func listenURL(addr string) string {
	host, port, _ := net.SplitHostPort(addr)
	if host == "" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// Start starts the HTTP server
func (s *HTTPServer) Start(ctx context.Context) error {
	slog.Info("Server started",
		slog.String("url", listenURL(s.server.Addr)),
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
