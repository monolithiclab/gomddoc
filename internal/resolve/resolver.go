// Package resolve maps extensionless URL paths to real file paths and vice versa.
// A PathResolver is built once at startup by walking an fs.FS and is immutable afterward.
package resolve

import (
	"io/fs"
	"log/slog"
	"maps"
	"path"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// RendererCheck reports whether a MIME type has an HTML renderer.
type RendererCheck func(mimeType string) bool

// PathResolver maps extensionless (clean) paths to real file paths and back.
// It is safe for concurrent read access after construction.
type PathResolver struct {
	toReal  map[string]string // extensionless -> real file path
	toClean map[string]string // real file path -> extensionless
}

// Build walks fsys and builds a PathResolver that strips the given extensions
// from file paths whose MIME types pass hasRenderer.
//
// Extensions in stripExts define priority: earlier extensions win collisions.
// If stripExts is empty, an empty resolver is returned.
func Build(fsys fs.FS, stripExts []string, hasRenderer RendererCheck) *PathResolver {
	r := &PathResolver{
		toReal:  make(map[string]string),
		toClean: make(map[string]string),
	}

	if len(stripExts) == 0 {
		return r
	}

	// Build extension priority map: lower index = higher priority.
	extPriority := make(map[string]int, len(stripExts))
	for i, ext := range stripExts {
		extPriority[ext] = i
	}

	// Collect directories for collision detection.
	dirs := make(map[string]bool)
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && p != "." {
			dirs[p] = true
		}
		return nil
	})

	// Track which extension claimed each clean path (for priority comparison).
	claimedBy := make(map[string]string) // clean path -> extension that claimed it

	// Walk again to find candidate files.
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		ext := path.Ext(p)
		if _, ok := extPriority[ext]; !ok {
			return nil // extension not in strip list
		}

		mimeType := negotiate.NormalizeMimeType(negotiate.DetectMIME(p))
		if !hasRenderer(mimeType) {
			return nil // no renderer for this MIME type
		}

		cleanPath := strings.TrimSuffix(p, ext)

		// Collision: another extension already claimed this clean path.
		if claimerExt, claimed := claimedBy[cleanPath]; claimed {
			claimerPri := extPriority[claimerExt]
			newPri := extPriority[ext]
			if newPri >= claimerPri {
				// Current claimer has equal or higher priority; skip.
				slog.Warn("extension collision: skipping file",
					"file", p,
					"clean_path", cleanPath,
					"claimed_by_ext", claimerExt,
				)
				return nil
			}
			// New file has higher priority; replace.
			oldReal := cleanPath + claimerExt
			delete(r.toClean, oldReal)
			slog.Warn("extension collision: replacing mapping",
				"clean_path", cleanPath,
				"old_file", oldReal,
				"new_file", p,
			)
		}

		// Directory collision: file wins, log warning.
		if dirs[cleanPath] {
			slog.Warn("file shadows directory for clean path",
				"file", p,
				"directory", cleanPath,
			)
		}

		r.toReal[cleanPath] = p
		r.toClean[p] = cleanPath
		claimedBy[cleanPath] = ext

		return nil
	})

	return r
}

// Resolve returns the real file path for a clean (extensionless) path.
func (r *PathResolver) Resolve(cleanPath string) (realPath string, found bool) {
	realPath, found = r.toReal[cleanPath]
	return
}

// CleanPath returns the extensionless path for a real file path.
func (r *PathResolver) CleanPath(realPath string) (cleanPath string, found bool) {
	cleanPath, found = r.toClean[realPath]
	return
}

// IsEmpty reports whether the resolver has no mappings.
func (r *PathResolver) IsEmpty() bool {
	return len(r.toReal) == 0
}

// AllMappings returns a copy of the real-to-clean mapping. The copy is
// defensive: callers may freely read or mutate it without affecting the
// resolver's internal state.
func (r *PathResolver) AllMappings() map[string]string {
	return maps.Clone(r.toClean)
}
