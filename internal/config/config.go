package config

import (
	"flag"
	"time"
)

// Config holds the application configuration
type Config struct {
	DefaultIndex    string
	Dir             string
	Port            string
	ShutdownTimeout time.Duration
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		DefaultIndex:    "README.md",
		Dir:             ".",
		Port:            ":8080",
		ShutdownTimeout: 1 * time.Second,
	}
}

// ParseFlags parses command line flags and updates the configuration
func (c *Config) ParseFlags() {
	flag.StringVar(&c.Dir, "d", c.Dir, "Markdown directory")
	flag.StringVar(&c.Port, "p", c.Port, "HTTP port (default: 8080)")
	flag.Parse()
}

// Validate validates the configuration values
func (c *Config) Validate() error {
	// Currently no validation needed, but this method provides
	// a hook for future configuration validation
	return nil
}
