package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/monolithiclab/gomddoc/internal/common"
	"github.com/monolithiclab/gomddoc/internal/text"

	"gopkg.in/yaml.v3"
)

// Default values
const (
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultWriteTimeout      = 30 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
	DefaultMaxHeaderMB       = 1
	DefaultPort              = ":8080"
	DefaultShutdownTimeout   = 1 * time.Second
	DefaultIndex             = "README.md"
	DefaultThemeName         = "default"
	DefaultHighlightTheme    = "github"

	// Configurable upper bounds
	MaxReadHeaderTimeout = 1 * time.Minute
	MaxWriteTimeout      = 5 * time.Minute
	MaxIdleTimeout       = 10 * time.Minute
	MaxMaxHeaderMB       = 10

	// Config file constants
	ConfigDirName  = ".gomddoc"
	ConfigFileName = "config.yml"
)

// Config represents the top-level configuration structure (GOMDDOC)
type Config struct {
	Server ServerConfig `env:"SERVER"`
	Site   SiteConfig   `env:"SITE"`
}

// ServerConfig holds server-side settings
type ServerConfig struct {
	Port      string     `env:"PORT"`
	DevMode   bool       `env:"DEV_MODE"`
	Dir       string     `env:"DIR"`
	GitSSHKey string     `env:"GIT_SSH_KEY"`
	HTTP      HTTPConfig `env:"HTTP"`
}

// HTTPConfig holds HTTP server tuning parameters
type HTTPConfig struct {
	ShutdownTimeout   time.Duration `env:"SHUTDOWN_TIMEOUT"`
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT"`
	WriteTimeout      time.Duration `env:"WRITE_TIMEOUT"`
	IdleTimeout       time.Duration `env:"IDLE_TIMEOUT"`
	MaxHeaderMB       int           `env:"MAX_HEADER_MB"`
}

// SiteConfig holds site-specific settings (loadable from file)
type SiteConfig struct {
	DefaultIndex string          `env:"DEFAULT_INDEX" yaml:"default_index"`
	DirIndex     bool            `env:"DIR_INDEX" yaml:"dir_index"`
	EditURL      string          `env:"EDIT_URL" yaml:"edit_url"`
	ColorChips   bool            `env:"COLOR_CHIPS" yaml:"color_chips"`
	Meta         MetaConfig      `env:"META" yaml:"meta"`
	Theme        ThemeConfig     `env:"THEME" yaml:"theme"`
	Highlighting HighlightConfig `env:"HIGHLIGHTING" yaml:"highlighting"`
}

// MetaConfig holds site metadata
type MetaConfig struct {
	Title       string `env:"TITLE" yaml:"title"`
	Description string `env:"DESCRIPTION" yaml:"description"`
	Domain      string `env:"DOMAIN" yaml:"domain"`
}

// ThemeConfig holds theme settings
type ThemeConfig struct {
	Name string `env:"NAME" yaml:"name"`
}

// HighlightConfig holds syntax highlighting settings
type HighlightConfig struct {
	Theme string `env:"THEME" yaml:"theme"`
}

// NewFromServeArgs creates a fully initialized Config from serve command arguments.
// Kong has already resolved flags > env vars > defaults for server-level settings.
// This function handles: build Config -> ComputeDynamicDefaults -> LoadFromFile -> ApplyEnvOverrides (site) -> Validate.
func NewFromServeArgs(dir, port string, devMode bool, gitSSHKey string) (*Config, error) {
	cfg := New()

	// 1. Apply serve command args (already resolved by Kong: flags > env > defaults)
	cfg.Server.Dir = dir
	cfg.Server.Port = port
	cfg.Server.DevMode = devMode
	cfg.Server.GitSSHKey = gitSSHKey

	// 2. Compute derived defaults (like Title from Dir)
	cfg.ComputeDynamicDefaults()

	// 3. Load from File (.gomddoc/config.yml)
	if err := cfg.Site.LoadFromFile(cfg.Server.Dir); err != nil {
		return nil, fmt.Errorf("load config file: %w", err)
	}

	// 4. Re-apply environment overrides for site-level settings (env > file)
	cfg.Site.ApplyEnvOverrides()

	// 5. Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		Server: ServerConfig{
			Port:      DefaultPort,
			DevMode:   false,
			Dir:       ".",
			GitSSHKey: "",
			HTTP: HTTPConfig{
				ShutdownTimeout:   DefaultShutdownTimeout,
				ReadHeaderTimeout: DefaultReadHeaderTimeout,
				WriteTimeout:      DefaultWriteTimeout,
				IdleTimeout:       DefaultIdleTimeout,
				MaxHeaderMB:       DefaultMaxHeaderMB,
			},
		},
		Site: NewSiteConfig("."), // Initialize with default "."
	}
}

