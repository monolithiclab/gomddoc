package config

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()
	config := New()

	if config.Dir != "." {
		t.Errorf("Expected default dir '.', got %q", config.Dir)
	}

	if config.Port != ":8080" {
		t.Errorf("Expected default port ':8080', got %q", config.Port)
	}

	// Test new ServerConfig
	if config.Server == nil {
		t.Fatal("Expected Server to be initialized")
	}

	if config.Server.DefaultIndex != "README.md" {
		t.Errorf("Expected server default index 'README.md', got %q", config.Server.DefaultIndex)
	}

	if config.Server.DirIndex != false {
		t.Errorf("Expected server dir index false (secure by default), got %v", config.Server.DirIndex)
	}

	if config.ShutdownTimeout.Seconds() != 1 {
		t.Errorf("Expected shutdown timeout 1s, got %v", config.ShutdownTimeout)
	}
}

func TestParseFlags(t *testing.T) {
	// Save original args to restore later
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test default values
	os.Args = []string{"cmd"}
	config := New()
	config.ParseFlags()

	if config.Dir != "." {
		t.Errorf("Expected default dir '.', got %q", config.Dir)
	}

	if config.Port != ":8080" {
		t.Errorf("Expected default port ':8080', got %q", config.Port)
	}
}

func TestValidate(t *testing.T) {
	config := New()
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(*testing.T, *Config)
	}{
		{
			name: "top-level string fields",
			envVars: map[string]string{
				"GOMDDOC_DIR":  "/custom/dir",
				"GOMDDOC_PORT": ":9000",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Dir != "/custom/dir" {
					t.Errorf("Expected Dir '/custom/dir', got %q", c.Dir)
				}
				if c.Port != ":9000" {
					t.Errorf("Expected Port ':9000', got %q", c.Port)
				}
			},
		},
		{
			name: "top-level bool fields",
			envVars: map[string]string{
				"GOMDDOC_DEV_MODE": "true",
			},
			validate: func(t *testing.T, c *Config) {
				if !c.DevMode {
					t.Errorf("Expected DevMode true, got %v", c.DevMode)
				}
			},
		},
		{
			name: "top-level duration fields",
			envVars: map[string]string{
				"GOMDDOC_SHUTDOWN_TIMEOUT": "5s",
			},
			validate: func(t *testing.T, c *Config) {
				if c.ShutdownTimeout.Seconds() != 5 {
					t.Errorf("Expected ShutdownTimeout 5s, got %v", c.ShutdownTimeout)
				}
			},
		},
		{
			name: "nested pointer struct fields (ServerConfig)",
			envVars: map[string]string{
				"GOMDDOC_SERVER_DIR_INDEX":     "true",
				"GOMDDOC_SERVER_DEFAULT_INDEX": "index.md",
			},
			validate: func(t *testing.T, c *Config) {
				if !c.Server.DirIndex {
					t.Errorf("Expected Server.DirIndex true, got %v", c.Server.DirIndex)
				}
				if c.Server.DefaultIndex != "index.md" {
					t.Errorf("Expected Server.DefaultIndex 'index.md', got %q", c.Server.DefaultIndex)
				}
			},
		},
		{
			name: "all config fields together",
			envVars: map[string]string{
				"GOMDDOC_DIR":                  "/test",
				"GOMDDOC_PORT":                 ":7777",
				"GOMDDOC_SHUTDOWN_TIMEOUT":     "10s",
				"GOMDDOC_DEV_MODE":             "true",
				"GOMDDOC_SERVER_DIR_INDEX":     "true",
				"GOMDDOC_SERVER_DEFAULT_INDEX": "HOME.md",
			},
			validate: func(t *testing.T, c *Config) {
				if c.Dir != "/test" {
					t.Errorf("Expected Dir '/test', got %q", c.Dir)
				}
				if c.Port != ":7777" {
					t.Errorf("Expected Port ':7777', got %q", c.Port)
				}
				if c.ShutdownTimeout.Seconds() != 10 {
					t.Errorf("Expected ShutdownTimeout 10s, got %v", c.ShutdownTimeout)
				}
				if !c.DevMode {
					t.Errorf("Expected DevMode true, got %v", c.DevMode)
				}
				if !c.Server.DirIndex {
					t.Errorf("Expected Server.DirIndex true, got %v", c.Server.DirIndex)
				}
				if c.Server.DefaultIndex != "HOME.md" {
					t.Errorf("Expected Server.DefaultIndex 'HOME.md', got %q", c.Server.DefaultIndex)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test env vars
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Create config and apply env overrides
			cfg := New()
			cfg.ApplyEnvOverrides()

			// Validate
			tt.validate(t, cfg)
		})
	}
}
