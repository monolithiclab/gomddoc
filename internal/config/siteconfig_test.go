package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSiteConfig_Defaults(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		wantTitle   string
		wantTheme   string
		wantDomain  string
		wantDesc    string
	}{
		{
			name:       "current directory",
			dir:        ".",
			wantTitle:  "Gomddoc", // Capitalized basename of current directory
			wantTheme:  "default",
			wantDomain: "",
			wantDesc:   "",
		},
		{
			name:       "custom directory",
			dir:        "/var/docs",
			wantTitle:  "Docs",
			wantTheme:  "default",
			wantDomain: "",
			wantDesc:   "",
		},
		{
			name:       "api-docs directory",
			dir:        "/var/www/api-docs",
			wantTitle:  "Api-Docs", // Title case properly capitalizes after hyphens
			wantTheme:  "default",
			wantDomain: "",
			wantDesc:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSiteConfig(tt.dir)

			if sc.Meta.Title == "" {
				t.Error("Title should not be empty")
			}

			// For current directory, we can't predict exact title
			// but we can verify it's capitalized
			if tt.dir == "." {
				if sc.Meta.Title[0] < 'A' || sc.Meta.Title[0] > 'Z' {
					t.Errorf("Title first letter should be capitalized, got %q", sc.Meta.Title)
				}
			} else {
				if sc.Meta.Title != tt.wantTitle {
					t.Errorf("Meta.Title = %q, want %q", sc.Meta.Title, tt.wantTitle)
				}
			}

			if sc.Theme.Name != tt.wantTheme {
				t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, tt.wantTheme)
			}

			if sc.Meta.Domain != tt.wantDomain {
				t.Errorf("Meta.Domain = %q, want %q", sc.Meta.Domain, tt.wantDomain)
			}

			if sc.Meta.Description != tt.wantDesc {
				t.Errorf("Meta.Description = %q, want %q", sc.Meta.Description, tt.wantDesc)
			}
		})
	}
}

func TestSiteConfig_LoadFromFile(t *testing.T) {
	// Create temporary directory with config file
	tmpDir := t.TempDir()
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	configContent := `meta:
  domain: example.com
  title: "Test Site"
  description: "Test Description"

theme:
  name: "custom"
`

	configPath := filepath.Join(gomddocDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create SiteConfig and load from file
	sc := NewSiteConfig(tmpDir)
	if err := sc.LoadFromFile(tmpDir); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Verify values were loaded
	if sc.Meta.Title != "Test Site" {
		t.Errorf("Meta.Title = %q, want %q", sc.Meta.Title, "Test Site")
	}

	if sc.Meta.Domain != "example.com" {
		t.Errorf("Meta.Domain = %q, want %q", sc.Meta.Domain, "example.com")
	}

	if sc.Meta.Description != "Test Description" {
		t.Errorf("Meta.Description = %q, want %q", sc.Meta.Description, "Test Description")
	}

	if sc.Theme.Name != "custom" {
		t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, "custom")
	}
}

func TestSiteConfig_LoadFromFile_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	sc := NewSiteConfig(tmpDir)
	originalTitle := sc.Meta.Title

	// Should not return error when file is missing
	if err := sc.LoadFromFile(tmpDir); err != nil {
		t.Errorf("LoadFromFile should not error on missing file: %v", err)
	}

	// Should keep default values
	if sc.Meta.Title != originalTitle {
		t.Errorf("Title should remain default when config file missing")
	}
}

func TestSiteConfig_LoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	invalidContent := `meta:
  title: "Unclosed quote
  domain: example.com
`

	configPath := filepath.Join(gomddocDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	sc := NewSiteConfig(tmpDir)

	// Should return error for invalid YAML
	if err := sc.LoadFromFile(tmpDir); err == nil {
		t.Error("LoadFromFile should error on invalid YAML")
	}
}

func TestSiteConfig_EnvOverrides_String(t *testing.T) {
	// Set environment variables
	os.Setenv("GOMDDOC_META_TITLE", "Env Title")
	os.Setenv("GOMDDOC_META_DOMAIN", "env.example.com")
	os.Setenv("GOMDDOC_THEME_NAME", "dark")
	defer func() {
		os.Unsetenv("GOMDDOC_META_TITLE")
		os.Unsetenv("GOMDDOC_META_DOMAIN")
		os.Unsetenv("GOMDDOC_THEME_NAME")
	}()

	sc := NewSiteConfig(".")
	sc.ApplyEnvOverrides()

	if sc.Meta.Title != "Env Title" {
		t.Errorf("Meta.Title = %q, want %q", sc.Meta.Title, "Env Title")
	}

	if sc.Meta.Domain != "env.example.com" {
		t.Errorf("Meta.Domain = %q, want %q", sc.Meta.Domain, "env.example.com")
	}

	if sc.Theme.Name != "dark" {
		t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, "dark")
	}
}