// NewSiteConfig creates a new SiteConfig with default values
func NewSiteConfig(dir string) SiteConfig {
	// Compute default title from directory basename
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	basename := filepath.Base(absDir)
	title := text.TitleCase(basename)

	return SiteConfig{
		DefaultIndex: DefaultIndex,
		DirIndex:     false,
		ColorChips:   true,
		Meta: MetaConfig{
			Title: title,
		},
		Theme: ThemeConfig{
			Name: DefaultThemeName,
		},
		Highlighting: HighlightConfig{
			Theme: DefaultHighlightTheme,
		},
	}
}

// LoadFromFile loads site config from .gomddoc/config.yml
func (sc *SiteConfig) LoadFromFile(rootDir string) error {
	path := filepath.Join(rootDir, ConfigDirName, ConfigFileName)

	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Debug("No site config file found, using defaults", slog.String("path", path))
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	// Unmarshal directly into struct.
	// yaml.Unmarshal will match fields with `yaml` tags.
	// Fields without matching keys in YAML will retain their existing values (defaults).
	if err := yaml.Unmarshal(data, sc); err != nil {
		return fmt.Errorf("parse config.yml: %w", err)
	}

	slog.Info("Loaded site configuration",
		slog.String("path", path),
		slog.String("title", sc.Meta.Title),
		slog.String("theme", sc.Theme.Name))

	return nil
}

// ApplyEnvOverrides applies environment variable overrides to Config using reflection
func (c *Config) ApplyEnvOverrides() {
	// GOMDDOC_...
	applyEnvOverridesWithPrefix(c, "GOMDDOC")
}

// ApplyEnvOverrides applies environment variable overrides to SiteConfig using the
// correct prefix GOMDDOC_SITE (useful for standalone usage or testing).
func (sc *SiteConfig) ApplyEnvOverrides() {
	applyEnvOverridesWithPrefix(sc, "GOMDDOC_SITE")
}

// ComputeDynamicDefaults calculates defaults that depend on other values
// e.g. Site Title depends on Dir
func (c *Config) ComputeDynamicDefaults() {
	if c.Site.Meta.Title == "" {
		// Compute default title from directory basename
		absDir, err := filepath.Abs(c.Server.Dir)
		if err != nil {
			absDir = c.Server.Dir
		}
		basename := filepath.Base(absDir)
		c.Site.Meta.Title = text.TitleCase(basename)
	}
}

// Validate validates the configuration values.
func (c *Config) Validate() error {
	return c.validateServer()
}

// Validate site config separately
func (sc *SiteConfig) Validate() error {
	if sc.Theme.Name == "" {
		sc.Theme.Name = DefaultThemeName
		slog.Warn("Empty theme name, using default")
	}

	if sc.Meta.Domain != "" {
		if strings.Contains(sc.Meta.Domain, "://") {
			return fmt.Errorf("domain should not include protocol: %s", sc.Meta.Domain)
		}
		if strings.Contains(sc.Meta.Domain, "/") {
			return fmt.Errorf("domain should not include path: %s", sc.Meta.Domain)
		}
		if _, err := url.Parse("//" + sc.Meta.Domain); err != nil {
			return fmt.Errorf("invalid domain: %w", err)
		}
	}
	return nil
}

