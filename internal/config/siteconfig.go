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
func walkStruct(v reflect.Value, t reflect.Type, prefix string) {
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Get env tag
		envTag := fieldType.Tag.Get("env")

		// Handle nested structs and pointers even if they don't have an env tag
		// as their children might have them.
		kind := field.Kind()
		if kind == reflect.Struct {
			newPrefix := prefix
			if envTag != "" {
				newPrefix = prefix + "_" + envTag
			}
			walkStruct(field, field.Type(), newPrefix)
			continue
		}

		if kind == reflect.Ptr {
			// Skip "Site" field to avoid circularity or redundant processing
			if fieldType.Name == "Site" {
				continue
			}

			if !field.IsNil() {
				newPrefix := prefix
				if envTag != "" {
					newPrefix = prefix + "_" + envTag
				}
				walkStruct(field.Elem(), field.Elem().Type(), newPrefix)
			}
			continue
		}

		// Leaf node: check env var and set if present
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
			// Handle time.Duration vs regular int
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				if duration, err := time.ParseDuration(envValue); err == nil {
					field.SetInt(int64(duration))
				} else {
					slog.Warn("Invalid duration format", slog.String("var", envVarName), slog.String("value", envValue))
				}
			} else {
				if intValue, err := strconv.ParseInt(envValue, 10, 64); err == nil {
					field.SetInt(intValue)
				} else {
					slog.Warn("Invalid int format", slog.String("var", envVarName), slog.String("value", envValue))
				}
			}

		case reflect.Bool:
			if boolValue, err := strconv.ParseBool(envValue); err == nil {
				field.SetBool(boolValue)
			} else {
				slog.Warn("Invalid bool format", slog.String("var", envVarName), slog.String("value", envValue))
			}
		}
	}
}
