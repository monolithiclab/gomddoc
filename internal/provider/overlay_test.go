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
