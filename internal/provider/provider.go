package provider

import "io/fs"

// Provider defines the interface for content providers
type Provider interface {
	// ReadFile reads a file at the given path
	ReadFile(path string) ([]byte, error)
	// Stat returns a FileInfo describing the named file
	Stat(path string) (fs.FileInfo, error)
	// DefaultIndex returns the default index file name
	DefaultIndex() string
}
