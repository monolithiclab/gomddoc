package provider

import (
	"errors"
	"io/fs"
	"log/slog"
	"slices"
	"strings"
)

// Ensure OverlayFS implements these interfaces at compile time
var (
	_ fs.FS         = (*OverlayFS)(nil)
	_ fs.ReadFileFS = (*OverlayFS)(nil)
	_ fs.StatFS     = (*OverlayFS)(nil)
	_ fs.ReadDirFS  = (*OverlayFS)(nil)
)

// OverlayFS implements fs.FS by searching through a chain of filesystems
// Files are returned from the first filesystem that contains them
// Implements: fs.FS, fs.ReadFileFS, fs.StatFS, fs.ReadDirFS
type OverlayFS struct {
	filesystems []fs.FS // Searched in order
}

// NewOverlayFS creates a new overlay filesystem from a variadic list of fs.FS
// Filesystems are searched in the order provided (first has highest priority)
// Example: NewOverlayFS(userOverrides, embeddedAssets)
func NewOverlayFS(filesystems ...fs.FS) *OverlayFS {
	return &OverlayFS{
		filesystems: filesystems,
	}
}

// Open implements fs.FS interface
// Searches through filesystems in order, returning the first match
func (o *OverlayFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fsPathErr(opOpen, name, fs.ErrInvalid)
	}

	var lastErr error

	for _, filesystem := range o.filesystems {
		f, err := filesystem.Open(name)
		if err == nil {
			return f, nil
		}
		lastErr = err
		// Continue on any error (ErrNotExist or otherwise) — lower-priority
		// filesystems may still be able to satisfy the request.
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fsPathErr(opOpen, name, fs.ErrNotExist)
}

// ReadFile implements fs.ReadFileFS interface
// Provides optimized ReadFile implementation by searching through filesystems
func (o *OverlayFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, fsPathErr(opReadFile, name, fs.ErrInvalid)
	}

	var lastErr error

	for i, filesystem := range o.filesystems {
		data, err := fs.ReadFile(filesystem, name)
		if err == nil {
			return data, nil
		}

		lastErr = err

		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		// For non-ErrNotExist errors (permission, I/O), log and return
		slog.Debug("Error reading file in filesystem",
			slog.String("file", name),
			slog.Int("fs_index", i),
			slog.Any("error", err))
		return nil, err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fsPathErr(opReadFile, name, fs.ErrNotExist)
}

// Stat implements fs.StatFS interface
// Returns file info from the first filesystem that contains the file
func (o *OverlayFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fsPathErr(opStat, name, fs.ErrInvalid)
	}

	var lastErr error

	for _, filesystem := range o.filesystems {
		// Try to use StatFS interface if available
		if statFS, ok := filesystem.(fs.StatFS); ok {
			info, err := statFS.Stat(name)
			if err == nil {
				return info, nil
			}
			lastErr = err
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
		} else {
			// Fallback to Open for fs.FS without StatFS
			f, err := filesystem.Open(name)
			if err == nil {
				info, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil {
					return info, nil
				}
				lastErr = statErr
				continue
			}
			lastErr = err
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fsPathErr(opStat, name, fs.ErrNotExist)
}

// ReadDir implements fs.ReadDirFS interface
// Merges directory entries from all filesystems, with first filesystem taking priority
func (o *OverlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, fsPathErr(opReadDir, name, fs.ErrInvalid)
	}

	var lastErr error
	seenEntries := make(map[string]fs.DirEntry)

	// Iterate through filesystems in reverse order so that
	// earlier filesystems (higher priority) overwrite later ones
	for i, filesystem := range slices.Backward(o.filesystems) {

		var entries []fs.DirEntry
		var err error

		// Try to use ReadDirFS interface if available
		if readDirFS, ok := filesystem.(fs.ReadDirFS); ok {
			entries, err = readDirFS.ReadDir(name)
		} else {
			// Fallback to fs.ReadDir for fs.FS without ReadDirFS
			entries, err = fs.ReadDir(filesystem, name)
		}

		if err != nil {
			lastErr = err
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			// For other errors, log and continue trying other filesystems
			slog.Debug("Error reading directory in filesystem",
				slog.String("dir", name),
				slog.Int("fs_index", i),
				slog.Any("error", err))
			continue
		}

		// Merge entries, with earlier filesystems taking priority
		for _, entry := range entries {
			seenEntries[entry.Name()] = entry
		}
	}

	// If we found entries in at least one filesystem, return them sorted by name
	if len(seenEntries) > 0 {
		result := make([]fs.DirEntry, 0, len(seenEntries))
		for _, entry := range seenEntries {
			result = append(result, entry)
		}
		slices.SortFunc(result, func(a, b fs.DirEntry) int {
			return strings.Compare(a.Name(), b.Name())
		})
		return result, nil
	}

	// No entries found in any filesystem
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fsPathErr(opReadDir, name, fs.ErrNotExist)
}
