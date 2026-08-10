// Package assets layers a site's own files over the ones embedded in the
// binary, so a site can override a theme file without forking the theme.
//
// Resolution is whole-file, and happens in provider.OverlayFS: Open returns
// the copy from the highest-priority layer that has the name, so a site
// dropping its own locales/en-US.yml replaces the baseline outright rather
// than merging key by key. ReadDir is the exception — it unions the layers,
// highest priority winning per name.
package assets

import (
	"io/fs"
	"log/slog"
	"path"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// BuildStaticFS creates a layered filesystem for serving static assets.
// It combines up to three sources (highest priority first):
//  1. Site-level: .gomddoc/static/ (user overrides)
//  2. Theme-level: theme's static/ directory
//  3. Shared: assets/shared/static/ (cross-theme resources)
//
// Returns nil if no static directories exist in any layer.
func BuildStaticFS(overlayFS fs.FS, themeName string) fs.FS {
	var layers []fs.FS
	candidates := []string{
		"static",
		path.Join("assets", "themes", themeName, "static"),
		path.Join("assets", "shared", "static"),
	}
	for _, dir := range candidates {
		if sub := subIfDir(overlayFS, dir); sub != nil {
			layers = append(layers, sub)
		}
	}
	if len(layers) == 0 {
		return nil
	}
	return provider.NewOverlayFS(layers...)
}

// subIfDir returns an fs.Sub of overlayFS at dir if dir exists and is a directory.
// Returns nil if dir does not exist or is not a directory.
func subIfDir(overlayFS fs.FS, dir string) fs.FS {
	info, err := fs.Stat(overlayFS, dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	sub, err := fs.Sub(overlayFS, dir)
	if err != nil {
		return nil
	}
	return sub
}

// BuildFS creates the asset filesystem used for template resolution.
// If the content root contains a .gomddoc/ directory, it is layered
// on top of the embedded assets so that local themes take priority.
// This works for both filesystem and Git-backed content providers.
func BuildFS(contentRoot fs.FS, embedded fs.FS) fs.FS {
	info, err := fs.Stat(contentRoot, config.ConfigDirName)
	if err != nil || !info.IsDir() {
		return embedded
	}
	sub, err := fs.Sub(contentRoot, config.ConfigDirName)
	if err != nil {
		return embedded
	}
	slog.Info("Using local asset overrides", slog.String("path", config.ConfigDirName))
	return provider.NewOverlayFS(sub, embedded)
}
