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

	// AuthStore gates /debug/pprof/* — see mountPprof.
	AuthStore *CredentialStore

	// HTTP carries the same listener timeouts as the main server; previously
	// only ReadHeaderTimeout was set, so a slow reader held a connection
	// indefinitely and keep-alives were never reaped. WriteTimeout is safe for
	// the long-sampling pprof routes: net/http/pprof extends the deadline to
	// WriteTimeout+seconds itself (configureWriteDeadline).
	HTTP config.HTTPConfig
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
		mountPprof(admin, cfg.AuthStore)
	}

	return &AdminServer{
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           mux,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
			MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes(),
		},
	}
}

// mountPprof registers the pprof routes on g, gated by store when credentials
// are configured. Both listeners mount through here — the main server when
// admin endpoints share its port, the admin server when they do not — because
// the two previously kept separate copies of this block and drifted apart:
// /debug/pprof/cmdline returns the full command line (including the path to the
// credential file) and /debug/pprof/heap dumps in-memory content, so "who may
// reach pprof" must not depend on which listener happens to carry it.
//
// g must not already apply auth, or credentials are demanded twice.
//
// WriteTimeout needs no special handling: net/http/pprof extends its own write
// deadline to WriteTimeout+seconds for the sampling endpoints.
func mountPprof(g *RouteGroup, store *CredentialStore) {
	var mw []func(http.Handler) http.Handler
	if store != nil {
		mw = append(mw, NewBasicAuthMiddleware(store, basicAuthRealm))
		slog.Warn("pprof profiling enabled — do not use in production")
	} else {
		slog.Warn("pprof profiling enabled without --basic-auth-file — do not use in production; " +
			"anyone who can reach this port can dump the heap and the command line")
	}

	debug := g.Subgroup("/debug/pprof", mw...)
	debug.HandleFunc("GET /", pprof.Index)
	debug.HandleFunc("GET /cmdline", pprof.Cmdline)
	debug.HandleFunc("GET /profile", pprof.Profile)
	debug.HandleFunc("GET /symbol", pprof.Symbol)
	debug.HandleFunc("GET /trace", pprof.Trace)
}

// Handler returns the admin server's HTTP handler for testing.
func (s *AdminServer) Handler() http.Handler {
	return s.server.Handler
}

// Start starts the admin HTTP server. When ctx is cancelled, the server
// is gracefully shut down.
func (s *AdminServer) Start(ctx context.Context) error {
	slog.Info("Admin server started", slog.String("url", ListenURL(s.server.Addr)))

	go func() { // #nosec G118 -- shutdown context is intentionally independent of the cancelled parent
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the admin server.
func (s *AdminServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
