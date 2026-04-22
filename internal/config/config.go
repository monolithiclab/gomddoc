package config

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/monolithiclab/gomddoc/internal/common"
)

// Default values for HTTP server timeouts
const (
	DefaultReadHeaderTimeout = 5 * time.Second   // Time to read request headers
	DefaultWriteTimeout      = 30 * time.Second  // Time to write response
	DefaultIdleTimeout       = 120 * time.Second // Time to keep idle connections open
	DefaultMaxHeaderMB       = 1                 // 1 MB max header size

	// Configurable upper bounds
	MaxReadHeaderTimeout = 1 * time.Minute  // Maximum configurable time to read request headers
	MaxWriteTimeout      = 5 * time.Minute  // Maximum configurable time to write response
	MaxIdleTimeout       = 10 * time.Minute // Maximum configurable time to keep idle connections open
	MaxMaxHeaderMB       = 10               // Maximum configurable header size
)

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	DefaultIndex string `env:"DEFAULT_INDEX"` // Default index file name (e.g., "README.md")
	DirIndex     bool   `env:"DIR_INDEX"`     // Enable directory listing (default: false, secure by default)
}

// Config holds the application (operational) configuration
// This is NOT exposed to templates and NOT configurable via .gomddoc/config.yml
// Configurable via: environment variables + CLI flags (CLI flags take precedence)
type Config struct {
	// Operational fields (environment variables + CLI flags)
	Dir             string        `env:"DIR"`
	Port            string        `env:"PORT"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT"`
	DevMode         bool          `env:"DEV_MODE"`

	// HTTP server timeouts (defense against slow clients and resource exhaustion)
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT"` // Time to read request headers
	WriteTimeout      time.Duration `env:"WRITE_TIMEOUT"`       // Time to write response
	IdleTimeout       time.Duration `env:"IDLE_TIMEOUT"`        // Time to keep idle connections open
	MaxHeaderMB       int           `env:"MAX_HEADER_MB"`       // Maximum size of request headers in MB

	// Server configuration
	Server *ServerConfig `env:"SERVER"`

	// Reference to site configuration (this IS exposed to templates)
	Site *SiteConfig
}

// New creates a new Config with default values
func New() *Config {
	defaultDir := "."
	return &Config{
		Dir:               defaultDir,
		Port:              ":8080",
		ShutdownTimeout:   1 * time.Second,
		DevMode:           false,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
		MaxHeaderMB:       DefaultMaxHeaderMB,
		Server: &ServerConfig{
			DefaultIndex: "README.md",
			DirIndex:     false, // Secure by default
		},
		Site: NewSiteConfig(defaultDir), // Initialize with default dir for title
	}
}

// ApplyEnvOverrides applies environment variable overrides to Config using reflection
// This is called before ParseFlags, so CLI flags take precedence over env vars
func (c *Config) ApplyEnvOverrides() {
	applyEnvOverridesWithPrefix(c, "GOMDDOC")
}

// ParseFlags parses command line flags and updates the configuration
// Call this AFTER ApplyEnvOverrides so CLI flags take precedence
func (c *Config) ParseFlags() {
	flag.StringVar(&c.Dir, "d", c.Dir, "Markdown directory")
	flag.StringVar(&c.Port, "p", c.Port, "HTTP port (default: 8080)")
	flag.BoolVar(&c.DevMode, "dev", c.DevMode, "Enable development mode (hot reload)")
	flag.Parse()
}

// Validate validates the configuration values.
// Critical configuration errors (port, directory) cause validation failure.
// For timeout values, invalid values trigger a warning and are reset to defaults.
func (c *Config) Validate() error {
	// Validate port format and range
	_, portStr, err := net.SplitHostPort(c.Port)
	if err != nil {
		return fmt.Errorf("invalid port format: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %s", portStr)
	}

	// Skip filesystem validation for Git URLs
	// Git URL validation happens at provider construction time
	if !common.IsGitURL(c.Dir) {
		// Validate directory exists and is accessible
		info, err := os.Stat(c.Dir)
		if err != nil {
			return fmt.Errorf("directory validation failed: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", c.Dir)
		}
	}

	// Validate shutdown timeout (negative is invalid, warn if too long)
	if c.ShutdownTimeout < 0 {
		return fmt.Errorf("shutdown timeout cannot be negative: %v", c.ShutdownTimeout)
	}
	if c.ShutdownTimeout > 60*time.Second {
		slog.Warn("Shutdown timeout is very long",
			slog.Duration("timeout", c.ShutdownTimeout),
			slog.Duration("recommended_max", 60*time.Second))
	}

	// Validate ReadHeaderTimeout (must be positive, max 60s)
	if c.ReadHeaderTimeout <= 0 {
		slog.Warn("ReadHeaderTimeout must be positive, using default",
			slog.Duration("configured", c.ReadHeaderTimeout),
			slog.Duration("default", DefaultReadHeaderTimeout))
		c.ReadHeaderTimeout = DefaultReadHeaderTimeout
	} else if c.ReadHeaderTimeout > MaxReadHeaderTimeout {
		slog.Warn("ReadHeaderTimeout exceeds maximum (60s), using default",
			slog.Duration("configured", c.ReadHeaderTimeout),
			slog.Duration("default", DefaultReadHeaderTimeout))
		c.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}

	// Validate WriteTimeout (must be positive, max 5 minutes)
	if c.WriteTimeout <= 0 {
		slog.Warn("WriteTimeout must be positive, using default",
			slog.Duration("configured", c.WriteTimeout),
			slog.Duration("default", DefaultWriteTimeout))
		c.WriteTimeout = DefaultWriteTimeout
	} else if c.WriteTimeout > MaxWriteTimeout {
		slog.Warn("WriteTimeout exceeds maximum (5m), using default",
			slog.Duration("configured", c.WriteTimeout),
			slog.Duration("default", DefaultWriteTimeout))
		c.WriteTimeout = DefaultWriteTimeout
	}

	// Validate IdleTimeout (must be positive, max 10 minutes)
	if c.IdleTimeout <= 0 {
		slog.Warn("IdleTimeout must be positive, using default",
			slog.Duration("configured", c.IdleTimeout),
			slog.Duration("default", DefaultIdleTimeout))
		c.IdleTimeout = DefaultIdleTimeout
	} else if c.IdleTimeout > MaxIdleTimeout {
		slog.Warn("IdleTimeout exceeds maximum (10m), using default",
			slog.Duration("configured", c.IdleTimeout),
			slog.Duration("default", DefaultIdleTimeout))
		c.IdleTimeout = DefaultIdleTimeout
	}

	// Validate MaxHeaderMB (must be positive, max 10MB)
	if c.MaxHeaderMB <= 0 {
		slog.Warn("MaxHeaderMB must be positive, using default",
			slog.Int("configured_mb", c.MaxHeaderMB),
			slog.Int("default_mb", DefaultMaxHeaderMB))
		c.MaxHeaderMB = DefaultMaxHeaderMB
	} else if c.MaxHeaderMB > MaxMaxHeaderMB {
		slog.Warn("MaxHeaderMB exceeds maximum (10MB), using default",
			slog.Int("configured_mb", c.MaxHeaderMB),
			slog.Int("default_mb", DefaultMaxHeaderMB))
		c.MaxHeaderMB = DefaultMaxHeaderMB
	}

	return nil
}

// MaxHeaderBytes returns the maximum header size in bytes.
// This converts MaxHeaderMB (megabytes) to bytes for use with http.Server.
func (c *Config) MaxHeaderBytes() int {
	return c.MaxHeaderMB << 20
}
