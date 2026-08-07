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
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// RendererCheck reports whether a MIME type has an HTML renderer.
type RendererCheck func(mimeType string) bool

// PathResolver maps extensionless (clean) paths to real file paths and back.
// It is safe for concurrent read access after construction.
type PathResolver struct {
	toReal  map[string]string // extensionless -> real file path
	toClean map[string]string // real file path -> extensionless
}

// BuildOptions configures Build. StripExtensions and Exclude are both string
// slices, so they are named rather than positional: transposing them would
// silently disable exclusion.
type BuildOptions struct {
	// StripExtensions lists the extensions to strip, in priority order:
	// earlier extensions win collisions. If empty, Build returns an empty
	// resolver.
	StripExtensions []string

	// Exclude lists path.Match patterns whose files get no clean-URL mapping.
	Exclude []string

	// HasRenderer reports whether a MIME type has an HTML renderer. Files
	// whose MIME type fails this check are not mapped.
	HasRenderer RendererCheck
}

// Build walks fsys and builds a PathResolver that strips the configured
// extensions from file paths whose MIME types have an HTML renderer.
//
// Hidden paths and paths matching opts.Exclude are skipped, so an excluded
// file has no clean-URL mapping. This is a security invariant, not an
// optimization: the request-path exclusion middleware cannot see through
// extension stripping, so a mapped exclusion would be reachable at its clean
// URL even though its real path 404s.
func Build(fsys fs.FS, opts BuildOptions) *PathResolver {
	r := &PathResolver{
		toReal:  make(map[string]string),
		toClean: make(map[string]string),
	}

	if len(opts.StripExtensions) == 0 {
		return r
	}

	// Build extension priority map: lower index = higher priority.
	extPriority := make(map[string]int, len(opts.StripExtensions))
	for i, ext := range opts.StripExtensions {
		extPriority[ext] = i
	}

	// Directories seen so far, for collision detection, and the extension that
	// claimed each clean path, for priority comparison.
	//
	// One walk suffices for both: WalkDir visits lexically and cleanPath is
	// always a sibling of p, so a directory "guide" is always visited before
	// the file "guide.md" that would shadow it.
	dirs := make(map[string]bool)
	claimedBy := make(map[string]string)

	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Keep walking — one unreadable subtree should not cost the site
			// every clean URL — but say so. The symptom otherwise is 404s on
			// clean URLs for pages that plainly exist, with nothing in the log
			// to connect them to a permission or I/O failure at startup.
			slog.Warn("clean-URL index: skipping unreadable path",
				"path", p,
				"error", err,
			)
			return nil
		}
		if skip, skipErr := provider.SkipWalkEntry(p, d.Name(), d.IsDir(), opts.Exclude); skip {
			return skipErr
		}
		if d.IsDir() {
			if p != "." {
				dirs[p] = true
			}
			return nil
		}

		ext := path.Ext(p)
		if _, ok := extPriority[ext]; !ok {
			return nil // extension not in strip list
		}

		mimeType := negotiate.NormalizeMimeType(negotiate.DetectMIME(p))
		if !opts.HasRenderer(mimeType) {
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

// PageURLPath returns the absolute URL path a content file is published at:
// the containing directory for a default-index file, the extensionless clean
// path when stripping mapped it, and the real path otherwise.
//
// Serve mode never needs it — it is handed the URL and resolves backwards to a
// file — but anything holding a real path and rendering a link does. See
// "One Derivation of a Page's URL" in docs/decisions.md.
//
// A nil receiver skips the clean-path lookup, so this is safe to call before
// Build.
func (r *PathResolver) PageURLPath(realPath, defaultIndex string) string {
	realPath = strings.TrimPrefix(realPath, "/")

	if IsDefaultIndex(realPath, defaultIndex) {
		dir := path.Dir(realPath)
		if dir == "." {
			return "/"
		}
		return "/" + dir
	}

	if r != nil {
		if clean, found := r.CleanPath(realPath); found {
			return "/" + clean
		}
	}
	return "/" + realPath
}

// IsDefaultIndex reports whether filePath's basename is the site's default
// index file (e.g. "README.md"), which is published at its directory's URL
// rather than at a path of its own.
func IsDefaultIndex(filePath, defaultIndex string) bool {
	return defaultIndex != "" && strings.EqualFold(path.Base(filePath), defaultIndex)
}
