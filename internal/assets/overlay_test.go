package assets

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestOverlayFS_Operations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filesystems []fstest.MapFS
		operation   string
		path        string
		wantContent string
		wantErr     error
		wantSize    int64
	}{
		{
			name: "single filesystem - open and read",
			filesystems: []fstest.MapFS{
				{"file.txt": &fstest.MapFile{Data: []byte("content1")}},
			},
			operation:   "open",
			path:        "file.txt",
			wantContent: "content1",
		},
		{
			name: "single filesystem - read file",
			filesystems: []fstest.MapFS{
				{"file.txt": &fstest.MapFile{Data: []byte("content1")}},
			},
			operation:   "read",
			path:        "file.txt",
			wantContent: "content1",
		},
		{
			name: "two filesystems - first wins",
			filesystems: []fstest.MapFS{
				{"file.txt": &fstest.MapFile{Data: []byte("from fs1")}},
				{"file.txt": &fstest.MapFile{Data: []byte("from fs2")}},
			},
			operation:   "read",
			path:        "file.txt",
			wantContent: "from fs1",
		},
		{
			name: "two filesystems - fallback to second",
			filesystems: []fstest.MapFS{
				{"other.txt": &fstest.MapFile{Data: []byte("other")}},
				{"file.txt": &fstest.MapFile{Data: []byte("from fs2")}},
			},
			operation:   "read",
			path:        "file.txt",
			wantContent: "from fs2",
		},
		{
			name: "file not found via open",
			filesystems: []fstest.MapFS{
				{"file1.txt": &fstest.MapFile{Data: []byte("content1")}},
				{"file2.txt": &fstest.MapFile{Data: []byte("content2")}},
			},
			operation: "open",
			path:      "nonexistent.txt",
			wantErr:   fs.ErrNotExist,
		},
		{
			name: "file not found in any filesystem",
			filesystems: []fstest.MapFS{
				{"file1.txt": &fstest.MapFile{Data: []byte("content1")}},
				{"file2.txt": &fstest.MapFile{Data: []byte("content2")}},
			},
			operation: "read",
			path:      "nonexistent.txt",
			wantErr:   fs.ErrNotExist,
		},
		{
			name: "stat - first filesystem",
			filesystems: []fstest.MapFS{
				{"file.txt": &fstest.MapFile{Data: []byte("from fs1")}},
				{"file.txt": &fstest.MapFile{Data: []byte("from fs2 - longer")}},
			},
			operation: "stat",
			path:      "file.txt",
			wantSize:  8, // "from fs1" = 8 bytes
		},
		{
			name: "stat - file not found",
			filesystems: []fstest.MapFS{
				{"file1.txt": &fstest.MapFile{Data: []byte("content")}},
			},
			operation: "stat",
			path:      "nonexistent.txt",
			wantErr:   fs.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Convert []fstest.MapFS to []fs.FS
			fss := make([]fs.FS, len(tt.filesystems))
			for i, fsys := range tt.filesystems {
				fss[i] = fsys
			}
			overlay := NewOverlayFS(fss...)

			switch tt.operation {
			case "open":
				f, err := overlay.Open(tt.path)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("Open() error = %v, want %v", err, tt.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("Open() unexpected error: %v", err)
				}
				defer f.Close()
				// Verify file can be opened successfully

			case "read":
				data, err := overlay.ReadFile(tt.path)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("ReadFile() error = %v, want %v", err, tt.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("ReadFile() unexpected error: %v", err)
				}
				if string(data) != tt.wantContent {
					t.Errorf("ReadFile() = %q, want %q", string(data), tt.wantContent)
				}

			case "stat":
				info, err := overlay.Stat(tt.path)
				if tt.wantErr != nil {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("Stat() error = %v, want %v", err, tt.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("Stat() unexpected error: %v", err)
				}
				if info.Size() != tt.wantSize {
					t.Errorf("Stat() size = %d, want %d", info.Size(), tt.wantSize)
				}
			}
		})
	}
}

func TestOverlayFS_ReadDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filesystems []fstest.MapFS
		path        string
		wantFiles   []string // Must be sorted by name
		wantErr     error
	}{
		{
			name: "single filesystem",
			filesystems: []fstest.MapFS{
				{
					"dir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
					"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
				},
			},
			path:      "dir",
			wantFiles: []string{"file1.txt", "file2.txt"},
		},
		{
			name: "merged directories - deduplicated",
			filesystems: []fstest.MapFS{
				{
					"dir/file1.txt": &fstest.MapFile{Data: []byte("from fs1")},
					"dir/file2.txt": &fstest.MapFile{Data: []byte("from fs1")},
				},
				{
					"dir/file2.txt": &fstest.MapFile{Data: []byte("from fs2")}, // Should be deduplicated
					"dir/file3.txt": &fstest.MapFile{Data: []byte("from fs2")},
				},
			},
			path:      "dir",
			wantFiles: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name: "three filesystems merged",
			filesystems: []fstest.MapFS{
				{"themes/file1.txt": &fstest.MapFile{Data: []byte("fs1")}},
				{"themes/file2.txt": &fstest.MapFile{Data: []byte("fs2")}},
				{"themes/file3.txt": &fstest.MapFile{Data: []byte("fs3")}},
			},
			path:      "themes",
			wantFiles: []string{"file1.txt", "file2.txt", "file3.txt"},
		},
		{
			name: "directory not found",
			filesystems: []fstest.MapFS{
				{"dir/file.txt": &fstest.MapFile{Data: []byte("content")}},
			},
			path:    "nonexistent",
			wantErr: fs.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fss := make([]fs.FS, len(tt.filesystems))
			for i, fsys := range tt.filesystems {
				fss[i] = fsys
			}
			overlay := NewOverlayFS(fss...)

			entries, err := overlay.ReadDir(tt.path)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ReadDir() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadDir() unexpected error: %v", err)
			}

			if len(entries) != len(tt.wantFiles) {
				t.Errorf("ReadDir() returned %d entries, want %d", len(entries), len(tt.wantFiles))
			}

			// Verify entries are sorted by name
			for i, entry := range entries {
				if entry.Name() != tt.wantFiles[i] {
					t.Errorf("ReadDir()[%d] = %q, want %q", i, entry.Name(), tt.wantFiles[i])
				}
			}
		})
	}
}

