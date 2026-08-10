// Package config loads and validates site configuration from four sources, in
// descending precedence: CLI flags, environment variables, the site's YAML
// file, and built-in defaults.
//
// Feature flags are the one thing that merges rather than overrides.
// MergeFeatures folds each set of overrides onto the base in turn, so a page's
// frontmatter can switch one feature off without restating the others.
package config

import (
	"errors"
	"fmt"
	"io"
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
	DefaultAutoPortStart     = 8080

	// adminLoopbackHost is the host a host-less --admin-port is bound to.
	adminLoopbackHost = "127.0.0.1"

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
	AdminPort string     `env:"ADMIN_PORT"`
	DevMode   bool       `env:"DEV_MODE"`
	Dir       string     `env:"DIR"`
	Pprof     bool       `env:"PPROF"`
	HTTP      HTTPConfig `env:"HTTP"`
}

// AdminOnMain reports whether the admin endpoints (health, metrics, pprof)
// live on the main listener rather than a dedicated one. Three call sites need
// this answer — route registration, listener construction, and admin-address
// normalization — and they must agree: if one splits the pair while another
// folds it, the process either binds the same address twice or serves nothing
// on the admin port.
func (s ServerConfig) AdminOnMain() bool {
	return s.AdminPort == "" || s.AdminPort == s.Port
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
	DefaultIndex    string          `env:"DEFAULT_INDEX" yaml:"default_index"`
	DirIndex        bool            `env:"DIR_INDEX" yaml:"dir_index"`
	EditURL         string          `env:"EDIT_URL" yaml:"edit_url"`
	Language        string          `env:"LANGUAGE" yaml:"language"`
	Meta            MetaConfig      `env:"META" yaml:"meta"`
	Theme           ThemeConfig     `env:"THEME" yaml:"theme"`
	Highlighting    HighlightConfig `env:"HIGHLIGHTING" yaml:"highlighting"`
	Search          SearchConfig    `env:"SEARCH" yaml:"search"`
	Exclude         []string        `yaml:"exclude"`
	StripExtensions []string        `yaml:"strip_extensions"`
}

// SearchConfig holds search settings
type SearchConfig struct {
	Index bool `env:"INDEX" yaml:"index"`
}

// MetaConfig holds site metadata
type MetaConfig struct {
	Title       string `env:"TITLE" yaml:"title"`
	Description string `env:"DESCRIPTION" yaml:"description"`
	Domain      string `env:"DOMAIN" yaml:"domain"`
	Robots      string `env:"ROBOTS" yaml:"robots"`
}

// ThemeConfig holds theme settings
type ThemeConfig struct {
	Name     string            `env:"NAME" yaml:"name"`
	Vars     map[string]string `yaml:"vars"`
	Features map[string]bool   `env:"FEATURES" yaml:"features"`
}

// HighlightConfig holds syntax highlighting settings
type HighlightConfig struct {
	Theme string `env:"THEME" yaml:"theme"`
}

// ServeArgs holds all serve/preview command arguments that feed into config
// construction. All fields are set before validation.
type ServeArgs struct {
	Dir       string
	Port      string
	AdminPort string
	DevMode   bool
	DirIndex  bool
	Pprof     bool
}

