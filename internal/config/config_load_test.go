package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFromServeArgs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a config file in the temp dir
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	configContent := `
default_index: "HOME.md"
meta:
  title: "Loaded Title"
`
	if err := os.WriteFile(filepath.Join(gomddocDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := NewFromServeArgs(ServeArgs{Dir: tmpDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if cfg.Server.Dir != tmpDir {
		t.Errorf("Expected Dir %q, got %q", tmpDir, cfg.Server.Dir)
	}

	if cfg.Site.DefaultIndex != "HOME.md" {
		t.Errorf("Expected DefaultIndex %q, got %q", "HOME.md", cfg.Site.DefaultIndex)
	}

	if cfg.Site.Meta.Title != "Loaded Title" {
		t.Errorf("Expected Title %q, got %q", "Loaded Title", cfg.Site.Meta.Title)
	}
}

func TestNewFromServeArgs_EnvOverrides(t *testing.T) {
	t.Setenv("GOMDDOC_SITE_META_TITLE", "Env Title")

	cfg, err := NewFromServeArgs(ServeArgs{Dir: t.TempDir(), Port: ":9999"})
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if cfg.Server.Port != ":9999" {
		t.Errorf("Expected Port %q, got %q", ":9999", cfg.Server.Port)
	}

	if cfg.Site.Meta.Title != "Env Title" {
		t.Errorf("Expected Title %q, got %q", "Env Title", cfg.Site.Meta.Title)
	}
}

func TestNewFromServeArgs_DynamicDefaults(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create minimal config without title
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Cannot create directory in temporary test folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gomddocDir, "config.yml"), []byte("default_index: README.md"), 0644); err != nil {
		t.Fatalf("Cannot write into temporary test folder: %v", err)
	}

	cfg, err := NewFromServeArgs(ServeArgs{Dir: tmpDir, Port: ":8080"})
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	// Title should be derived from directory name
	if cfg.Site.Meta.Title == "" {
		t.Error("Expected Title to be generated, got empty")
	}
}

func TestNewFromServeArgs_DevMode(t *testing.T) {
	t.Parallel()

	cfg, err := NewFromServeArgs(ServeArgs{Dir: t.TempDir(), Port: ":8080", DevMode: true})
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if !cfg.Server.DevMode {
		t.Error("Expected DevMode to be true")
	}
}

// TestNewFromServeArgs_ServerEnvOverrides covers the server-level env vars that no
// CLI flag owns. A config walk that reaches Site but not Server makes them all inert
// while `info` still advertises them.
func TestNewFromServeArgs_ServerEnvOverrides(t *testing.T) {
	t.Setenv("GOMDDOC_SERVER_DEV_MODE", "true")
	t.Setenv("GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("GOMDDOC_SERVER_HTTP_READ_HEADER_TIMEOUT", "7s")
	t.Setenv("GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT", "45s")
	t.Setenv("GOMDDOC_SERVER_HTTP_IDLE_TIMEOUT", "90s")
	t.Setenv("GOMDDOC_SERVER_HTTP_MAX_HEADER_MB", "4")

	// DevMode false here on purpose: `serve` has no --dev flag, so the env var is
	// the only way to set it and an unconditional assignment would erase it.
	cfg, err := NewFromServeArgs(ServeArgs{Dir: t.TempDir(), Port: ":8080"})
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if !cfg.Server.DevMode {
		t.Error("GOMDDOC_SERVER_DEV_MODE=true did not enable DevMode")
	}

	want := HTTPConfig{
		ShutdownTimeout:   3 * time.Second,
		ReadHeaderTimeout: 7 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderMB:       4,
	}
	if cfg.Server.HTTP != want {
		t.Errorf("HTTP = %+v, want %+v", cfg.Server.HTTP, want)
	}
}

// TestNewFromServeArgs_ArgsBeatEnv guards the ordering of the whole-Config env pass.
// Kong already folds `env:` tags into the flags it owns, so the args carry the resolved
// flag > env > default answer; running the env walk after the args re-reads the
// environment and inverts that. Pprof is the sharp case — the env var is set here and
// the arg is false, which is what `--no-pprof` produces.
func TestNewFromServeArgs_ArgsBeatEnv(t *testing.T) {
	t.Setenv("GOMDDOC_SERVER_PORT", ":1234")
	t.Setenv("GOMDDOC_SERVER_ADMIN_PORT", ":1235")
	t.Setenv("GOMDDOC_SERVER_PPROF", "true")

	cfg, err := NewFromServeArgs(ServeArgs{Dir: t.TempDir(), Port: ":9999", AdminPort: ":9998"})
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if cfg.Server.Port != ":9999" {
		t.Errorf("Port = %q, want %q (flag must beat env)", cfg.Server.Port, ":9999")
	}
	if want := "127.0.0.1:9998"; cfg.Server.AdminPort != want {
		t.Errorf("AdminPort = %q, want %q (flag must beat env)", cfg.Server.AdminPort, want)
	}
	if cfg.Server.Pprof {
		t.Error("Pprof = true; --no-pprof must beat GOMDDOC_SERVER_PPROF")
	}
}
