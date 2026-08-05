package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestInitCmd_Run(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := &InitCmd{Dir: dir, Theme: "default"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Init.Run() error: %v", err)
	}

	configPath := filepath.Join(dir, config.ConfigDirName, config.ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "default_index") {
		t.Error("config missing default_index")
	}
	if !strings.Contains(content, "name: default") {
		t.Error("config missing theme name")
	}
}

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := &InitCmd{Dir: dir, Theme: "default"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("first Init.Run() error: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error on second init, got nil")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitCmd_CustomTheme(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := &InitCmd{Dir: dir, Theme: "midnight"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Init.Run() error: %v", err)
	}

	configPath := filepath.Join(dir, config.ConfigDirName, config.ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if !strings.Contains(string(data), "name: midnight") {
		t.Error("config does not contain custom theme")
	}
}

func TestInitCmd_ConfigRoundTrips(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cmd := &InitCmd{Dir: dir, Theme: "nord"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Init.Run() error: %v", err)
	}

	// Load through the production path, not yaml.Unmarshal: initConfig
	// (init.go) hand-mirrors SiteConfig's yaml tags with no compile-time link,
	// and LoadFromFile now rejects keys SiteConfig does not define. Decoding
	// leniently here would let the scaffold drift into generating a config
	// that `gomddoc serve` refuses to load, with this test still green.
	sc := config.NewSiteConfig(dir)
	if err := sc.LoadFromFile(dir); err != nil {
		t.Fatalf("LoadFromFile on a freshly scaffolded site: %v", err)
	}

	if sc.DefaultIndex != config.DefaultIndex {
		t.Errorf("DefaultIndex = %q, want %q", sc.DefaultIndex, config.DefaultIndex)
	}
	if sc.Theme.Name != "nord" {
		t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, "nord")
	}
}

func TestGenerateConfigYAML(t *testing.T) {
	t.Parallel()

	content, err := generateConfigYAML("My Project", "material")
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "title: My Project") {
		t.Error("missing title")
	}
	if !strings.Contains(s, "name: material") {
		t.Error("missing theme")
	}
}

func TestInitCmd_NonexistentParentDir(t *testing.T) {
	t.Parallel()

	cmd := &InitCmd{Dir: "/nonexistent/path/that/does/not/exist", Theme: "default"}
	err := cmd.Run()
	if err == nil {
		t.Error("Init.Run() should fail for nonexistent parent dir")
	}
}

func TestInitCmd_Defaults(t *testing.T) {
	t.Parallel()

	cmd := InitCmd{}
	if cmd.Dir != "" {
		t.Errorf("Dir default = %q, want empty (Kong sets '.')", cmd.Dir)
	}
	if cmd.Theme != "" {
		t.Errorf("Theme default = %q, want empty (Kong sets 'default')", cmd.Theme)
	}
}

func TestInitCmd_ReadOnlyDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0750) })

	cmd := &InitCmd{Dir: dir, Theme: "default"}
	err := cmd.Run()
	if err == nil {
		t.Error("Init.Run() should fail with read-only dir")
	}
}

func TestGenerateConfigYAML_AllFields(t *testing.T) {
	t.Parallel()

	content, err := generateConfigYAML("Test Project", "nord")
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, config.DefaultIndex) {
		t.Error("missing default_index value")
	}
	if !strings.Contains(s, "dir_index: false") {
		t.Error("missing dir_index setting")
	}
	if !strings.Contains(s, config.DefaultHighlightTheme) {
		t.Error("missing highlight theme")
	}
}