// NewFromServeArgs creates a fully initialized Config from serve command arguments.
// Kong has already resolved flags > env vars > defaults for server-level settings.
func NewFromServeArgs(args ServeArgs) (*Config, error) {
	cfg := New()

	// 1. Env overrides for the whole Config, before the args below so Kong-resolved
	//    flags still beat env: re-reading the environment afterwards would let
	//    GOMDDOC_SERVER_PORT override an explicit --port. The fields that survive this
	//    pass are the ones no flag owns — GOMDDOC_SERVER_HTTP_* and SERVER_DEV_MODE.
	//    The Site half is redundant with step 5, which is the one that matters because
	//    it runs after LoadFromFile; don't delete step 5 in favour of this call.
	cfg.ApplyEnvOverrides()

	// 2. Apply serve command args (already resolved by Kong: flags > env > defaults)
	cfg.Server.Dir = args.Dir
	cfg.Server.Port = args.Port
	cfg.Server.AdminPort = args.AdminPort
	cfg.Server.Pprof = args.Pprof
	cfg.Site.DirIndex = args.DirIndex
	// DevMode is OR'd, not assigned: it is the one field with no flag on any command,
	// so args.DevMode is false for `serve` even when the env var asked for dev mode.
	cfg.Server.DevMode = cfg.Server.DevMode || args.DevMode

	// 3. Compute derived defaults (like Title from Dir)
	cfg.ComputeDynamicDefaults()

	// 4. Load from File (.gomddoc/config.yml)
	if err := cfg.Site.LoadFromFile(cfg.Server.Dir); err != nil {
		return nil, fmt.Errorf("load config file: %w", err)
	}

	// 5. Re-apply environment overrides for site-level settings (env > file)
	cfg.Site.ApplyEnvOverrides()

	slog.Info("Loaded site configuration",
		slog.String("title", cfg.Site.Meta.Title),
		slog.String("theme", cfg.Site.Theme.Name))

	// 6. Normalize (fix up invalid values with sensible defaults)
	cfg.Normalize()

	// 7. Validate (pure checks, no mutations)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// NewFromDir creates a Config from a content directory without server-specific
// settings. Used by the build command and other non-server contexts. Server-shaped
// env vars (GOMDDOC_SERVER_*) are still read into the returned Config; no non-server
// caller reads those fields.
func NewFromDir(dir string) (*Config, error) {
	return NewFromServeArgs(ServeArgs{Dir: dir, Port: ":8080"})
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    DefaultPort,
			DevMode: false,
			Dir:     ".",
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
	return SiteConfig{
		DefaultIndex: DefaultIndex,
		DirIndex:     false,
		Language:     "en-US",
		Meta: MetaConfig{
			Title: titleFromDir(dir),
		},
		Theme: ThemeConfig{
			Name: DefaultThemeName,
		},
		Highlighting: HighlightConfig{
			Theme: DefaultHighlightTheme,
		},
		Search: SearchConfig{
			Index: true,
		},
		StripExtensions: []string{".md"},
	}
}

// LoadFromFile loads site config from .gomddoc/config.yml
func (sc *SiteConfig) LoadFromFile(rootDir string) error {
	path := filepath.Join(rootDir, ConfigDirName, ConfigFileName)

	f, err := os.Open(path) // #nosec G304
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Debug("No site config file found, using defaults", slog.String("path", path))
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Fields with no matching key keep their existing values (defaults), but a
	// key the struct does not define is an error: without KnownFields(true) a
	// misplaced key parses fine and applies nothing. A `site:` wrapper silently
	// discarded meta.domain (no Sitemap: line, no sitemap.xml), and a top-level
	// `features:` or `color_chips:` never reached theme.features.
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(sc); err != nil {
		if errors.Is(err, io.EOF) {
			// Empty or comment-only: a valid way to say "defaults".
			return nil
		}
		return fmt.Errorf("%s: %w", path, err)
	}

	// Decode reads one document. A stray `---` would park the rest of the file
	// in a second one and drop it — the same invisible failure KnownFields is
	// here to prevent, through a different door.
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return fmt.Errorf("%s: only the first YAML document is read; remove the `---` separator at line %d", path, extra.Line)
	}

	return nil
}

// ApplyEnvOverrides applies environment variable overrides to Config using reflection
func (c *Config) ApplyEnvOverrides() {
	// GOMDDOC_...
	applyEnvOverridesWithPrefix(c, "GOMDDOC")
}

// ApplyEnvOverrides applies environment variable overrides to SiteConfig using the
// correct prefix GOMDDOC_SITE. NewFromServeArgs calls this after LoadFromFile, which
// is what puts env above the config file; the whole-Config pass runs before the CLI
// args and cannot serve that role.
func (sc *SiteConfig) ApplyEnvOverrides() {
	applyEnvOverridesWithPrefix(sc, "GOMDDOC_SITE")
}

// titleFromDir derives a human-readable title from a directory path.
func titleFromDir(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	basename := filepath.Base(absDir)
	if basename == "." || basename == string(filepath.Separator) {
		basename = "Documentation"
	}
	return text.TitleCase(basename)
}

