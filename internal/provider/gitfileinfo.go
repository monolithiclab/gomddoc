package provider

import (
	"io/fs"
	"time"
)

// gitFileInfo implements fs.FileInfo for Git objects.
// It provides file metadata based on Git tree entries and commit timestamps.
type gitFileInfo struct {
	name    string      // Base name of the file
	size    int64       // Size of the file in bytes
	mode    fs.FileMode // File mode and permission bits
	modTime time.Time   // Modification time (commit timestamp)
	isDir   bool        // True if this is a directory
}

// Ensure gitFileInfo implements fs.FileInfo
var _ fs.FileInfo = (*gitFileInfo)(nil)

// Name returns the base name of the file.
func (fi *gitFileInfo) Name() string { return fi.name }

// Size returns the length in bytes for regular files.
// For directories, this returns 0.
func (fi *gitFileInfo) Size() int64 { return fi.size }

// Mode returns the file mode bits.
// For directories, this includes fs.ModeDir.
func (fi *gitFileInfo) Mode() fs.FileMode { return fi.mode }

// ModTime returns the modification time.
// For Git files, this is the commit timestamp.
func (fi *gitFileInfo) ModTime() time.Time { return fi.modTime }

// IsDir reports whether the file is a directory.
func (fi *gitFileInfo) IsDir() bool { return fi.isDir }

// Sys returns nil as there is no underlying system data.
func (fi *gitFileInfo) Sys() any { return nil }

// gitDirEntry implements fs.DirEntry for Git tree entries.
// It represents an entry in a Git directory listing.
type gitDirEntry struct {
	name     string      // Name of the entry
	isDir    bool        // True if this is a directory
	fileMode fs.FileMode // File mode and permission bits
	size     int64       // Size for files, 0 for directories
	modTime  time.Time   // Modification time (commit timestamp)
}

// Ensure gitDirEntry implements fs.DirEntry
var _ fs.DirEntry = (*gitDirEntry)(nil)

// Name returns the name of the file or directory.
func (e *gitDirEntry) Name() string { return e.name }

// IsDir reports whether the entry is a directory.
func (e *gitDirEntry) IsDir() bool { return e.isDir }

// Type returns the type bits for the entry (extracted from the file mode).
func (e *gitDirEntry) Type() fs.FileMode { return e.fileMode.Type() }

// Info returns the FileInfo for the file or directory.
func (e *gitDirEntry) Info() (fs.FileInfo, error) {
	return &gitFileInfo{
		name:    e.name,
		size:    e.size,
		mode:    e.fileMode,
		modTime: e.modTime,
		isDir:   e.isDir,
	}, nil
}
