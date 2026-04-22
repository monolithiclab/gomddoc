package config

import (
	"fmt"
	"log/slog"
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

const (
	// DefaultThemeName is the name of the default theme
	DefaultThemeName = "default"

	// ConfigDirName is the directory name for site configuration
	ConfigDirName = ".gomddoc"

	// ConfigFileName is the name of the site configuration file
	ConfigFileName = "config.yml"
)

// SiteConfig holds the site (content/presentation) configuration
// This IS exposed to templates and IS configurable via .gomddoc/config.yml
type SiteConfig struct {
	Meta  MetaSection  `env:"META" yaml:"meta"`
	Theme ThemeSection `env:"THEME" yaml:"theme"`
}

type MetaSection struct {
	Domain      string `env:"DOMAIN" yaml:"domain"`
	Title       string `env:"TITLE" yaml:"title"`
	Description string `env:"DESCRIPTION" yaml:"description"`
}

type ThemeSection struct {
	Name string `env:"NAME" yaml:"name"`
}

// NewSiteConfig creates a new SiteConfig with default values
// The title defaults to the capitalized basename of the directory
func NewSiteConfig(dir string) *SiteConfig {
	// Compute default title from directory basename
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir // Fallback to original if abs fails
	}
	basename := filepath.Base(absDir)

	// Use shared titleCase function for proper Unicode capitalization
	title := text.TitleCase(basename)

	return &SiteConfig{
		Meta: MetaSection{
			Domain:      "",
			Title:       title,
			Description: "",
		},
		Theme: ThemeSection{
			Name: DefaultThemeName,
		},
	}
}

// LoadFromFile loads site config from .gomddoc/config.yml
func (sc *SiteConfig) LoadFromFile(rootDir string) error {
	path := filepath.Join(rootDir, ConfigDirName, ConfigFileName)

	// Missing file is NOT an error - use defaults
	if _, err := os.Stat(path); os.IsNotExist(err) {
		slog.Debug("No site config file found, using defaults",
			slog.String("path", path))
		return nil
	}

	data, err := os.ReadFile(path) // #nosec G304 - reading config file by design
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	// Parse YAML directly into SiteConfig
	// This will use the yaml struct tags
	if err := yaml.Unmarshal(data, sc); err != nil {
		return fmt.Errorf("parse config.yml: %w", err)
	}

	slog.Info("Loaded site configuration",
		slog.String("path", path),
		slog.String("title", sc.Meta.Title),
		slog.String("theme", sc.Theme.Name))

	return nil
}

// ApplyEnvOverrides applies environment variable overrides using reflection
// Environment variable names are built from:
// - Configurable prefix (default: "GOMDDOC")
// - Struct field env tags in hierarchical order
// Example: GOMDDOC_META_TITLE, GOMDDOC_THEME_NAME
func (sc *SiteConfig) ApplyEnvOverrides() {
	applyEnvOverridesWithPrefix(sc, "GOMDDOC")
}

// Validate validates the site configuration and auto-fixes where appropriate
func (sc *SiteConfig) Validate() error {
	// Validate theme name is not empty
	if sc.Theme.Name == "" {
		sc.Theme.Name = DefaultThemeName // Auto-fix
		slog.Warn("Empty theme name, using default")
	}

	// Validate domain format
	if sc.Meta.Domain != "" {
		// Check for protocol separator
		if strings.Contains(sc.Meta.Domain, "://") {
			return fmt.Errorf("domain should not include protocol: %s", sc.Meta.Domain)
		}
		// Check for path separator
		if strings.Contains(sc.Meta.Domain, "/") {
			return fmt.Errorf("domain should not include path: %s", sc.Meta.Domain)
		}
		// Validate it's a parseable domain
		if _, err := url.Parse("//" + sc.Meta.Domain); err != nil {
			return fmt.Errorf("invalid domain: %w", err)
		}
	}

	return nil
}

