package provider

import (
	"io/fs"
	"os"
	"path"
	"strings"
)

// FilesystemProvider implements Provider for local filesystem access
type FilesystemProvider struct {
	root         *os.Root
	defaultIndex string
}

// NewFilesystemProvider creates a new filesystem provider with path traversal protection
func NewFilesystemProvider(dir, defaultIndex string) (*FilesystemProvider, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}

	return &FilesystemProvider{
		root:         root,
		defaultIndex: defaultIndex,
	}, nil
}

// ReadFile reads a file from the filesystem with path traversal protection
func (f *FilesystemProvider) ReadFile(filename string) ([]byte, error) {
	if filename == "/" {
		filename = f.DefaultIndex()
	}
	// Clean the path first, then remove leading slash to make it relative
	filename = path.Clean(filename)
	filename = strings.TrimPrefix(filename, "/")

	stat, err := f.root.Stat(filename)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		filename = path.Join(filename, f.DefaultIndex())
		_, err = f.root.Stat(filename)
		if err != nil {
			return nil, err
		}
	}

	return f.root.ReadFile(filename)
}

// Stat returns a FileInfo describing the named file
func (f *FilesystemProvider) Stat(filename string) (fs.FileInfo, error) {
	if filename == "/" {
		filename = "."
	} else {
		// Clean the path first, then remove leading slash to make it relative
		filename = path.Clean(filename)
		filename = strings.TrimPrefix(filename, "/")
	}

	return f.root.Stat(filename)
}

// DefaultIndex returns the configured default index file
func (f *FilesystemProvider) DefaultIndex() string {
	return f.defaultIndex
}

// Close releases resources held by the provider
// Currently os.Root doesn't require explicit cleanup, but this method
// provides a hook for future implementations that may need resource cleanup
func (f *FilesystemProvider) Close() error {
	// No cleanup needed for os.Root currently
	// Future providers (database, network) may need cleanup here
	return nil
}