func TestSiteConfig_EnvOverrides_Precedence(t *testing.T) {
	tmpDir := t.TempDir()
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	// Create config file with one value
	configContent := `meta:
  title: "File Title"
  domain: "file.example.com"
`
	configPath := filepath.Join(gomddocDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set env var to override
	os.Setenv("GOMDDOC_META_TITLE", "Env Title")
	defer os.Unsetenv("GOMDDOC_META_TITLE")

	sc := NewSiteConfig(tmpDir)

	// Load from file
	if err := sc.LoadFromFile(tmpDir); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// File value should be set
	if sc.Meta.Title != "File Title" {
		t.Errorf("After file load: Meta.Title = %q, want %q", sc.Meta.Title, "File Title")
	}

	// Apply env overrides
	sc.ApplyEnvOverrides()

	// Env should override file
	if sc.Meta.Title != "Env Title" {
		t.Errorf("After env override: Meta.Title = %q, want %q", sc.Meta.Title, "Env Title")
	}

	// Domain should remain from file (no env var set)
	if sc.Meta.Domain != "file.example.com" {
		t.Errorf("Meta.Domain = %q, want %q", sc.Meta.Domain, "file.example.com")
	}
}

func TestSiteConfig_Validate_EmptyTheme(t *testing.T) {
	sc := NewSiteConfig(".")
	sc.Theme.Name = ""

	if err := sc.Validate(); err != nil {
		t.Errorf("Validate should not error on empty theme: %v", err)
	}

	// Should auto-fix to default
	if sc.Theme.Name != "default" {
		t.Errorf("Theme.Name should be auto-fixed to 'default', got %q", sc.Theme.Name)
	}
}

func TestSiteConfig_Validate_DomainWithProtocol(t *testing.T) {
	sc := NewSiteConfig(".")
	sc.Meta.Domain = "https://example.com"

	if err := sc.Validate(); err == nil {
		t.Error("Validate should error when domain includes protocol")
	}
}

func TestSiteConfig_Validate_DomainWithPath(t *testing.T) {
	sc := NewSiteConfig(".")
	sc.Meta.Domain = "example.com/path"

	if err := sc.Validate(); err == nil {
		t.Error("Validate should error when domain includes path")
	}
}

func TestSiteConfig_Validate_ValidDomain(t *testing.T) {
	sc := NewSiteConfig(".")
	sc.Meta.Domain = "example.com"

	if err := sc.Validate(); err != nil {
		t.Errorf("Validate should not error on valid domain: %v", err)
	}

	// Empty domain should also be valid
	sc.Meta.Domain = ""
	if err := sc.Validate(); err != nil {
		t.Errorf("Validate should not error on empty domain: %v", err)
	}
}

func TestConfig_ApplyEnvOverrides(t *testing.T) {
	// Set environment variables for Config
	os.Setenv("GOMDDOC_PORT", ":9000")
	os.Setenv("GOMDDOC_DIR", "/tmp/test")
	os.Setenv("GOMDDOC_DEV_MODE", "true")
	defer func() {
		os.Unsetenv("GOMDDOC_PORT")
		os.Unsetenv("GOMDDOC_DIR")
		os.Unsetenv("GOMDDOC_DEV_MODE")
	}()

	cfg := New()
	cfg.ApplyEnvOverrides()

	if cfg.Port != ":9000" {
		t.Errorf("Port = %q, want %q", cfg.Port, ":9000")
	}

	if cfg.Dir != "/tmp/test" {
		t.Errorf("Dir = %q, want %q", cfg.Dir, "/tmp/test")
	}

	if !cfg.DevMode {
		t.Error("DevMode should be true")
	}
}

func TestConfig_EnvOverrides_Duration(t *testing.T) {
	os.Setenv("GOMDDOC_SHUTDOWN_TIMEOUT", "5s")
	defer os.Unsetenv("GOMDDOC_SHUTDOWN_TIMEOUT")

	cfg := New()
	cfg.ApplyEnvOverrides()

	if cfg.ShutdownTimeout.Seconds() != 5.0 {
		t.Errorf("ShutdownTimeout = %v, want 5s", cfg.ShutdownTimeout)
	}
}
