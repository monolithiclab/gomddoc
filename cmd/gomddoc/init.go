package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// InitCmd holds all flags for the init subcommand.
type InitCmd struct {
	Dir   string `name:"dir" short:"d" default:"." help:"Directory to initialize."`
	Theme string `name:"theme" short:"t" default:"default" help:"Theme to use (default, academic, gitbook, material, midnight, minimal, nord, ocean)."`
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

	content := generateConfigYAML(title, i.Theme)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil { // #nosec G306
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	fmt.Printf("Initialized gomddoc in %s\n", configDir)
	fmt.Println("\nNext steps:")
	fmt.Println("  gomddoc serve    — start the documentation server")
	fmt.Println("  gomddoc preview  — quick preview with auto-open browser")
	return nil
}

// generateConfigYAML produces the default config.yml content.
func generateConfigYAML(title, theme string) string {
	return fmt.Sprintf(`# gomddoc site configuration
# See: https://github.com/monolithiclab/gomddoc

default_index: "%s"
dir_index: false
color_chips: true

meta:
  title: "%s"
  description: ""

theme:
  name: "%s"

highlighting:
  theme: "%s"
`, config.DefaultIndex, title, theme, config.DefaultHighlightTheme)
}
