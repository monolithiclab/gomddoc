package provider

import (
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

// DefaultIndex returns the configured default index file
func (f *FilesystemProvider) DefaultIndex() string {
	return f.defaultIndex
}