// ComputeDynamicDefaults calculates defaults that depend on other values
// e.g. Site Title depends on Dir
func (c *Config) ComputeDynamicDefaults() {
	if c.Site.Meta.Title == "" {
		c.Site.Meta.Title = titleFromDir(c.Server.Dir)
	}
}

// Normalize fixes up configuration values that can be auto-corrected, such as
// resetting out-of-range timeouts to defaults. Must be called before Validate.
func (c *Config) Normalize() {
	c.Site.Normalize()
	c.normalizeHTTP()
	c.normalizeAdminAddr()
}

// normalizeAdminAddr binds a host-less admin address to loopback. The admin
// port carries /metrics and, with --pprof, /debug/pprof/* — heap dumps of
// excluded content and parsed credentials, and a free 30s-CPU-burn in
// /debug/pprof/profile. ":9090" means "every interface" to net.Listen, which is
// not what an operator writing a port number expects. An explicit host
// ("0.0.0.0:9090") is honoured as the deliberate opt-in it is.
//
// Server.Port is deliberately NOT normalized this way: a container publishing
// :8080 must bind the wildcard, so defaulting the content listener to loopback
// would break every containerized deployment.
func (c *Config) normalizeAdminAddr() {
	// Rewriting an address that equals Port would split the pair and start a
	// second listener on an address already in use.
	if c.Server.AdminOnMain() {
		return
	}
	host, port, err := net.SplitHostPort(c.Server.AdminPort)
	if err != nil || host != "" {
		return // malformed: Validate's job. Explicit host: respect it.
	}
	c.Server.AdminPort = net.JoinHostPort(adminLoopbackHost, port)
	slog.Info("Admin port bound to loopback; set an explicit host to widen",
		slog.String("addr", c.Server.AdminPort))
}

// Normalize fixes up site config values that can be auto-corrected.
func (sc *SiteConfig) Normalize() {
	if sc.Theme.Name == "" {
		sc.Theme.Name = DefaultThemeName
		slog.Warn("Empty theme name, using default")
	}
}

func (c *Config) normalizeHTTP() {
	h := &c.Server.HTTP

	if h.ReadHeaderTimeout <= 0 {
		slog.Warn("ReadHeaderTimeout must be positive, using default",
			slog.Duration("configured", h.ReadHeaderTimeout),
			slog.Duration("default", DefaultReadHeaderTimeout))
		h.ReadHeaderTimeout = DefaultReadHeaderTimeout
	} else if h.ReadHeaderTimeout > MaxReadHeaderTimeout {
		slog.Warn("ReadHeaderTimeout exceeds maximum (60s), using default",
			slog.Duration("configured", h.ReadHeaderTimeout),
			slog.Duration("default", DefaultReadHeaderTimeout))
		h.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}

	if h.WriteTimeout <= 0 {
		slog.Warn("WriteTimeout must be positive, using default",
			slog.Duration("configured", h.WriteTimeout),
			slog.Duration("default", DefaultWriteTimeout))
		h.WriteTimeout = DefaultWriteTimeout
	} else if h.WriteTimeout > MaxWriteTimeout {
		slog.Warn("WriteTimeout exceeds maximum (5m), using default",
			slog.Duration("configured", h.WriteTimeout),
			slog.Duration("default", DefaultWriteTimeout))
		h.WriteTimeout = DefaultWriteTimeout
	}

	if h.IdleTimeout <= 0 {
		slog.Warn("IdleTimeout must be positive, using default",
			slog.Duration("configured", h.IdleTimeout),
			slog.Duration("default", DefaultIdleTimeout))
		h.IdleTimeout = DefaultIdleTimeout
	} else if h.IdleTimeout > MaxIdleTimeout {
		slog.Warn("IdleTimeout exceeds maximum (10m), using default",
			slog.Duration("configured", h.IdleTimeout),
			slog.Duration("default", DefaultIdleTimeout))
		h.IdleTimeout = DefaultIdleTimeout
	}

	if h.MaxHeaderMB <= 0 {
		slog.Warn("MaxHeaderMB must be positive, using default",
			slog.Int("configured_mb", h.MaxHeaderMB),
			slog.Int("default_mb", DefaultMaxHeaderMB))
		h.MaxHeaderMB = DefaultMaxHeaderMB
	} else if h.MaxHeaderMB > MaxMaxHeaderMB {
		slog.Warn("MaxHeaderMB exceeds maximum (10MB), using default",
			slog.Int("configured_mb", h.MaxHeaderMB),
			slog.Int("default_mb", DefaultMaxHeaderMB))
		h.MaxHeaderMB = DefaultMaxHeaderMB
	}
}

