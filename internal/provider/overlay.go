package provider

import (
	"context"
	"errors"
	"io/fs"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// OverlayProvider wraps a primary provider and a fallback filesystem.
// It implements the Provider interface by first checking the primary
// provider, and then falling back to the filesystem if not found.
type OverlayProvider struct {
	primary  Provider
	fallback fs.FS
}

// NewOverlayProvider creates a new overlay provider.
func NewOverlayProvider(primary Provider, fallback fs.FS) *OverlayProvider {
	return &OverlayProvider{
		primary:  primary,
		fallback: fallback,
	}
}

// ReadFile reads from the primary provider, falling back to the filesystem if not found.
func (o *OverlayProvider) ReadFile(ctx context.Context, requestPath string) ([]byte, string, error) {
	content, mimeType, err := o.primary.ReadFile(ctx, requestPath)
	if err != nil && o.fallback != nil && errors.Is(err, ErrNotFound) {
		cleanPath := normalizePath(requestPath)
		if cleanPath == "." {
			return nil, "", err
		}
		if data, readErr := fs.ReadFile(o.fallback, cleanPath); readErr == nil {
			return data, negotiate.DetectMIME(requestPath), nil
		}
	}
	return content, mimeType, err
}

// Stat returns info from the primary provider, falling back to the filesystem if not found.
func (o *OverlayProvider) Stat(ctx context.Context, requestPath string) (fs.FileInfo, error) {
	info, err := o.primary.Stat(ctx, requestPath)
	if err != nil && o.fallback != nil && errors.Is(err, ErrNotFound) {
		cleanPath := normalizePath(requestPath)
		if cleanPath == "." {
			return nil, err
		}
		if statInfo, statErr := fs.Stat(o.fallback, cleanPath); statErr == nil {
			return statInfo, nil
		}
	}
	return info, err
}

// DefaultIndex returns the primary provider's default index.
func (o *OverlayProvider) DefaultIndex() string {
	return o.primary.DefaultIndex()
}

// RootFS returns an OverlayFS combining the primary's RootFS and the fallback FS.
func (o *OverlayProvider) RootFS(ctx context.Context) (fs.FS, error) {
	primaryFS, err := o.primary.RootFS(ctx)
	if err != nil {
		return nil, err
	}
	if o.fallback == nil {
		return primaryFS, nil
	}
	return NewOverlayFS(primaryFS, o.fallback), nil
}

// Close closes the primary provider.
func (o *OverlayProvider) Close() error {
	return o.primary.Close()
}
