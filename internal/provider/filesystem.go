package provider

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path"

	"github.com/monolithiclab/gomddoc/internal/common"
)

// FilesystemProvider implements Provider for local filesystem access
type FilesystemProvider struct {
	root         fs.StatFS
	defaultIndex string
	dirIndex     bool
}

// NewFilesystemProvider creates a new filesystem provider with path traversal protection.
// The provider uses os.DirFS for secure filesystem access within the specified directory.
//
// Parameters:
//   - dir: Root directory to serve files from
//   - defaultIndex: Default index file name (e.g., "README.md")
//   - dirIndex: Enable directory listing generation (false = secure by default)
func NewFilesystemProvider(dir, defaultIndex string, dirIndex bool) (*FilesystemProvider, error) {
	fsys := os.DirFS(dir)
	return NewFilesystemProviderFromFS(fsys, defaultIndex, dirIndex)
}

// NewFilesystemProviderFromFS creates a new filesystem provider from an existing fs.FS.
// This is useful for testing with in-memory filesystems or using custom fs implementations.
func NewFilesystemProviderFromFS(fsys fs.FS, defaultIndex string, dirIndex bool) (*FilesystemProvider, error) {
	if defaultIndex == "" {
		return nil, fmt.Errorf("defaultIndex must not be empty")
	}

	// Type assertion to fs.StatFS (needed for Stat() method)
	statFS, ok := fsys.(fs.StatFS)
	if !ok {
		return nil, fmt.Errorf("filesystem does not support Stat")
	}

	return &FilesystemProvider{
		root:         statFS,
		defaultIndex: defaultIndex,
		dirIndex:     dirIndex,
	}, nil
}

// ReadFile reads a file at the given path and returns its content with MIME type.
//
// Path handling:
//   - Leading slashes are automatically stripped for fs.FS compatibility
//   - Root path "/" is treated as "." (current directory)
//
// Directory handling:
//  1. Try default index file first (always)
//  2. If not found and dirIndex=false: Return ErrDirListingDisabled
//  3. If not found and dirIndex=true: Generate markdown listing
//
// MIME type detection:
//   - Uses mime.TypeByExtension() for file extension mapping
//   - Returns "text/markdown" for README.md and directory listings
//   - Defaults to "application/octet-stream" for unknown types
func (f *FilesystemProvider) ReadFile(_ context.Context, requestPath string) ([]byte, string, error) {
	cleanPath := normalizePath(requestPath)

	// Check if path is directory
	info, err := fs.Stat(f.root, cleanPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", &PathError{Op: "read", Path: requestPath, Err: ErrNotFound}
		}
		return nil, "", &PathError{Op: "stat", Path: requestPath, Err: err}
	}

	if info.IsDir() {
		return f.handleDirectory(cleanPath, requestPath)
	}

	// Read regular file
	content, err := fs.ReadFile(f.root, cleanPath)
	if err != nil {
		return nil, "", &PathError{Op: "read", Path: requestPath, Err: err}
	}

	// Detect MIME type (returns full type with charset if registered)
	mimeType := common.DetectMIME(requestPath)

	return content, mimeType, nil
}

// handleDirectory processes directory requests with default index fallback and optional listing.
func (f *FilesystemProvider) handleDirectory(cleanPath, requestPath string) ([]byte, string, error) {
	// Try default index file first (always)
	indexPath := path.Join(cleanPath, f.defaultIndex)
	if cleanPath == "." {
		indexPath = f.defaultIndex
	}

	if content, err := fs.ReadFile(f.root, indexPath); err == nil {
		// Detect MIME type for the index file
		mimeType := mime.TypeByExtension(path.Ext(f.defaultIndex))
		if mimeType == "" {
			mimeType = "text/markdown"
		}
		return content, mimeType, nil
	}

	// Directory listing disabled (secure by default)
	if !f.dirIndex {
		return nil, "", &PathError{Op: "list", Path: requestPath, Err: ErrDirListingDisabled}
	}

	// Generate directory listing
	entries, err := fs.ReadDir(f.root, cleanPath)
	if err != nil {
		return nil, "", &PathError{Op: "list", Path: requestPath, Err: err}
	}

	content := GenerateMarkdownListing(requestPath, entries)
	return content, "text/markdown; charset=utf-8", nil
}

// Stat returns a FileInfo describing the named file
func (f *FilesystemProvider) Stat(_ context.Context, requestPath string) (fs.FileInfo, error) {
	cleanPath := normalizePath(requestPath)

	info, err := fs.Stat(f.root, cleanPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &PathError{Op: "stat", Path: requestPath, Err: ErrNotFound}
		}
		return nil, &PathError{Op: "stat", Path: requestPath, Err: err}
	}

	return info, nil
}

// DefaultIndex returns the configured default index file
func (f *FilesystemProvider) DefaultIndex() string {
	return f.defaultIndex
}

// RootFS returns the content root as an fs.FS
func (f *FilesystemProvider) RootFS(_ context.Context) (fs.FS, error) {
	return f.root, nil
}

// Close releases resources held by the provider
// Currently os.DirFS doesn't require explicit cleanup, but this method
// provides a hook for future implementations that may need resource cleanup
func (f *FilesystemProvider) Close() error {
	// No cleanup needed for os.DirFS currently
	// Future providers (database, network) may need cleanup here
	return nil
}