func TestOverlayFS_EmptyFilesystems(t *testing.T) {
	t.Parallel()

	overlay := NewOverlayFS()

	_, err := overlay.Open("any.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open() on empty overlay error = %v, want ErrNotExist", err)
	}

	_, err = overlay.ReadFile("any.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile() on empty overlay error = %v, want ErrNotExist", err)
	}

	_, err = overlay.Stat("any.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() on empty overlay error = %v, want ErrNotExist", err)
	}

	_, err = overlay.ReadDir("any")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadDir() on empty overlay error = %v, want ErrNotExist", err)
	}
}

func TestOverlayFS_UserAssetOverride(t *testing.T) {
	t.Parallel()

	userAssets := fstest.MapFS{
		"themes/custom/layout.html": &fstest.MapFile{
			Data: []byte("<html>Custom</html>"),
		},
	}

	embeddedAssets := fstest.MapFS{
		"themes/default/layout.html": &fstest.MapFile{
			Data: []byte("<html>Default</html>"),
		},
		"themes/custom/layout.html": &fstest.MapFile{
			Data: []byte("<html>Embedded Custom</html>"),
		},
	}

	overlay := NewOverlayFS(userAssets, embeddedAssets)

	// User custom theme should override embedded
	data, err := overlay.ReadFile("themes/custom/layout.html")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "<html>Custom</html>" {
		t.Errorf("ReadFile() = %q, want user asset to override", string(data))
	}

	// Default theme should come from embedded
	data, err = overlay.ReadFile("themes/default/layout.html")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "<html>Default</html>" {
		t.Errorf("ReadFile() = %q, want embedded asset", string(data))
	}
}

// basicFS wraps MapFS but hides StatFS interface to test fallback
type basicFS struct {
	fstest.MapFS
}

func TestOverlayFS_StatFallback(t *testing.T) {
	t.Parallel()

	// Use basicFS which doesn't implement fs.StatFS
	fsys := basicFS{
		MapFS: fstest.MapFS{
			"file.txt": &fstest.MapFile{Data: []byte("content")},
		},
	}

	overlay := NewOverlayFS(fsys)

	// Should fall back to Open() -> Stat()
	info, err := overlay.Stat("file.txt")
	if err != nil {
		t.Fatalf("Stat() fallback error: %v", err)
	}

	if info.Size() != 7 {
		t.Errorf("Stat() size = %d, want 7", info.Size())
	}
}

func TestBuildFS(t *testing.T) {
	t.Parallel()

	embedded := fstest.MapFS{
		"themes/default/layout.html": &fstest.MapFile{Data: []byte("embedded")},
	}

	t.Run("returns embedded when no config dir", func(t *testing.T) {
		t.Parallel()
		contentRoot := fstest.MapFS{
			"README.md": &fstest.MapFile{Data: []byte("hello")},
		}
		result := BuildFS(contentRoot, embedded)
		data, err := fs.ReadFile(result, "themes/default/layout.html")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "embedded" {
			t.Errorf("got %q, want %q", data, "embedded")
		}
	})

	t.Run("overlays config dir on top of embedded", func(t *testing.T) {
		t.Parallel()
		contentRoot := fstest.MapFS{
			config.ConfigDirName + "/themes/default/layout.html": &fstest.MapFile{Data: []byte("local")},
		}
		result := BuildFS(contentRoot, embedded)

		// Local file should win
		data, err := fs.ReadFile(result, "themes/default/layout.html")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "local" {
			t.Errorf("got %q, want %q", data, "local")
		}
	})

	t.Run("falls back to embedded for missing files", func(t *testing.T) {
		t.Parallel()
		contentRoot := fstest.MapFS{
			config.ConfigDirName + "/themes/custom/layout.html": &fstest.MapFile{Data: []byte("custom")},
		}
		result := BuildFS(contentRoot, embedded)

		// Default theme should come from embedded
		data, err := fs.ReadFile(result, "themes/default/layout.html")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "embedded" {
			t.Errorf("got %q, want %q", data, "embedded")
		}
	})
}
