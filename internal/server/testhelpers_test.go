package server

import (
	"io/fs"
	"strings"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

// memoryProvider implements provider.Provider using an in-memory filesystem
// This eliminates disk I/O from tests and makes them faster and more reliable
type memoryProvider struct {
	fsys         fstest.MapFS
	defaultIndex string
	dirIndex     bool
}

var _ provider.Provider = (*memoryProvider)(nil)

// newMemoryProvider creates a provider backed by fstest.MapFS (in-memory)
func newMemoryProvider(files fstest.MapFS, defaultIndex string, dirIndex bool) *memoryProvider {
	return &memoryProvider{
		fsys:         files,
		defaultIndex: defaultIndex,
		dirIndex:     dirIndex,
	}
}

func (m *memoryProvider) ReadFile(path string) ([]byte, string, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" || path == "." {
		path = m.defaultIndex
	}

	// Check if path is a directory
	info, err := m.Stat(path)
	if err == nil && info.IsDir() {
		// Try default index in directory
		indexPath := path + "/" + m.defaultIndex
		if data, err := fs.ReadFile(m.fsys, indexPath); err == nil {
			return data, detectMimeType(indexPath), nil
		}

		if m.dirIndex {
			// Generate directory listing
			entries, err := fs.ReadDir(m.fsys, path)
			if err != nil {
				return nil, "", provider.ErrNotFound
			}
			listing := provider.GenerateMarkdownListing(path, entries)
			return listing, "text/markdown; charset=utf-8", nil
		}

		return nil, "", provider.ErrDirListingDisabled
	}

	data, err := fs.ReadFile(m.fsys, path)
	if err != nil {
		return nil, "", provider.ErrNotFound
	}

	return data, detectMimeType(path), nil
}

func (m *memoryProvider) Stat(path string) (fs.FileInfo, error) {
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

func (m *memoryProvider) DefaultIndex() string {
	return m.defaultIndex
}

func (m *memoryProvider) Close() error {
	return nil
}

// detectMimeType returns MIME type based on file extension
func detectMimeType(path string) string {
	switch {
	case strings.HasSuffix(path, ".md"):
		return "text/markdown; charset=utf-8"
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".txt"):
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// Test helpers shared across test files

func setupTestRenderer() *tmpl.HTMLRenderer {
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Site.Meta.Title}}</title></head>
<body>{{.Page.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layout.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"

	return tmpl.NewHTMLRenderer(&siteConfig, testFS)
}

func setupTestRegistry() renderer.RendererRegistry {
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownRenderer(""))
	registry.Register(renderer.NewPassthroughRenderer())
	return registry
}
