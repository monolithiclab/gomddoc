package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/text"
	"gopkg.in/yaml.v3"
)

// InitCmd holds all flags for the init subcommand.
type InitCmd struct {
	Dir   string `arg:"" optional:"" default:"." help:"Directory to initialize."`
	Theme string `name:"theme" short:"t" default:"default" help:"Theme name."`
}

// Run executes the init command.
func (i *InitCmd) Run() error {
	configPath := filepath.Join(i.Dir, config.ConfigDirName, config.ConfigFileName)

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("already initialized: %s exists", configPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("check config: %w", err)
	}

	configDir := filepath.Join(i.Dir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("create %s: %w", configDir, err)
	}

	absDir, err := filepath.Abs(i.Dir)
	if err != nil {
		absDir = i.Dir
	}
	title := text.TitleCase(filepath.Base(absDir))

	content, marshalErr := generateConfigYAML(title, i.Theme)
	if marshalErr != nil {
		return fmt.Errorf("generate config: %w", marshalErr)
	}

	if err := os.WriteFile(configPath, content, 0644); err != nil { // #nosec G306
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	fmt.Printf("Initialized gomddoc in %s\n", configDir)
	fmt.Println("\nNext steps:")
	fmt.Println("  gomddoc serve    — start the documentation server")
	fmt.Println("  gomddoc preview  — quick preview with auto-open browser")
	return nil
}

// initConfig is the YAML structure for the generated config file.
// Separate from config.SiteConfig to control field order and include only init-relevant fields.
type initConfig struct {
	DefaultIndex string          `yaml:"default_index"`
	DirIndex     bool            `yaml:"dir_index"`
	ColorChips   bool            `yaml:"color_chips"`
	Meta         initMetaConfig  `yaml:"meta"`
	Theme        initThemeConfig `yaml:"theme"`
	Highlighting initHLConfig    `yaml:"highlighting"`
}

type initMetaConfig struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

type initThemeConfig struct {
	Name string `yaml:"name"`
}

type initHLConfig struct {
	Theme string `yaml:"theme"`
}

const configHeader = "# gomddoc site configuration\n# See: https://github.com/monolithiclab/gomddoc\n\n"

// generateConfigYAML produces the default config.yml content.
func generateConfigYAML(title, theme string) ([]byte, error) {
	cfg := initConfig{
		DefaultIndex: config.DefaultIndex,
		ColorChips:   true,
		Meta:         initMetaConfig{Title: title},
		Theme:        initThemeConfig{Name: theme},
		Highlighting: initHLConfig{Theme: config.DefaultHighlightTheme},
	}
	body, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return append([]byte(configHeader), body...), nil
}
