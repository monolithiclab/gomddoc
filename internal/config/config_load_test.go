package config

import (
	"os"
	"path/filepath"
	"testing"
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

	cfg, err := NewFromServeArgs(tmpDir, ":8080", false, "")
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

	cfg, err := NewFromServeArgs(".", ":9999", false, "")
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

	cfg, err := NewFromServeArgs(tmpDir, ":8080", false, "")
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

	cfg, err := NewFromServeArgs(".", ":8080", true, "")
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if !cfg.Server.DevMode {
		t.Error("Expected DevMode to be true")
	}
}

func TestNewFromServeArgs_GitSSHKey(t *testing.T) {
	t.Parallel()

	cfg, err := NewFromServeArgs(".", ":8080", false, "/path/to/key")
	if err != nil {
		t.Fatalf("NewFromServeArgs() returned error: %v", err)
	}

	if cfg.Server.GitSSHKey != "/path/to/key" {
		t.Errorf("Expected GitSSHKey %q, got %q", "/path/to/key", cfg.Server.GitSSHKey)
	}
}