func (c *Config) validateServer() error {
	// Validate Port
	_, portStr, err := net.SplitHostPort(c.Server.Port)
	if err != nil {
		return fmt.Errorf("invalid port format: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %s", portStr)
	}

	// Validate Dir (skip if Git URL)
	if !common.IsGitURL(c.Server.Dir) {
		info, err := os.Stat(c.Server.Dir)
		if err != nil {
			return fmt.Errorf("directory validation failed: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", c.Server.Dir)
		}
	}

	// Validate Site
	if err := c.Site.Validate(); err != nil {
		return err
	}

	// Validate HTTP timeouts
	if c.Server.HTTP.ShutdownTimeout < 0 {
		return fmt.Errorf("shutdown timeout cannot be negative: %v", c.Server.HTTP.ShutdownTimeout)
	}
	if c.Server.HTTP.ShutdownTimeout > 60*time.Second {
		slog.Warn("Shutdown timeout is very long",
			slog.Duration("timeout", c.Server.HTTP.ShutdownTimeout),
			slog.Duration("recommended_max", 60*time.Second))
	}

	if c.Server.HTTP.ReadHeaderTimeout <= 0 {
		slog.Warn("ReadHeaderTimeout must be positive, using default",
			slog.Duration("configured", c.Server.HTTP.ReadHeaderTimeout),
			slog.Duration("default", DefaultReadHeaderTimeout))
		c.Server.HTTP.ReadHeaderTimeout = DefaultReadHeaderTimeout
	} else if c.Server.HTTP.ReadHeaderTimeout > MaxReadHeaderTimeout {
		slog.Warn("ReadHeaderTimeout exceeds maximum (60s), using default",
			slog.Duration("configured", c.Server.HTTP.ReadHeaderTimeout),
			slog.Duration("default", DefaultReadHeaderTimeout))
		c.Server.HTTP.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}

	if c.Server.HTTP.WriteTimeout <= 0 {
		slog.Warn("WriteTimeout must be positive, using default",
			slog.Duration("configured", c.Server.HTTP.WriteTimeout),
			slog.Duration("default", DefaultWriteTimeout))
		c.Server.HTTP.WriteTimeout = DefaultWriteTimeout
	} else if c.Server.HTTP.WriteTimeout > MaxWriteTimeout {
		slog.Warn("WriteTimeout exceeds maximum (5m), using default",
			slog.Duration("configured", c.Server.HTTP.WriteTimeout),
			slog.Duration("default", DefaultWriteTimeout))
		c.Server.HTTP.WriteTimeout = DefaultWriteTimeout
	}

	if c.Server.HTTP.IdleTimeout <= 0 {
		slog.Warn("IdleTimeout must be positive, using default",
			slog.Duration("configured", c.Server.HTTP.IdleTimeout),
			slog.Duration("default", DefaultIdleTimeout))
		c.Server.HTTP.IdleTimeout = DefaultIdleTimeout
	} else if c.Server.HTTP.IdleTimeout > MaxIdleTimeout {
		slog.Warn("IdleTimeout exceeds maximum (10m), using default",
			slog.Duration("configured", c.Server.HTTP.IdleTimeout),
			slog.Duration("default", DefaultIdleTimeout))
		c.Server.HTTP.IdleTimeout = DefaultIdleTimeout
	}

	if c.Server.HTTP.MaxHeaderMB <= 0 {
		slog.Warn("MaxHeaderMB must be positive, using default",
			slog.Int("configured_mb", c.Server.HTTP.MaxHeaderMB),
			slog.Int("default_mb", DefaultMaxHeaderMB))
		c.Server.HTTP.MaxHeaderMB = DefaultMaxHeaderMB
	} else if c.Server.HTTP.MaxHeaderMB > MaxMaxHeaderMB {
		slog.Warn("MaxHeaderMB exceeds maximum (10MB), using default",
			slog.Int("configured_mb", c.Server.HTTP.MaxHeaderMB),
			slog.Int("default_mb", DefaultMaxHeaderMB))
		c.Server.HTTP.MaxHeaderMB = DefaultMaxHeaderMB
	}

	return nil
}

