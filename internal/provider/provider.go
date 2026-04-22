package provider

import (
	"io"
	"io/fs"
)

// Provider defines the interface for content providers
// Embeds io.Closer for proper resource cleanup
type Provider interface {
	// ReadFile reads a file at the given path and detects its MIME type
	// Returns full MIME type with parameters (e.g., "text/html; charset=utf-8")
	// For directories, may return README.md content or directory listing based on configuration
	ReadFile(path string) (content []byte, mimeType string, err error)
	// Stat returns a FileInfo describing the named file
	Stat(path string) (fs.FileInfo, error)
	// DefaultIndex returns the default index file name
	DefaultIndex() string
	// RootFS returns the content root as an fs.FS for reading files directly.
	// For filesystem providers this is the directory FS; for Git providers
	// this is backed by the cloned tree (triggering a clone if needed).
	RootFS() (fs.FS, error)
	// Close releases resources held by the provider
	io.Closer
}
