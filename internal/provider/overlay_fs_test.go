package provider

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
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

// TestOverlayFS_PathErrorContract pins what io/fs owes every method that takes
// a name: a *fs.PathError carrying the operation and the path, not a bare
// sentinel that errors.As cannot reach and fs.WalkDir cannot report.
//
// The "missing" rows run against zero layers, the only configuration that
// reaches OverlayFS's own not-found returns — with a layer present the wrapped
// filesystem supplies the error. The invalid-path rows run against a populated
// overlay on purpose: fstest.MapFS would answer with its own *fs.PathError, so
// the op string is what proves the guard fired here and not one layer down.
func TestOverlayFS_PathErrorContract(t *testing.T) {
	t.Parallel()

	empty := NewOverlayFS()
	layered := NewOverlayFS(fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("x")}})

	methods := []struct {
		name string
		op   string
		call func(fs.FS, string) error
	}{
		{"Open", opOpen, func(f fs.FS, n string) error { _, err := f.Open(n); return err }},
		{"ReadFile", opReadFile, func(f fs.FS, n string) error { _, err := fs.ReadFile(f, n); return err }},
		{"Stat", opStat, func(f fs.FS, n string) error { _, err := fs.Stat(f, n); return err }},
		{"ReadDir", opReadDir, func(f fs.FS, n string) error { _, err := fs.ReadDir(f, n); return err }},
	}

	paths := []struct {
		name    string
		fsys    fs.FS
		path    string
		wantErr error
	}{
		{"missing", empty, "dir/any.txt", fs.ErrNotExist},
		{"parent escape", layered, "../escape.txt", fs.ErrInvalid},
		{"absolute", layered, "/etc/passwd", fs.ErrInvalid},
	}

	for _, m := range methods {
		for _, p := range paths {
			t.Run(m.name+"/"+p.name, func(t *testing.T) {
				t.Parallel()

				err := m.call(p.fsys, p.path)
				if err == nil {
					t.Fatalf("%s(%q) succeeded, want error", m.name, p.path)
				}
				var pe *fs.PathError
				if !errors.As(err, &pe) {
					t.Fatalf("%s(%q) error = %v (%T), want *fs.PathError", m.name, p.path, err, err)
				}
				if pe.Op != m.op {
					t.Errorf("Op = %q, want %q", pe.Op, m.op)
				}
				if pe.Path != p.path {
					t.Errorf("Path = %q, want %q", pe.Path, p.path)
				}
				if !errors.Is(err, p.wantErr) {
					t.Errorf("error = %v, want %v", err, p.wantErr)
				}
			})
		}
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

// nonStatFS wraps an fs.FS to ensure it does NOT implement fs.StatFS.
// Only the Open method is exposed, forcing the Stat fallback path.
type nonStatFS struct {
	inner fs.FS
}

func (n *nonStatFS) Open(name string) (fs.File, error) {
	return n.inner.Open(name)
}

func TestOverlayFS_Stat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		buildFS  func() *OverlayFS
		path     string
		wantName string
		wantSize int64
		wantErr  error
	}{
		{
			name: "StatFS path - file found",
			buildFS: func() *OverlayFS {
				return NewOverlayFS(fstest.MapFS{
					"file.txt": &fstest.MapFile{Data: []byte("hello"), Mode: 0644},
				})
			},
			path:     "file.txt",
			wantName: "file.txt",
			wantSize: 5,
		},
		{
			name: "StatFS path - file not found",
			buildFS: func() *OverlayFS {
				return NewOverlayFS(fstest.MapFS{
					"other.txt": &fstest.MapFile{Data: []byte("x")},
				})
			},
			path:    "missing.txt",
			wantErr: fs.ErrNotExist,
		},
		{
			name: "StatFS path - first filesystem wins",
			buildFS: func() *OverlayFS {
				fs1 := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("short")}}
				fs2 := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("longer content")}}
				return NewOverlayFS(fs1, fs2)
			},
			path:     "file.txt",
			wantName: "file.txt",
			wantSize: 5,
		},
		{
			name: "StatFS path - fallback to second filesystem",
			buildFS: func() *OverlayFS {
				fs1 := fstest.MapFS{"other.txt": &fstest.MapFile{Data: []byte("x")}}
				fs2 := fstest.MapFS{"target.txt": &fstest.MapFile{Data: []byte("found")}}
				return NewOverlayFS(fs1, fs2)
			},
			path:     "target.txt",
			wantName: "target.txt",
			wantSize: 5,
		},
		{
			name: "non-StatFS fallback - file found via Open+Stat",
			buildFS: func() *OverlayFS {
				inner := fstest.MapFS{
					"doc.txt": &fstest.MapFile{Data: []byte("content")},
				}
				return NewOverlayFS(&nonStatFS{inner: inner})
			},
			path:     "doc.txt",
			wantName: "doc.txt",
			wantSize: 7,
		},
		{
			name: "non-StatFS fallback - file not found",
			buildFS: func() *OverlayFS {
				inner := fstest.MapFS{
					"other.txt": &fstest.MapFile{Data: []byte("x")},
				}
				return NewOverlayFS(&nonStatFS{inner: inner})
			},
			path:    "missing.txt",
			wantErr: fs.ErrNotExist,
		},
		{
			name: "mixed StatFS and non-StatFS - non-StatFS first wins",
			buildFS: func() *OverlayFS {
				inner := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("from non-stat")}}
				statFS := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("from stat")}}
				return NewOverlayFS(&nonStatFS{inner: inner}, statFS)
			},
			path:     "file.txt",
			wantName: "file.txt",
			wantSize: int64(len("from non-stat")),
		},
		{
			name: "non-StatFS not found falls back to StatFS",
			buildFS: func() *OverlayFS {
				inner := fstest.MapFS{"other.txt": &fstest.MapFile{Data: []byte("x")}}
				statFS := fstest.MapFS{"target.txt": &fstest.MapFile{Data: []byte("found in stat")}}
				return NewOverlayFS(&nonStatFS{inner: inner}, statFS)
			},
			path:     "target.txt",
			wantName: "target.txt",
			wantSize: int64(len("found in stat")),
		},
		{
			name: "StatFS not found falls back to non-StatFS",
			buildFS: func() *OverlayFS {
				statFS := fstest.MapFS{"other.txt": &fstest.MapFile{Data: []byte("x")}}
				inner := fstest.MapFS{"target.txt": &fstest.MapFile{Data: []byte("found via open")}}
				return NewOverlayFS(statFS, &nonStatFS{inner: inner})
			},
			path:     "target.txt",
			wantName: "target.txt",
			wantSize: int64(len("found via open")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			overlay := tt.buildFS()
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
			if info.Name() != tt.wantName {
				t.Errorf("Stat() Name = %q, want %q", info.Name(), tt.wantName)
			}
			if info.Size() != tt.wantSize {
				t.Errorf("Stat() Size = %d, want %d", info.Size(), tt.wantSize)
			}
		})
	}
}
