package config

import (
	"os"
	"testing"
	"time"
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

	// Test HTTP server timeout defaults
	if config.ReadHeaderTimeout != DefaultReadHeaderTimeout {
		t.Errorf("Expected ReadHeaderTimeout %v, got %v", DefaultReadHeaderTimeout, config.ReadHeaderTimeout)
	}

	if config.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("Expected WriteTimeout %v, got %v", DefaultWriteTimeout, config.WriteTimeout)
	}

	if config.IdleTimeout != DefaultIdleTimeout {
		t.Errorf("Expected IdleTimeout %v, got %v", DefaultIdleTimeout, config.IdleTimeout)
	}

	if config.MaxHeaderMB != DefaultMaxHeaderMB {
		t.Errorf("Expected MaxHeaderMB %v, got %v", DefaultMaxHeaderMB, config.MaxHeaderMB)
	}

	// Test MaxHeaderBytes() method converts MB to bytes
	expectedBytes := DefaultMaxHeaderMB << 20
	if config.MaxHeaderBytes() != expectedBytes {
		t.Errorf("Expected MaxHeaderBytes() %v, got %v", expectedBytes, config.MaxHeaderBytes())
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

func TestValidate_InvalidPort(t *testing.T) {
	t.Parallel()
	config := New()
	config.Port = "invalid"
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
			cfg.Port = tt.port
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := New()
			cfg.Dir = tt.dir
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
	cfg.Dir = tmpFile.Name()
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
			cfg.ShutdownTimeout = tt.timeout
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
				c.ReadHeaderTimeout = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.ReadHeaderTimeout != DefaultReadHeaderTimeout {
					t.Errorf("Expected ReadHeaderTimeout to be reset to %v, got %v",
						DefaultReadHeaderTimeout, c.ReadHeaderTimeout)
				}
			},
		},
		{
			name: "zero ReadHeaderTimeout resets to default",
			mutate: func(c *Config) {
				c.ReadHeaderTimeout = 0
			},
			check: func(t *testing.T, c *Config) {
				if c.ReadHeaderTimeout != DefaultReadHeaderTimeout {
					t.Errorf("Expected ReadHeaderTimeout to be reset to %v, got %v",
						DefaultReadHeaderTimeout, c.ReadHeaderTimeout)
				}
			},
		},
		{
			name: "excessive ReadHeaderTimeout resets to default",
			mutate: func(c *Config) {
				c.ReadHeaderTimeout = 120 * time.Second
			},
			check: func(t *testing.T, c *Config) {
				if c.ReadHeaderTimeout != DefaultReadHeaderTimeout {
					t.Errorf("Expected ReadHeaderTimeout to be reset to %v, got %v",
						DefaultReadHeaderTimeout, c.ReadHeaderTimeout)
				}
			},
		},
		{
			name: "negative WriteTimeout resets to default",
			mutate: func(c *Config) {
				c.WriteTimeout = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.WriteTimeout != DefaultWriteTimeout {
					t.Errorf("Expected WriteTimeout to be reset to %v, got %v",
						DefaultWriteTimeout, c.WriteTimeout)
				}
			},
		},
		{
			name: "excessive WriteTimeout resets to default",
			mutate: func(c *Config) {
				c.WriteTimeout = 10 * time.Minute
			},
			check: func(t *testing.T, c *Config) {
				if c.WriteTimeout != DefaultWriteTimeout {
					t.Errorf("Expected WriteTimeout to be reset to %v, got %v",
						DefaultWriteTimeout, c.WriteTimeout)
				}
			},
		},
		{
			name: "negative IdleTimeout resets to default",
			mutate: func(c *Config) {
				c.IdleTimeout = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.IdleTimeout != DefaultIdleTimeout {
					t.Errorf("Expected IdleTimeout to be reset to %v, got %v",
						DefaultIdleTimeout, c.IdleTimeout)
				}
			},
		},
		{
			name: "excessive IdleTimeout resets to default",
			mutate: func(c *Config) {
				c.IdleTimeout = 15 * time.Minute
			},
			check: func(t *testing.T, c *Config) {
				if c.IdleTimeout != DefaultIdleTimeout {
					t.Errorf("Expected IdleTimeout to be reset to %v, got %v",
						DefaultIdleTimeout, c.IdleTimeout)
				}
			},
		},
		{
			name: "negative MaxHeaderMB resets to default",
			mutate: func(c *Config) {
				c.MaxHeaderMB = -1
			},
			check: func(t *testing.T, c *Config) {
				if c.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Expected MaxHeaderMB to be reset to %v, got %v",
						DefaultMaxHeaderMB, c.MaxHeaderMB)
				}
			},
		},
		{
			name: "zero MaxHeaderMB resets to default",
			mutate: func(c *Config) {
				c.MaxHeaderMB = 0
			},
			check: func(t *testing.T, c *Config) {
				if c.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Expected MaxHeaderMB to be reset to %v, got %v",
						DefaultMaxHeaderMB, c.MaxHeaderMB)
				}
			},
		},
		{
			name: "excessive MaxHeaderMB resets to default",
			mutate: func(c *Config) {
				c.MaxHeaderMB = 20 // 20 MB exceeds max of 10
			},
			check: func(t *testing.T, c *Config) {
				if c.MaxHeaderMB != DefaultMaxHeaderMB {
					t.Errorf("Expected MaxHeaderMB to be reset to %v, got %v",
						DefaultMaxHeaderMB, c.MaxHeaderMB)
				}
			},
		},
		{
			name: "valid custom timeouts are preserved",
			mutate: func(c *Config) {
				c.ReadHeaderTimeout = 10 * time.Second
				c.WriteTimeout = 60 * time.Second
				c.IdleTimeout = 180 * time.Second
				c.MaxHeaderMB = 5 // 5 MB
			},
			check: func(t *testing.T, c *Config) {
				if c.ReadHeaderTimeout != 10*time.Second {
					t.Errorf("Expected ReadHeaderTimeout 10s, got %v", c.ReadHeaderTimeout)
				}
				if c.WriteTimeout != 60*time.Second {
					t.Errorf("Expected WriteTimeout 60s, got %v", c.WriteTimeout)
				}
				if c.IdleTimeout != 180*time.Second {
					t.Errorf("Expected IdleTimeout 180s, got %v", c.IdleTimeout)
				}
				if c.MaxHeaderMB != 5 {
					t.Errorf("Expected MaxHeaderMB 5, got %v", c.MaxHeaderMB)
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