// MaxHeaderBytes returns the maximum header size in bytes.
func (c *Config) MaxHeaderBytes() int {
	return c.Server.HTTP.MaxHeaderMB << 20
}

// EnvVar describes an available environment variable.
type EnvVar struct {
	Name         string
	Type         string
	DefaultValue string
}

// EnvVars returns all environment variables recognized by the config system.
func EnvVars() []EnvVar {
	cfg := New()
	var vars []EnvVar
	collectEnvVars(reflect.ValueOf(cfg).Elem(), reflect.TypeFor[Config](), "GOMDDOC", &vars)
	return vars
}

// collectEnvVars recursively walks a struct and collects env var metadata.
func collectEnvVars(v reflect.Value, t reflect.Type, prefix string, vars *[]EnvVar) {
	for i := range t.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		envTag := fieldType.Tag.Get("env")

		kind := field.Kind()
		if kind == reflect.Struct {
			newPrefix := prefix
			if envTag != "" {
				newPrefix = prefix + "_" + envTag
			}
			collectEnvVars(field, field.Type(), newPrefix, vars)
			continue
		}

		if envTag == "" {
			continue
		}

		envVarName := prefix + "_" + envTag
		typeName := field.Type().String()
		if field.Type() == reflect.TypeFor[time.Duration]() {
			typeName = "duration"
		}

		*vars = append(*vars, EnvVar{
			Name:         envVarName,
			Type:         typeName,
			DefaultValue: fmt.Sprintf("%v", field.Interface()),
		})
	}
}

// applyEnvOverridesWithPrefix applies env overrides to any struct with env tags
func applyEnvOverridesWithPrefix(target any, prefix string) {
	v := reflect.ValueOf(target).Elem()
	t := v.Type()
	walkStruct(v, t, prefix)
}

// walkStruct recursively walks any struct and applies env overrides
func walkStruct(v reflect.Value, t reflect.Type, prefix string) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		envTag := fieldType.Tag.Get("env")

		kind := field.Kind()
		if kind == reflect.Struct {
			newPrefix := prefix
			if envTag != "" {
				newPrefix = prefix + "_" + envTag
			}
			walkStruct(field, field.Type(), newPrefix)
			continue
		}

		if kind == reflect.Pointer {
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				newPrefix := prefix
				if envTag != "" {
					newPrefix = prefix + "_" + envTag
				}
				walkStruct(field.Elem(), field.Elem().Type(), newPrefix)
			}
			continue
		}

		if envTag == "" {
			continue
		}

		envVarName := prefix + "_" + envTag
		envValue := os.Getenv(envVarName)
		if envValue == "" {
			continue
		}

		slog.Debug("Applied env override",
			slog.String("var", envVarName),
			slog.String("kind", kind.String()))

		switch kind {
		case reflect.String:
			field.SetString(envValue)

		case reflect.Int, reflect.Int64:
			if field.Type() == reflect.TypeFor[time.Duration]() {
				if duration, err := time.ParseDuration(envValue); err == nil {
					field.SetInt(int64(duration))
				} else {
					slog.Warn("Invalid duration format", slog.String("var", envVarName), text.Safe("value", envValue)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
				}
			} else {
				if intValue, err := strconv.ParseInt(envValue, 10, 64); err == nil {
					field.SetInt(intValue)
				} else {
					slog.Warn("Invalid int format", slog.String("var", envVarName), text.Safe("value", envValue)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
				}
			}

		case reflect.Bool:
			if boolValue, err := strconv.ParseBool(envValue); err == nil {
				field.SetBool(boolValue)
			} else {
				slog.Warn("Invalid bool format", slog.String("var", envVarName), text.Safe("value", envValue)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
			}
		}
	}
}
