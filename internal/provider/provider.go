// Package provider abstracts where content comes from: a directory on disk, a
// git repository, or an overlay of several sources.
//
// Every method taking a name owes an *fs.PathError or nothing, constructed
// through the one helper here so two implementations cannot spell the same Op
// differently. The mapping onto fs.ErrNotExist runs in both directions: only
// genuinely-missing errors become the sentinel, because collapsing a corrupt
// repository into "not found" serves a themed 404 on every page and never
// tells the operator the repository, rather than the URL, is broken.
package provider

import (
	"context"
	"io"
	"io/fs"
)

// Provider defines the interface for content providers
// Embeds io.Closer for proper resource cleanup
type Provider interface {
	// ReadFile reads a file at the given path and detects its MIME type
	// Returns full MIME type with parameters (e.g., "text/html; charset=utf-8")
	// For directories, may return README.md content or directory listing based on configuration
	ReadFile(ctx context.Context, path string) (content []byte, mimeType string, err error)
	// Stat returns a FileInfo describing the named file
	Stat(ctx context.Context, path string) (fs.FileInfo, error)
	// DefaultIndex returns the default index file name
	DefaultIndex() string
	// RootFS returns the content root as an fs.FS for reading files directly.
	// For filesystem providers this is the directory FS; for Git providers
	// this is backed by the cloned tree (triggering a clone if needed).
	RootFS(ctx context.Context) (fs.FS, error)
	// Close releases resources held by the provider
	io.Closer
}
