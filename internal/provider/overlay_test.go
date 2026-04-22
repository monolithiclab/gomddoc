package provider

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestOverlayProvider(t *testing.T) {
	primaryFiles := fstest.MapFS{
		"README.md":           &fstest.MapFile{Data: []byte("# Primary README")},
		"only-in-primary.txt": &fstest.MapFile{Data: []byte("primary only")},
	}
	fallbackFiles := fstest.MapFS{
		"favicon.ico":          &fstest.MapFile{Data: []byte{0x00, 0x00, 0x01, 0x00}},
		"README.md":            &fstest.MapFile{Data: []byte("# Fallback README")},
		"only-in-fallback.txt": &fstest.MapFile{Data: []byte("fallback only")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, fallbackFiles)

	ctx := context.Background()

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantErr     error
	}{
		{
			name:        "reads from primary",
			path:        "/README.md",
			wantContent: "# Primary README",
		},
		{
			name:        "reads from primary only",
			path:        "/only-in-primary.txt",
			wantContent: "primary only",
		},
		{
			name:        "falls back to filesystem",
			path:        "/favicon.ico",
			wantContent: "\x00\x00\x01\x00",
		},
		{
			name:        "falls back to filesystem only",
			path:        "/only-in-fallback.txt",
			wantContent: "fallback only",
		},
		{
			name:    "not found in both",
			path:    "/missing.txt",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, _, err := overlay.ReadFile(ctx, tt.path)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("content = %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}

func TestOverlayProvider_RootFS(t *testing.T) {
	primaryFiles := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
	}
	fallbackFiles := fstest.MapFS{
		"b.txt": &fstest.MapFile{Data: []byte("b")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, fallbackFiles)

	root, err := overlay.RootFS(context.Background())
	if err != nil {
		t.Fatalf("RootFS failed: %v", err)
	}

	// Should see files from both
	if _, err := fs.Stat(root, "a.txt"); err != nil {
		t.Errorf("a.txt not found in RootFS")
	}
	if _, err := fs.Stat(root, "b.txt"); err != nil {
		t.Errorf("b.txt not found in RootFS")
	}
}

func TestOverlayProvider_RootFS_NilFallback(t *testing.T) {
	t.Parallel()

	primaryFiles := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, nil)

	root, err := overlay.RootFS(context.Background())
	if err != nil {
		t.Fatalf("RootFS() error = %v", err)
	}

	// Should return primary FS directly (not wrapped in OverlayFS)
	if _, err := fs.Stat(root, "a.txt"); err != nil {
		t.Errorf("a.txt not found in RootFS: %v", err)
	}
}

func TestOverlayProvider_Stat(t *testing.T) {
	t.Parallel()

	primaryFiles := fstest.MapFS{
		"primary.txt": &fstest.MapFile{Data: []byte("p")},
	}
	fallbackFiles := fstest.MapFS{
		"fallback.txt": &fstest.MapFile{Data: []byte("f")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, fallbackFiles)
	ctx := context.Background()

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"stat from primary", "/primary.txt", nil},
		{"stat from fallback", "/fallback.txt", nil},
		{"stat not found in both", "/missing.txt", ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info, err := overlay.Stat(ctx, tt.path)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Stat() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Stat() error = %v, want nil", err)
			}
			if info == nil {
				t.Fatal("Stat() returned nil info")
			}
		})
	}
}

func TestOverlayProvider_Stat_RootPath(t *testing.T) {
	t.Parallel()

	primaryFiles := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
	}
	fallbackFiles := fstest.MapFS{
		"b.txt": &fstest.MapFile{Data: []byte("b")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, fallbackFiles)

	// Stat on root "/" normalizes to "." — primary handles it
	info, err := overlay.Stat(context.Background(), "/")
	if err != nil {
		t.Fatalf("Stat(\"/\") error = %v", err)
	}
	if !info.IsDir() {
		t.Error("Stat(\"/\") should report IsDir=true")
	}
}

func TestOverlayProvider_DefaultIndex(t *testing.T) {
	t.Parallel()

	primaryFiles := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "index.md", false)
	overlay := NewOverlayProvider(primary, nil)

	if got := overlay.DefaultIndex(); got != "index.md" {
		t.Errorf("DefaultIndex() = %q, want %q", got, "index.md")
	}
}

func TestOverlayProvider_Close(t *testing.T) {
	t.Parallel()

	primaryFiles := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
	}

	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, nil)

	if err := overlay.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestOverlayProvider_ReadFile_RootPath(t *testing.T) {
	t.Parallel()

	// When primary returns a non-ErrNotFound error for root "/" path,
	// fallback should NOT be tried (only ErrNotFound triggers fallback).
	primaryFiles := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("a")},
	}
	fallbackFiles := fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("# Fallback")},
	}

	// dirIndex=false: root "/" with no README.md returns ErrDirListingDisabled
	primary, _ := NewFilesystemProviderFromFS(primaryFiles, "README.md", false)
	overlay := NewOverlayProvider(primary, fallbackFiles)

	_, _, err := overlay.ReadFile(context.Background(), "/")
	if err == nil {
		t.Fatal("ReadFile(\"/\") error = nil, want error")
	}
	// Error is ErrDirListingDisabled (not ErrNotFound), so fallback is not attempted
	if !errors.Is(err, ErrDirListingDisabled) {
		t.Errorf("ReadFile(\"/\") error = %v, want ErrDirListingDisabled", err)
	}
}
