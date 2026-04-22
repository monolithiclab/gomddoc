package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()
	config := New()

	if config.Server.Dir != "." {
		t.Errorf("Expected default dir '.', got %q", config.Server.Dir)
	}

	if config.Server.Port != ":8080" {
		t.Errorf("Expected default port ':8080', got %q", config.Server.Port)
	}

	if config.Site.DefaultIndex != "README.md" {
		t.Errorf("Expected site default index 'README.md', got %q", config.Site.DefaultIndex)
	}

	if config.Site.DirIndex != false {
		t.Errorf("Expected site dir index false (secure by default), got %v", config.Site.DirIndex)
	}

	if config.Server.HTTP.ShutdownTimeout.Seconds() != 1 {
		t.Errorf("Expected shutdown timeout 1s, got %v", config.Server.HTTP.ShutdownTimeout)
	}

	// Test HTTP server timeout defaults
	if config.Server.HTTP.ReadHeaderTimeout != DefaultReadHeaderTimeout {
		t.Errorf("Expected ReadHeaderTimeout %v, got %v", DefaultReadHeaderTimeout, config.Server.HTTP.ReadHeaderTimeout)
	}

	if config.Server.HTTP.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("Expected WriteTimeout %v, got %v", DefaultWriteTimeout, config.Server.HTTP.WriteTimeout)
	}

	if config.Server.HTTP.IdleTimeout != DefaultIdleTimeout {
		t.Errorf("Expected IdleTimeout %v, got %v", DefaultIdleTimeout, config.Server.HTTP.IdleTimeout)
	}

	if config.Server.HTTP.MaxHeaderMB != DefaultMaxHeaderMB {
		t.Errorf("Expected MaxHeaderMB %v, got %v", DefaultMaxHeaderMB, config.Server.HTTP.MaxHeaderMB)
	}

	// Test MaxHeaderBytes() method converts MB to bytes
	expectedBytes := DefaultMaxHeaderMB << 20
	if config.MaxHeaderBytes() != expectedBytes {
		t.Errorf("Expected MaxHeaderBytes() %v, got %v", expectedBytes, config.MaxHeaderBytes())
	}
}

func TestValidate(t *testing.T) {
	config := New()
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	t.Parallel()
	config := New()
	config.Server.Port = "invalid"
	err := config.Validate()
	if err == nil {
		t.Error("Expected validation to fail for invalid port format")
	}
}

func TestValidate_PortRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		port      string
		wantError bool
	}{
		{
			name:      "port 0 is invalid",
			port:      ":0",
			wantError: true,
		},
		{
			name:      "port 65536 is invalid",
			port:      ":65536",
			wantError: true,
		},
		{
			name:      "port 99999 is invalid",
			port:      ":99999",
			wantError: true,
		},
		{
			name:      "negative port is invalid",
			port:      ":-1",
			wantError: true,
		},
		{
			name:      "port 1 is valid",
			port:      ":1",
			wantError: false,
		},
		{
			name:      "port 8080 is valid",
			port:      ":8080",
			wantError: false,
		},
		{
			name:      "port 65535 is valid",
			port:      ":65535",
			wantError: false,
		},
		{
			name:      "port with host is valid",
			port:      "localhost:8080",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Server.Port = tt.port
			err := cfg.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation to fail for port %q", tt.port)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected validation to pass for port %q, got error: %v", tt.port, err)
			}
		})
	}
}

func TestValidate_Directory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dir       string
		wantError bool
	}{
		{
			name:      "current directory is valid",
			dir:       ".",
			wantError: false,
		},
		{
			name:      "non-existent directory fails",
			dir:       "/nonexistent/path/that/does/not/exist",
			wantError: true,
		},
		// Git URLs skip filesystem validation
		{
			name:      "git protocol URL skips validation",
			dir:       "git://github.com/user/repo",
			wantError: false,
		},
		{
			name:      "git+ssh URL skips validation",
			dir:       "git+ssh://git@github.com/org/docs#main",
			wantError: false,
		},
		{
			name:      "git+https URL skips validation",
			dir:       "git+https://github.com/user/docs#develop:docs",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Server.Dir = tt.dir
			err := cfg.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation to fail for dir %q", tt.dir)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected validation to pass for dir %q, got error: %v", tt.dir, err)
			}
		})
	}
}

