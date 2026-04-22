package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AdminServerConfig holds dependencies for creating an AdminServer.
type AdminServerConfig struct {
	Addr     string
	Provider provider.Provider
	Pprof    bool
}

// AdminServer serves admin endpoints (metrics, health, pprof) on a dedicated port.
type AdminServer struct {
	server *http.Server
}

// NewAdminServer creates a new admin server with health, metrics, and optionally pprof.
func NewAdminServer(cfg AdminServerConfig) *AdminServer {
	mux := http.NewServeMux()
	admin := NewGroup(mux, "")

	// Health endpoints
	healthHandler := NewHealthHandler(cfg.Provider)
	health := admin.Subgroup("/health")
	health.HandleFunc("GET /live", healthHandler.LiveHandler)
	health.HandleFunc("GET /ready", healthHandler.ReadyHandler)

	// Metrics
	admin.Handle("/metrics", promhttp.Handler())

	// Pprof
	if cfg.Pprof {
		slog.Warn("pprof profiling enabled on admin port — do not expose publicly")
		debug := admin.Subgroup("/debug/pprof")
		debug.HandleFunc("GET /", pprof.Index)
		debug.HandleFunc("GET /cmdline", pprof.Cmdline)
		debug.HandleFunc("GET /profile", pprof.Profile)
		debug.HandleFunc("GET /symbol", pprof.Symbol)
		debug.HandleFunc("GET /trace", pprof.Trace)
	}

	return &AdminServer{
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           mux,
			ReadHeaderTimeout: config.DefaultReadHeaderTimeout,
		},
	}
}

// Handler returns the admin server's HTTP handler for testing.
func (s *AdminServer) Handler() http.Handler {
	return s.server.Handler
}

// Start starts the admin HTTP server.
func (s *AdminServer) Start(ctx context.Context) error {
	slog.Info("Admin server started", slog.String("url", ListenURL(s.server.Addr)))
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the admin server.
func (s *AdminServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
