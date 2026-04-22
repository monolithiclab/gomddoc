package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/template"
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
) *HTTPServer {
	// Create handler with new signature
	handler := NewHandler(provider, registry, templateRenderer, &cfg.Site)

	// Apply middleware chain to content handler (outermost first)
	var h http.Handler = http.HandlerFunc(handler.ServeContent)
	h = BlockHiddenPaths(h)                              // Block all hidden files/directories
	h = MethodFilter(http.MethodGet, http.MethodHead)(h) // Only allow GET and HEAD
	h = Compression(h)                                   // Gzip responses >= 1KB when client accepts
	h = SecurityHeaders(h)                               // Must be outermost so headers are set first

	// Health endpoints bypass all middleware
	healthHandler := NewHealthHandler(provider)

	// Route health endpoints before middleware-wrapped content handler
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", healthHandler.LiveHandler)
	mux.HandleFunc("GET /health/ready", healthHandler.ReadyHandler)
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

// Start starts the HTTP server
func (s *HTTPServer) Start(ctx context.Context) error {
	slog.Info("Listening...", slog.String("Addr", s.server.Addr))
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