// Validate validates the configuration values.
func (c *Config) Validate() error {
	return c.validateServer()
}

// Validate validates site config separately. Does not mutate; call Normalize first.
func (sc *SiteConfig) Validate() error {
	if sc.DefaultIndex == "" {
		return fmt.Errorf("default_index must not be empty")
	}

	if sc.EditURL != "" {
		u, err := url.Parse(sc.EditURL)
		if err != nil {
			return fmt.Errorf("invalid edit_url: %w", err)
		}
		if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("edit_url must use http or https scheme, got %q", u.Scheme)
		}
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

	if err := ValidateFeatureKeys(sc.Theme.Features); err != nil {
		return err
	}

	for _, ext := range sc.StripExtensions {
		if ext == "" || ext[0] != '.' {
			return fmt.Errorf("strip_extensions: %q must start with a dot", ext)
		}
	}

	return nil
}

// IsGitURL checks if a string is a Git URL.
// Returns true for URLs starting with git://, git+ssh://, or git+https://.
func IsGitURL(s string) bool {
	return strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "git+ssh://") ||
		strings.HasPrefix(s, "git+https://")
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

	// Validate AdminPort (if set)
	if c.Server.AdminPort != "" {
		_, adminPortStr, err := net.SplitHostPort(c.Server.AdminPort)
		if err != nil {
			return fmt.Errorf("invalid admin port format: %w", err)
		}
		adminPort, err := strconv.Atoi(adminPortStr)
		if err != nil || adminPort < 1 || adminPort > 65535 {
			return fmt.Errorf("admin port must be between 1 and 65535, got: %s", adminPortStr)
		}
	}

	// Validate Dir (skip if Git URL)
	if !IsGitURL(c.Server.Dir) {
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

	// Validate HTTP timeouts (Normalize should be called first to fix up values)
	if c.Server.HTTP.ShutdownTimeout < 0 {
		return fmt.Errorf("shutdown timeout cannot be negative: %v", c.Server.HTTP.ShutdownTimeout)
	}
	if c.Server.HTTP.ShutdownTimeout > 60*time.Second {
		slog.Warn("Shutdown timeout is very long",
			slog.Duration("timeout", c.Server.HTTP.ShutdownTimeout),
			slog.Duration("recommended_max", 60*time.Second))
	}

	return nil
}

// MaxHeaderBytes returns the maximum header size in bytes.
func (h HTTPConfig) MaxHeaderBytes() int {
	return h.MaxHeaderMB << 20
}

// MaxHeaderBytes returns the maximum header size in bytes.
func (c *Config) MaxHeaderBytes() int {
	return c.Server.HTTP.MaxHeaderBytes()
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
	for i := range v.NumField() {
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

		if kind == reflect.Map && field.Type() == reflect.TypeFor[map[string]bool]() {
			if envTag == "" {
				continue
			}
			mapPrefix := prefix + "_" + envTag + "_"
			for _, env := range os.Environ() {
				if !strings.HasPrefix(env, mapPrefix) {
					continue
				}
				parts := strings.SplitN(env, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.ToLower(strings.TrimPrefix(parts[0], mapPrefix))
				if boolValue, err := strconv.ParseBool(parts[1]); err == nil {
					if field.IsNil() {
						field.Set(reflect.MakeMap(field.Type()))
					}
					field.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(boolValue))
					slog.Debug("Applied env override",
						slog.String("var", parts[0]),
						slog.String("kind", "map[string]bool"))
				}
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
