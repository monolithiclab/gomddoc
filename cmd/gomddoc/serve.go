package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/server"
)

// ServeCmd holds all flags for the serve subcommand.
type ServeCmd struct {
	Dir           string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory or Git URL."`
	Port          string `name:"port" short:"p" default:":8080" env:"GOMDDOC_SERVER_PORT" help:"HTTP listen address (host:port). Use ':auto' for automatic port assignment."`
	DevMode       bool   `name:"dev" default:"false" env:"GOMDDOC_SERVER_DEV_MODE" help:"Enable development mode (no caching, verbose logging)."`
	GitSSHKey     string `name:"git-key-file" default:"" env:"GOMDDOC_SERVER_GIT_SSH_KEY" help:"Path to SSH private key file for Git authentication."`
	GitStorageDir string `name:"git-storage-dir" default:"" env:"GOMDDOC_SERVER_GIT_STORAGE_DIR" help:"Directory for disk-based Git clone storage (default: in-memory)."`
	Pprof         bool   `name:"pprof" default:"false" env:"GOMDDOC_SERVER_PPROF" help:"Enable pprof profiling endpoints at /debug/pprof/."`
	BasicAuthFile string `name:"basic-auth-file" default:"" env:"GOMDDOC_SERVER_BASIC_AUTH_FILE" help:"Path to htpasswd file for HTTP Basic Auth (bcrypt hashes only)."`
}

// resolvePort resolves the port, handling auto-port assignment.
// Returns the resolved port string (e.g., ":8081").
func resolvePort(port string) (string, error) {
	if !server.IsAutoPort(port) {
		return port, nil
	}
	resolved, err := server.FindAvailablePort(config.DefaultAutoPortStart)
	if err != nil {
		return "", fmt.Errorf("auto-port: %w", err)
	}
	slog.Info("Auto-assigned port", slog.String("port", resolved))
	return resolved, nil
}

// serveSetupResult holds the assembled server and cleanup function.
type serveSetupResult struct {
	httpServer *server.HTTPServer
	cleanup    func()
}

// setup creates the provider, pipeline, and HTTP server without starting it.
func (s *ServeCmd) setup() (*serveSetupResult, error) {
	port, err := resolvePort(s.Port)
	if err != nil {
		return nil, err
	}

	cfg, err := config.NewFromServeArgs(s.Dir, port, s.DevMode, s.GitSSHKey)
	if err != nil {
		return nil, err
	}

	cfg.Server.Pprof = s.Pprof

	if cfg.Server.DevMode {
		slog.Info("Development mode enabled",
			slog.String("dir", cfg.Server.Dir),
			slog.String("port", cfg.Server.Port),
			slog.String("theme", cfg.Site.Theme.Name),
			slog.Bool("pprof", cfg.Server.Pprof),
		)
	}

	var gitCfg provider.GitProviderConfig
	if cfg.Server.GitSSHKey != "" {
		gitCfg.SSHKeyFile = cfg.Server.GitSSHKey
	}
	if s.GitStorageDir != "" {
		h := sha256.Sum256([]byte(s.Dir))
		subdir := filepath.Join(s.GitStorageDir, hex.EncodeToString(h[:8]))
		gitCfg.StorageFactory = provider.DiskStorageFactory(subdir)
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, gitCfg)
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
	}

	if s.BasicAuthFile != "" {
		authStore, err := loadAuthStore(s.BasicAuthFile)
		if err != nil {
			_ = prov.Close()
			return nil, err
		}
		serverConfig.AuthStore = authStore
		slog.Info("Basic authentication enabled", slog.Int("users", authStore.Len()))
	}

	httpServer := server.NewHTTPServer(serverConfig)

	return &serveSetupResult{
		httpServer: httpServer,
		cleanup: func() {
			if closeErr := prov.Close(); closeErr != nil {
				slog.Error("Failed to close provider", slog.Any("error", closeErr))
			}
		},
	}, nil
}

// Run executes the serve command.
func (s *ServeCmd) Run() error {
	result, err := s.setup()
	if err != nil {
		return err
	}
	defer result.cleanup()

	sigCtx, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	return runUntilCancelled(sigCtx, result.httpServer)
}

// loadAuthStore parses an htpasswd file into a CredentialStore.
func loadAuthStore(path string) (*server.CredentialStore, error) {
	f, err := os.Open(path) // #nosec G304 -- user-specified config file path
	if err != nil {
		return nil, fmt.Errorf("opening htpasswd file: %w", err)
	}
	defer f.Close()
	store, err := server.ParseHTPasswd(f)
	if err != nil {
		return nil, fmt.Errorf("parsing htpasswd file: %w", err)
	}
	return store, nil
}
