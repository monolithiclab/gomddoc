// Package provider offers an in-memory provider.Provider for tests that need
// a content tree without a directory on disk or a git repository behind it.
package provider

import (
	"context"
	"io/fs"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// MemoryProvider implements provider.Provider using an in-memory filesystem
type MemoryProvider struct {
	fsys         fs.FS
	defaultIndex string
	dirIndex     bool
}

var _ provider.Provider = (*MemoryProvider)(nil)

// NewMemoryProvider creates a provider backed by an in-memory filesystem,
// usually an fstest.MapFS.
//
// The field is the fs.FS interface rather than fstest.MapFS so a test can wrap
// the tree — a fake that fails one path, a counting FS — and have the wrapper
// reach RootFS and the read methods alike. Taking the concrete type instead
// forces the caller to override RootFS, which leaves the provider reading a
// tree that disagrees with the one it hands out.
func NewMemoryProvider(files fs.FS, defaultIndex string, dirIndex bool) *MemoryProvider {
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
