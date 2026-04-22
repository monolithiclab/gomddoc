package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSiteConfig_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dir       string
		wantTheme string
	}{
		{
			name:      "current directory",
			dir:       ".",
			wantTheme: "default",
		},
		{
			name:      "custom directory",
			dir:       "/var/docs",
			wantTheme: "default",
		},
		{
			name:      "api-docs directory",
			dir:       "/var/www/api-docs",
			wantTheme: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := NewSiteConfig(tt.dir)

			if sc.Meta.Title == "" {
				t.Error("Title should not be empty")
			}

			// For current directory, verify title is capitalized
			if tt.dir == "." {
				if sc.Meta.Title[0] < 'A' || sc.Meta.Title[0] > 'Z' {
					t.Errorf("Title first letter should be capitalized, got %q", sc.Meta.Title)
				}
			}

			if sc.Theme.Name != tt.wantTheme {
				t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, tt.wantTheme)
			}

			if sc.Meta.Domain != "" {
				t.Errorf("Meta.Domain = %q, want empty", sc.Meta.Domain)
			}

			if sc.Meta.Description != "" {
				t.Errorf("Meta.Description = %q, want empty", sc.Meta.Description)
			}
		})
	}
}

func TestSiteConfig_LoadFromFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		configYAML   string
		wantTitle    string
		wantDomain   string
		wantDesc     string
		wantTheme    string
		wantErr      bool
		missingFile  bool
		expectDefaut bool
	}{
		{
			name: "valid config file",
			configYAML: `meta:
  domain: example.com
  title: "Test Site"
  description: "Test Description"

theme:
  name: "custom"
`,
			wantTitle:  "Test Site",
			wantDomain: "example.com",
			wantDesc:   "Test Description",
			wantTheme:  "custom",
		},
		{
			name:         "missing config file - uses defaults",
			missingFile:  true,
			expectDefaut: true,
		},
		{
			name: "invalid YAML",
			configYAML: `meta:
  title: "Unclosed quote
  domain: example.com
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			sc := NewSiteConfig(tmpDir)
			originalTitle := sc.Meta.Title

			// Create config file if needed
			if !tt.missingFile {
				gomddocDir := filepath.Join(tmpDir, ".gomddoc")
				if err := os.MkdirAll(gomddocDir, 0755); err != nil {
					t.Fatalf("Failed to create .gomddoc dir: %v", err)
				}

				configPath := filepath.Join(gomddocDir, "config.yml")
				if err := os.WriteFile(configPath, []byte(tt.configYAML), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			err := sc.LoadFromFile(tmpDir)
			if tt.wantErr {
				if err == nil {
					t.Error("LoadFromFile() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadFromFile() error = %v, want nil", err)
			}

			if tt.expectDefaut {
				if sc.Meta.Title != originalTitle {
					t.Error("Title should remain default when config file missing")
				}
				return
			}

			if sc.Meta.Title != tt.wantTitle {
				t.Errorf("Meta.Title = %q, want %q", sc.Meta.Title, tt.wantTitle)
			}

			if sc.Meta.Domain != tt.wantDomain {
				t.Errorf("Meta.Domain = %q, want %q", sc.Meta.Domain, tt.wantDomain)
			}

			if sc.Meta.Description != tt.wantDesc {
				t.Errorf("Meta.Description = %q, want %q", sc.Meta.Description, tt.wantDesc)
			}

			if sc.Theme.Name != tt.wantTheme {
				t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, tt.wantTheme)
			}
		})
	}
}

func TestSiteConfig_EnvOverrides(t *testing.T) {
	// Note: Cannot use t.Parallel() because subtests use t.Setenv()

	tests := []struct {
		name      string
		envVars   map[string]string
		wantTitle string
		wantValue string
		checkFunc func(*testing.T, *SiteConfig)
	}{
		{
			name: "override title",
			envVars: map[string]string{
				"GOMDDOC_META_TITLE": "Env Title",
			},
			checkFunc: func(t *testing.T, sc *SiteConfig) {
				if sc.Meta.Title != "Env Title" {
					t.Errorf("Meta.Title = %q, want %q", sc.Meta.Title, "Env Title")
				}
			},
		},
		{
			name: "override domain",
			envVars: map[string]string{
				"GOMDDOC_META_DOMAIN": "env.example.com",
			},
			checkFunc: func(t *testing.T, sc *SiteConfig) {
				if sc.Meta.Domain != "env.example.com" {
					t.Errorf("Meta.Domain = %q, want %q", sc.Meta.Domain, "env.example.com")
				}
			},
		},
		{
			name: "override theme",
			envVars: map[string]string{
				"GOMDDOC_THEME_NAME": "dark",
			},
			checkFunc: func(t *testing.T, sc *SiteConfig) {
				if sc.Theme.Name != "dark" {
					t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, "dark")
				}
			},
		},
		{
			name: "env overrides file config",
			envVars: map[string]string{
				"GOMDDOC_META_TITLE": "Env Title",
			},
			checkFunc: func(t *testing.T, sc *SiteConfig) {
				if sc.Meta.Title != "Env Title" {
					t.Errorf("After env override: Meta.Title = %q, want %q", sc.Meta.Title, "Env Title")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() with t.Setenv()

			// Set environment variables using t.Setenv (Go 1.17+)
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			sc := NewSiteConfig(".")
			sc.ApplyEnvOverrides()

			tt.checkFunc(t, sc)
		})
	}
}

func TestSiteConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(*SiteConfig)
		wantErr bool
		wantFix func(*testing.T, *SiteConfig)
	}{
		{
			name: "empty theme auto-fixes to default",
			setup: func(sc *SiteConfig) {
				sc.Theme.Name = ""
			},
			wantErr: false,
			wantFix: func(t *testing.T, sc *SiteConfig) {
				if sc.Theme.Name != "default" {
					t.Errorf("Theme.Name should be auto-fixed to 'default', got %q", sc.Theme.Name)
				}
			},
		},
		{
			name: "domain with protocol",
			setup: func(sc *SiteConfig) {
				sc.Meta.Domain = "https://example.com"
			},
			wantErr: true,
		},
		{
			name: "domain with path",
			setup: func(sc *SiteConfig) {
				sc.Meta.Domain = "example.com/path"
			},
			wantErr: true,
		},
		{
			name: "valid domain",
			setup: func(sc *SiteConfig) {
				sc.Meta.Domain = "example.com"
			},
			wantErr: false,
		},
		{
			name: "empty domain",
			setup: func(sc *SiteConfig) {
				sc.Meta.Domain = ""
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := NewSiteConfig(".")
			tt.setup(sc)

			err := sc.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}

			if tt.wantFix != nil {
				tt.wantFix(t, sc)
			}
		})
	}
}

func TestConfig_ApplyEnvOverrides(t *testing.T) {
	// Note: Cannot use t.Parallel() because subtests use t.Setenv()

	tests := []struct {
		name      string
		envVars   map[string]string
		checkFunc func(*testing.T, *Config)
	}{
		{
			name: "string overrides",
			envVars: map[string]string{
				"GOMDDOC_PORT": ":9000",
				"GOMDDOC_DIR":  "/tmp/test",
			},
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.Port != ":9000" {
					t.Errorf("Port = %q, want %q", cfg.Port, ":9000")
				}
				if cfg.Dir != "/tmp/test" {
					t.Errorf("Dir = %q, want %q", cfg.Dir, "/tmp/test")
				}
			},
		},
		{
			name: "bool override",
			envVars: map[string]string{
				"GOMDDOC_DEV_MODE": "true",
			},
			checkFunc: func(t *testing.T, cfg *Config) {
				if !cfg.DevMode {
					t.Error("DevMode should be true")
				}
			},
		},
		{
			name: "duration override",
			envVars: map[string]string{
				"GOMDDOC_SHUTDOWN_TIMEOUT": "5s",
			},
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.ShutdownTimeout.Seconds() != 5.0 {
					t.Errorf("ShutdownTimeout = %v, want 5s", cfg.ShutdownTimeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() with t.Setenv()

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg := New()
			cfg.ApplyEnvOverrides()

			tt.checkFunc(t, cfg)
		})
	}
}