func TestValidate_DirectoryIsFile(t *testing.T) {
	// Create a temporary file to test that files are rejected
	tmpFile, err := os.CreateTemp("", "config_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cfg := New()
	cfg.Server.Dir = tmpFile.Name()
	err = cfg.Validate()
	if err == nil {
		t.Error("Expected validation to fail when Dir is a file, not a directory")
	}
}

func TestValidate_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		timeout   time.Duration
		wantError bool
	}{
		{
			name:      "positive timeout is valid",
			timeout:   5 * time.Second,
			wantError: false,
		},
		{
			name:      "zero timeout is valid",
			timeout:   0,
			wantError: false,
		},
		{
			name:      "negative timeout fails",
			timeout:   -1 * time.Second,
			wantError: true,
		},
		{
			name:      "very long timeout is valid but warns",
			timeout:   120 * time.Second,
			wantError: false, // Warns but doesn't fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Server.HTTP.ShutdownTimeout = tt.timeout
			err := cfg.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation to fail for timeout %v", tt.timeout)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected validation to pass for timeout %v, got error: %v", tt.timeout, err)
			}
		})
	}
}

func TestValidate_TimeoutDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Config)
		check  func(*testing.T, *Config)
	}{
		{
			name: "negative ReadHeaderTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.ReadHeaderTimeout = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.ReadHeaderTimeout != DefaultReadHeaderTimeout {
					t.Errorf("Expected ReadHeaderTimeout to be reset to %v, got %v",
						DefaultReadHeaderTimeout, c.Server.HTTP.ReadHeaderTimeout)
				}
			},
		},
		{
			name: "zero ReadHeaderTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.ReadHeaderTimeout = 0
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.ReadHeaderTimeout != DefaultReadHeaderTimeout {
					t.Errorf("Expected ReadHeaderTimeout to be reset to %v, got %v",
						DefaultReadHeaderTimeout, c.Server.HTTP.ReadHeaderTimeout)
				}
			},
		},
		{
			name: "excessive ReadHeaderTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.ReadHeaderTimeout = 120 * time.Second
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.ReadHeaderTimeout != DefaultReadHeaderTimeout {
					t.Errorf("Expected ReadHeaderTimeout to be reset to %v, got %v",
						DefaultReadHeaderTimeout, c.Server.HTTP.ReadHeaderTimeout)
				}
			},
		},
		{
			name: "negative WriteTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.WriteTimeout = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.WriteTimeout != DefaultWriteTimeout {
					t.Errorf("Expected WriteTimeout to be reset to %v, got %v",
						DefaultWriteTimeout, c.Server.HTTP.WriteTimeout)
				}
			},
		},
		{
			name: "excessive WriteTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.WriteTimeout = 10 * time.Minute
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.WriteTimeout != DefaultWriteTimeout {
					t.Errorf("Expected WriteTimeout to be reset to %v, got %v",
						DefaultWriteTimeout, c.Server.HTTP.WriteTimeout)
				}
			},
		},
		{
			name: "negative IdleTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.IdleTimeout = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.IdleTimeout != DefaultIdleTimeout {
					t.Errorf("Expected IdleTimeout to be reset to %v, got %v",
						DefaultIdleTimeout, c.Server.HTTP.IdleTimeout)
				}
			},
		},
		{
			name: "excessive IdleTimeout resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.IdleTimeout = 15 * time.Minute
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.IdleTimeout != DefaultIdleTimeout {
					t.Errorf("Expected IdleTimeout to be reset to %v, got %v",
						DefaultIdleTimeout, c.Server.HTTP.IdleTimeout)
				}
			},
		},
		{
			name: "negative MaxHeaderMB resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.MaxHeaderMB = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Expected MaxHeaderMB to be reset to %v, got %v",
						DefaultMaxHeaderMB, c.Server.HTTP.MaxHeaderMB)
				}
			},
		},
		{
			name: "zero MaxHeaderMB resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.MaxHeaderMB = 0
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Expected MaxHeaderMB to be reset to %v, got %v",
						DefaultMaxHeaderMB, c.Server.HTTP.MaxHeaderMB)
				}
			},
		},
		{
			name: "excessive MaxHeaderMB resets to default",
			mutate: func(c *Config) {
				c.Server.HTTP.MaxHeaderMB = 20 // 20 MB exceeds max of 10
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Expected MaxHeaderMB to be reset to %v, got %v",
						DefaultMaxHeaderMB, c.Server.HTTP.MaxHeaderMB)
				}
			},
		},
		{
			name: "valid custom timeouts are preserved",
			mutate: func(c *Config) {
				c.Server.HTTP.ReadHeaderTimeout = 10 * time.Second
				c.Server.HTTP.WriteTimeout = 60 * time.Second
				c.Server.HTTP.IdleTimeout = 180 * time.Second
				c.Server.HTTP.MaxHeaderMB = 5 // 5 MB
			},
			check: func(t *testing.T, c *Config) {
				if c.Server.HTTP.ReadHeaderTimeout != 10*time.Second {
					t.Errorf("Expected ReadHeaderTimeout 10s, got %v", c.Server.HTTP.ReadHeaderTimeout)
				}
				if c.Server.HTTP.WriteTimeout != 60*time.Second {
					t.Errorf("Expected WriteTimeout 60s, got %v", c.Server.HTTP.WriteTimeout)
				}
				if c.Server.HTTP.IdleTimeout != 180*time.Second {
					t.Errorf("Expected IdleTimeout 180s, got %v", c.Server.HTTP.IdleTimeout)
				}
				if c.Server.HTTP.MaxHeaderMB != 5 {
					t.Errorf("Expected MaxHeaderMB 5, got %v", c.Server.HTTP.MaxHeaderMB)
				}
				// Also verify bytes conversion
				if c.MaxHeaderBytes() != 5<<20 {
					t.Errorf("Expected MaxHeaderBytes() %v, got %v", 5<<20, c.MaxHeaderBytes())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			tt.mutate(cfg)
			err := cfg.Validate()
			if err != nil {
				t.Fatalf("Unexpected validation error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(*testing.T, *Config)
	}{
		{
			name: "server port override",
			envVars: map[string]string{
				"GOMDDOC_SERVER_PORT": ":9000",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.Port != ":9000" {
					t.Errorf("Expected Server.Port ':9000', got %q", c.Server.Port)
				}
			},
		},
		{
			name: "server dir override",
			envVars: map[string]string{
				"GOMDDOC_SERVER_DIR": "/custom/dir",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.Dir != "/custom/dir" {
					t.Errorf("Expected Server.Dir '/custom/dir', got %q", c.Server.Dir)
				}
			},
		},
		{
			name: "server dev mode override",
			envVars: map[string]string{
				"GOMDDOC_SERVER_DEV_MODE": "true",
			},
			validate: func(t *testing.T, c *Config) {
				if !c.Server.DevMode {
					t.Errorf("Expected Server.DevMode true, got %v", c.Server.DevMode)
				}
			},
		},
		{
			name: "http shutdown timeout override",
			envVars: map[string]string{
				"GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT": "5s",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.HTTP.ShutdownTimeout.Seconds() != 5 {
					t.Errorf("Expected Server.HTTP.ShutdownTimeout 5s, got %v", c.Server.HTTP.ShutdownTimeout)
				}
			},
		},
		{
			name: "site defaults",
			envVars: map[string]string{
				"GOMDDOC_SITE_DIR_INDEX":     "true",
				"GOMDDOC_SITE_DEFAULT_INDEX": "index.md",
			},
			validate: func(t *testing.T, c *Config) {
				if !c.Site.DirIndex {
					t.Errorf("Expected Site.DirIndex true, got %v", c.Site.DirIndex)
				}
				if c.Site.DefaultIndex != "index.md" {
					t.Errorf("Expected Site.DefaultIndex 'index.md', got %q", c.Site.DefaultIndex)
				}
			},
		},
		{
			name: "all config fields together",
			envVars: map[string]string{
				"GOMDDOC_SERVER_DIR":                   "/test",
				"GOMDDOC_SERVER_PORT":                  ":7777",
				"GOMDDOC_SERVER_HTTP_SHUTDOWN_TIMEOUT": "10s",
				"GOMDDOC_SERVER_DEV_MODE":              "true",
				"GOMDDOC_SITE_DIR_INDEX":               "true",
				"GOMDDOC_SITE_DEFAULT_INDEX":           "HOME.md",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.Dir != "/test" {
					t.Errorf("Expected Server.Dir '/test', got %q", c.Server.Dir)
				}
				if c.Server.Port != ":7777" {
					t.Errorf("Expected Server.Port ':7777', got %q", c.Server.Port)
				}
				if c.Server.HTTP.ShutdownTimeout.Seconds() != 10 {
					t.Errorf("Expected Server.HTTP.ShutdownTimeout 10s, got %v", c.Server.HTTP.ShutdownTimeout)
				}
				if !c.Server.DevMode {
					t.Errorf("Expected Server.DevMode true, got %v", c.Server.DevMode)
				}
				if !c.Site.DirIndex {
					t.Errorf("Expected Site.DirIndex true, got %v", c.Site.DirIndex)
				}
				if c.Site.DefaultIndex != "HOME.md" {
					t.Errorf("Expected Site.DefaultIndex 'HOME.md', got %q", c.Site.DefaultIndex)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg := New()
			// ApplyEnvOverrides applies to the whole struct including Site (via nested walking)
			cfg.ApplyEnvOverrides()

			tt.validate(t, cfg)
		})
	}
}

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

func TestEnvVars(t *testing.T) {
	t.Parallel()

	vars := EnvVars()
	if len(vars) == 0 {
		t.Fatal("EnvVars() returned empty slice")
	}

	// Build a set for lookup
	names := make(map[string]bool)
	for _, v := range vars {
		names[v.Name] = true
	}

	required := []string{
		"GOMDDOC_SERVER_PORT",
		"GOMDDOC_SERVER_DIR",
		"GOMDDOC_SERVER_DEV_MODE",
		"GOMDDOC_SITE_META_TITLE",
		"GOMDDOC_SITE_THEME_NAME",
		"GOMDDOC_SITE_HIGHLIGHTING_THEME",
	}
	for _, name := range required {
		if !names[name] {
			t.Errorf("Expected env var %q in EnvVars() output", name)
		}
	}

	// Verify each var has non-empty Name and Type
	for _, v := range vars {
		if v.Name == "" {
			t.Error("EnvVar has empty Name")
		}
		if v.Type == "" {
			t.Errorf("EnvVar %q has empty Type", v.Name)
		}
	}
}

func TestComputeDynamicDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		title     string
		dir       string
		wantTitle string
	}{
		{
			name:      "sets title from dir basename when empty",
			title:     "",
			dir:       "/some/path/my-project",
			wantTitle: "My-Project",
		},
		{
			name:      "preserves existing title",
			title:     "Custom Title",
			dir:       "/some/path/my-project",
			wantTitle: "Custom Title",
		},
		{
			name:      "handles simple basename",
			title:     "",
			dir:       "/docs",
			wantTitle: "Docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := New()
			cfg.Site.Meta.Title = tt.title
			cfg.Server.Dir = tt.dir
			cfg.ComputeDynamicDefaults()

			if cfg.Site.Meta.Title != tt.wantTitle {
				t.Errorf("ComputeDynamicDefaults() title = %q, want %q", cfg.Site.Meta.Title, tt.wantTitle)
			}
		})
	}
}

func TestApplyEnvOverrides_WalkStructTypes(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(*testing.T, *Config)
	}{
		{
			name: "string field via SiteConfig.ApplyEnvOverrides",
			envVars: map[string]string{
				"GOMDDOC_SITE_DEFAULT_INDEX": "index.md",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Site.DefaultIndex != "index.md" {
					t.Errorf("Site.DefaultIndex = %q, want %q", c.Site.DefaultIndex, "index.md")
				}
			},
		},
		{
			name: "bool field via SiteConfig.ApplyEnvOverrides",
			envVars: map[string]string{
				"GOMDDOC_SITE_DIR_INDEX": "true",
			},
			validate: func(t *testing.T, c *Config) {
				if !c.Site.DirIndex {
					t.Errorf("Site.DirIndex = %v, want true", c.Site.DirIndex)
				}
			},
		},
		{
			name: "nested struct field via SiteConfig.ApplyEnvOverrides",
			envVars: map[string]string{
				"GOMDDOC_SITE_META_TITLE": "Env Title",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Site.Meta.Title != "Env Title" {
					t.Errorf("Site.Meta.Title = %q, want %q", c.Site.Meta.Title, "Env Title")
				}
			},
		},
		{
			name:    "missing env var preserves default",
			envVars: map[string]string{},
			validate: func(t *testing.T, c *Config) {
				if c.Site.DefaultIndex != DefaultIndex {
					t.Errorf("Site.DefaultIndex = %q, want default %q", c.Site.DefaultIndex, DefaultIndex)
				}
			},
		},
		{
			name: "duration field via Config.ApplyEnvOverrides",
			envVars: map[string]string{
				"GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT": "45s",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.HTTP.WriteTimeout != 45*time.Second {
					t.Errorf("Server.HTTP.WriteTimeout = %v, want 45s", c.Server.HTTP.WriteTimeout)
				}
			},
		},
		{
			name: "int field via Config.ApplyEnvOverrides",
			envVars: map[string]string{
				"GOMDDOC_SERVER_HTTP_MAX_HEADER_MB": "5",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.HTTP.MaxHeaderMB != 5 {
					t.Errorf("Server.HTTP.MaxHeaderMB = %d, want 5", c.Server.HTTP.MaxHeaderMB)
				}
			},
		},
		{
			name: "invalid duration is ignored",
			envVars: map[string]string{
				"GOMDDOC_SERVER_HTTP_WRITE_TIMEOUT": "notaduration",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.HTTP.WriteTimeout != DefaultWriteTimeout {
					t.Errorf("Server.HTTP.WriteTimeout = %v, want default %v", c.Server.HTTP.WriteTimeout, DefaultWriteTimeout)
				}
			},
		},
		{
			name: "invalid int is ignored",
			envVars: map[string]string{
				"GOMDDOC_SERVER_HTTP_MAX_HEADER_MB": "notanint",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Server.HTTP.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Server.HTTP.MaxHeaderMB = %d, want default %d", c.Server.HTTP.MaxHeaderMB, DefaultMaxHeaderMB)
				}
			},
		},
		{
			name: "invalid bool is ignored",
			envVars: map[string]string{
				"GOMDDOC_SITE_DIR_INDEX": "notabool",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Site.DirIndex {
					t.Errorf("Site.DirIndex = true, want false (default preserved on invalid input)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg := New()

			// Use SiteConfig.ApplyEnvOverrides for site-level tests,
			// Config.ApplyEnvOverrides for server-level tests
			hasSiteOnly := true
			for k := range tt.envVars {
				if strings.HasPrefix(k, "GOMDDOC_SERVER_") {
					hasSiteOnly = false
					break
				}
			}

			if hasSiteOnly {
				cfg.Site.ApplyEnvOverrides()
			} else {
				cfg.ApplyEnvOverrides()
			}

			tt.validate(t, cfg)
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
		{
			name: "valid editURL with https",
			setup: func(sc *SiteConfig) {
				sc.EditURL = "https://github.com/org/repo/edit/main"
			},
			wantErr: false,
		},
		{
			name: "valid editURL relative path",
			setup: func(sc *SiteConfig) {
				sc.EditURL = "/edit"
			},
			wantErr: false,
		},
		{
			name: "editURL with javascript scheme rejected",
			setup: func(sc *SiteConfig) {
				sc.EditURL = "javascript:alert(1)"
			},
			wantErr: true,
		},
		{
			name: "editURL with data scheme rejected",
			setup: func(sc *SiteConfig) {
				sc.EditURL = "data:text/html,<script>alert(1)</script>"
			},
			wantErr: true,
		},
		{
			name: "empty editURL valid",
			setup: func(sc *SiteConfig) {
				sc.EditURL = ""
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := NewSiteConfig(".")
			tt.setup(&sc)

			err := sc.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}

			if tt.wantFix != nil {
				tt.wantFix(t, &sc)
			}
		})
	}
}

func TestSiteConfig_LoadFromFile_ThemeVars(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	configYAML := `theme:
  name: "default"
  vars:
    primary: "#e63946"
    background: "#fafafa"
    dark-primary: "#ff6b6b"
`
	configPath := filepath.Join(gomddocDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	sc := NewSiteConfig(tmpDir)
	if err := sc.LoadFromFile(tmpDir); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if sc.Theme.Name != "default" {
		t.Errorf("Theme.Name = %q, want %q", sc.Theme.Name, "default")
	}

	wantVars := map[string]string{
		"primary":      "#e63946",
		"background":   "#fafafa",
		"dark-primary": "#ff6b6b",
	}
	for k, want := range wantVars {
		if got := sc.Theme.Vars[k]; got != want {
			t.Errorf("Theme.Vars[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestSiteConfig_LoadFromFile_ThemeVarsNil(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gomddocDir := filepath.Join(tmpDir, ".gomddoc")
	if err := os.MkdirAll(gomddocDir, 0755); err != nil {
		t.Fatalf("Failed to create .gomddoc dir: %v", err)
	}

	configYAML := `theme:
  name: "default"
`
	configPath := filepath.Join(gomddocDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	sc := NewSiteConfig(tmpDir)
	if err := sc.LoadFromFile(tmpDir); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if sc.Theme.Vars != nil {
		t.Errorf("Theme.Vars should be nil when not configured, got %v", sc.Theme.Vars)
	}
}

func TestNewSiteConfig_DotDirectory(t *testing.T) {
	t.Parallel()

	sc := NewSiteConfig(".")
	// filepath.Abs(".") resolves to the actual CWD, so the title should
	// never be "." — it should be the TitleCase of the CWD basename.
	if sc.Meta.Title == "." {
		t.Error("NewSiteConfig(\".\") should not produce title \".\"")
	}
	if sc.Meta.Title == "" {
		t.Error("NewSiteConfig(\".\") should produce a non-empty title")
	}
}