// applyEnvOverridesWithPrefix applies env overrides to any struct with env tags
// This is a shared helper used by both Config and SiteConfig
func applyEnvOverridesWithPrefix(target any, prefix string) {
	v := reflect.ValueOf(target).Elem()
	t := v.Type()
	walkStruct(v, t, prefix)
}

// walkStruct recursively walks any struct and applies env overrides
// This is a shared helper used by both Config and SiteConfig
func walkStruct(v reflect.Value, t reflect.Type, prefix string) {
	// Panic recovery for defensive programming against reflection edge cases
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic during config reflection",
				slog.String("prefix", prefix),
				slog.Any("panic", r))
		}
	}()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// More robust validation: check IsValid, CanSet
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		// Get env tag (skip if not present)
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Build full env var name
		envVarName := prefix + "_" + envTag

		slog.Debug("Processing config field",
			slog.String("field", fieldType.Name),
			slog.String("env_var", envVarName),
			slog.String("kind", field.Kind().String()))

		// Handle based on field kind
		switch field.Kind() {
		case reflect.Struct:
			// Recurse into nested struct (builds hierarchical env var names)
			slog.Debug("Recursing into struct field",
				slog.String("field", fieldType.Name),
				slog.String("prefix", envVarName))
			walkStruct(field, field.Type(), envVarName)

		case reflect.Ptr:
			// Handle pointer fields by dereferencing and recursing
			// Skip only the "Site" field since it's configured separately via SiteConfig.ApplyEnvOverrides()
			// But process other pointer fields like "Server" (*ServerConfig)
			if fieldType.Name == "Site" {
				slog.Debug("Skipping Site field (configured separately)",
					slog.String("field", fieldType.Name))
				continue
			}

			// Dereference pointer and recurse if not nil
			if !field.IsNil() {
				slog.Debug("Dereferencing pointer field",
					slog.String("field", fieldType.Name),
					slog.String("prefix", envVarName))
				walkStruct(field.Elem(), field.Elem().Type(), envVarName)
			} else {
				slog.Debug("Skipping nil pointer field",
					slog.String("field", fieldType.Name))
			}

		case reflect.String:
			// Leaf node: check env var and set if present
			if envValue := os.Getenv(envVarName); envValue != "" {
				field.SetString(envValue)
				slog.Debug("Applied env override",
					slog.String("var", envVarName),
					slog.String("value", envValue))
			}

		case reflect.Int, reflect.Int64:
			// Support for int and duration types
			if envValue := os.Getenv(envVarName); envValue != "" {
				// Check if it's a time.Duration field
				if field.Type() == reflect.TypeOf(time.Duration(0)) {
					if duration, err := time.ParseDuration(envValue); err == nil {
						field.SetInt(int64(duration))
						slog.Debug("Applied env override (duration)",
							slog.String("var", envVarName),
							slog.String("value", envValue))
					} else {
						slog.Warn("Invalid duration format",
							slog.String("var", envVarName),
							slog.String("value", envValue))
					}
				} else {
					// Regular int
					if intValue, err := strconv.ParseInt(envValue, 10, 64); err == nil {
						field.SetInt(intValue)
						slog.Debug("Applied env override (int)",
							slog.String("var", envVarName),
							slog.String("value", envValue))
					} else {
						slog.Warn("Invalid int format",
							slog.String("var", envVarName),
							slog.String("value", envValue))
					}
				}
			}

		case reflect.Bool:
			// Support for bool types
			if envValue := os.Getenv(envVarName); envValue != "" {
				if boolValue, err := strconv.ParseBool(envValue); err == nil {
					field.SetBool(boolValue)
					slog.Debug("Applied env override (bool)",
						slog.String("var", envVarName),
						slog.String("value", envValue))
				} else {
					slog.Warn("Invalid bool format",
						slog.String("var", envVarName),
						slog.String("value", envValue))
				}
			}
		}
	}
}
