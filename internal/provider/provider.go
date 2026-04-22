package provider

import (
	"io"
	"io/fs"
)

// Provider defines the interface for content providers
// Embeds io.Closer for proper resource cleanup
type Provider interface {
	// ReadFile reads a file at the given path
	ReadFile(path string) ([]byte, error)
	// Stat returns a FileInfo describing the named file
	Stat(path string) (fs.FileInfo, error)
	// DefaultIndex returns the default index file name
	DefaultIndex() string
	// Close releases resources held by the provider
	io.Closer
}
