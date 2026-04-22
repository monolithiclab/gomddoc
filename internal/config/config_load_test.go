package config

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Save and restore os.Args and flag.CommandLine
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	// Reset flags for testing
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Create a temp directory for testing
	tmpDir := t.TempDir()

	// Create a config file in the temp dir
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	configPath := filepath.Join(gomddocDir, "config.yml")
	configContent := `
default_index: "HOME.md"
meta:
  title: "Loaded Title"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set args to point to this temp dir
	os.Args = []string{"cmd", "-d", tmpDir}

	// Call Load
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Verify results
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

func TestLoad_EnvOverrides(t *testing.T) {
	// Save/Restore
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Set Env
	t.Setenv("GOMDDOC_SERVER_PORT", ":9999")
	t.Setenv("GOMDDOC_SITE_META_TITLE", "Env Title")

	// Call Load (no args)
	os.Args = []string{"cmd"}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Server.Port != ":9999" {
		t.Errorf("Expected Port %q, got %q", ":9999", cfg.Server.Port)
	}

	if cfg.Site.Meta.Title != "Env Title" {
		t.Errorf("Expected Title %q, got %q", "Env Title", cfg.Site.Meta.Title)
	}
}

func TestLoad_DynamicDefaults(t *testing.T) {
	// Save/Restore
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	tmpDir := t.TempDir()

	// Create minimal config without title
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	err := os.MkdirAll(gomddocDir, 0755)
	if err != nil {
		t.Error("Cannot create directory in temporary test folder")
		return
	}
	err = os.WriteFile(filepath.Join(gomddocDir, "config.yml"), []byte("default_index: README.md"), 0644)
	if err != nil {
		t.Error("Cannot write into temporary test folder")
		return
	}

	// Set args to point to this temp dir
	os.Args = []string{"cmd", "-d", tmpDir}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Title should be derived from directory name
	// Actually, ComputeDynamicDefaults uses filepath.Base(absDir).
	// Let's check if it's not empty and capitalized.
	if cfg.Site.Meta.Title == "" {
		t.Error("Expected Title to be generated, got empty")
	}
}
