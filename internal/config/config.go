package config

import (
	"flag"
	"time"
)

// Config holds the application (operational) configuration
// This is NOT exposed to templates and NOT configurable via .gomddoc/config.yml
// Configurable via: environment variables + CLI flags (CLI flags take precedence)
type Config struct {
	// Operational fields (environment variables + CLI flags)
	DefaultIndex    string        `env:"DEFAULT_INDEX"`
	Dir             string        `env:"DIR"`
	Port            string        `env:"PORT"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT"`
	DevMode         bool          `env:"DEV_MODE"`

	// Reference to site configuration (this IS exposed to templates)
	Site *SiteConfig
}

// New creates a new Config with default values
func New() *Config {
	defaultDir := "."
	return &Config{
		DefaultIndex:    "README.md",
		Dir:             defaultDir,
		Port:            ":8080",
		ShutdownTimeout: 1 * time.Second,
		DevMode:         false,
		Site:            NewSiteConfig(defaultDir), // Initialize with default dir for title
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

// Validate validates the configuration values
func (c *Config) Validate() error {
	// Validate application config
	// Site config validation happens separately in SiteConfig.Validate()
	return nil
}
