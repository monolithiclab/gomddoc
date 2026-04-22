package assets

import (
	"io/fs"
	"log/slog"
	"path"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// NewOverlayFS creates a new overlay filesystem from a variadic list of fs.FS
// Filesystems are searched in the order provided (first has highest priority)
func NewOverlayFS(filesystems ...fs.FS) fs.FS {
	return provider.NewOverlayFS(filesystems...)
}

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
