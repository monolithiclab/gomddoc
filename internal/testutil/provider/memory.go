package provider

import (
	"context"
	"io/fs"
	"strings"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// MemoryProvider implements provider.Provider using an in-memory filesystem
type MemoryProvider struct {
	fsys         fstest.MapFS
	defaultIndex string
	dirIndex     bool
}

var _ provider.Provider = (*MemoryProvider)(nil)

// NewMemoryProvider creates a provider backed by fstest.MapFS (in-memory)
func NewMemoryProvider(files fstest.MapFS, defaultIndex string, dirIndex bool) *MemoryProvider {
	return &MemoryProvider{
		fsys:         files,
		defaultIndex: defaultIndex,
		dirIndex:     dirIndex,
	}
}

func (m *MemoryProvider) ReadFile(ctx context.Context, path string) ([]byte, string, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" || path == "." {
		path = m.defaultIndex
	}

	// Check if path is a directory
	info, err := m.Stat(ctx, path)
	if err == nil && info.IsDir() {
		// Try default index in directory
		indexPath := path
		if indexPath != "." {
			indexPath += "/"
		}
		indexPath += m.defaultIndex

		if data, err := fs.ReadFile(m.fsys, indexPath); err == nil {
			return data, negotiate.DetectMIME(indexPath), nil
		}

		if m.dirIndex {
			// Generate directory listing
			entries, err := fs.ReadDir(m.fsys, path)
			if err != nil {
				return nil, "", provider.ErrNotFound
			}
			listing := provider.GenerateMarkdownListing(path, entries, nil)
			return listing, "text/markdown; charset=utf-8", nil
		}

		return nil, "", provider.ErrDirListingDisabled
	}

	data, err := fs.ReadFile(m.fsys, path)
	if err != nil {
		return nil, "", provider.ErrNotFound
	}

	return data, negotiate.DetectMIME(path), nil
}

func (m *MemoryProvider) Stat(_ context.Context, path string) (fs.FileInfo, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		path = "."
	}

	info, err := fs.Stat(m.fsys, path)
	if err != nil {
		return nil, provider.ErrNotFound
	}
	return info, nil
}

func (m *MemoryProvider) DefaultIndex() string {
	return m.defaultIndex
}

func (m *MemoryProvider) RootFS(_ context.Context) (fs.FS, error) {
	return m.fsys, nil
}

func (m *MemoryProvider) Close() error {
	return nil
}
