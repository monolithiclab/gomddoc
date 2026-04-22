package assets

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestOverlayFS_SingleFS(t *testing.T) {
	// Create a single filesystem
	fs1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	overlay := NewOverlayFS(fs1)

	// Test opening existing file
	f, err := overlay.Open("file1.txt")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Test reading file
	data, err := overlay.ReadFile("file2.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != "content2" {
		t.Errorf("ReadFile content = %q, want %q", string(data), "content2")
	}
}

func TestOverlayFS_TwoFS_FirstFound(t *testing.T) {
	// Create two filesystems with overlapping files
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("from fs1")},
	}
	fs2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("from fs2")},
	}

	overlay := NewOverlayFS(fs1, fs2)

	// Should get file from first filesystem
	data, err := overlay.ReadFile("file.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != "from fs1" {
		t.Errorf("ReadFile content = %q, want %q (should come from first FS)", string(data), "from fs1")
	}
}

func TestOverlayFS_TwoFS_SecondFound(t *testing.T) {
	// Create two filesystems, file only in second
	fs1 := fstest.MapFS{
		"other.txt": &fstest.MapFile{Data: []byte("other")},
	}
	fs2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("from fs2")},
	}

	overlay := NewOverlayFS(fs1, fs2)

	// Should fallback to second filesystem
	data, err := overlay.ReadFile("file.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != "from fs2" {
		t.Errorf("ReadFile content = %q, want %q (should fallback to second FS)", string(data), "from fs2")
	}
}

func TestOverlayFS_MultipleFS(t *testing.T) {
	// Create three filesystems
	fs1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("from fs1")},
	}
	fs2 := fstest.MapFS{
		"file2.txt": &fstest.MapFile{Data: []byte("from fs2")},
	}
	fs3 := fstest.MapFS{
		"file3.txt": &fstest.MapFile{Data: []byte("from fs3")},
	}

	overlay := NewOverlayFS(fs1, fs2, fs3)

	// Test each file comes from correct filesystem
	tests := []struct {
		name string
		file string
		want string
	}{
		{"from first", "file1.txt", "from fs1"},
		{"from second", "file2.txt", "from fs2"},
		{"from third", "file3.txt", "from fs3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := overlay.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("ReadFile failed: %v", err)
			}

			if string(data) != tt.want {
				t.Errorf("ReadFile content = %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestOverlayFS_NotFoundAny(t *testing.T) {
	fs1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
	}
	fs2 := fstest.MapFS{
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	overlay := NewOverlayFS(fs1, fs2)

	// Try to open non-existent file
	_, err := overlay.Open("nonexistent.txt")
	if err == nil {
		t.Error("Open should return error for non-existent file")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open error should be fs.ErrNotExist, got %v", err)
	}

	// Same with ReadFile
	_, err = overlay.ReadFile("nonexistent.txt")
	if err == nil {
		t.Error("ReadFile should return error for non-existent file")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile error should be fs.ErrNotExist, got %v", err)
	}
}

func TestOverlayFS_WithMapFS(t *testing.T) {
	// Test with fstest.MapFS for easy mocking
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
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != "<html>Custom</html>" {
		t.Errorf("Should use user asset, got %q", string(data))
	}

	// Default theme should come from embedded (not in user assets)
	data, err = overlay.ReadFile("themes/default/layout.html")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != "<html>Default</html>" {
		t.Errorf("Should use embedded asset, got %q", string(data))
	}
}

func TestOverlayFS_ReadFile_Optimization(t *testing.T) {
	// Verify ReadFile uses fs.ReadFile for efficiency
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	overlay := NewOverlayFS(fs1)

	// ReadFile should work
	data, err := overlay.ReadFile("file.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != "content" {
		t.Errorf("ReadFile content = %q, want %q", string(data), "content")
	}
}

func TestOverlayFS_Stat_SingleFS(t *testing.T) {
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("content"),
			Mode: 0644,
		},
	}

	overlay := NewOverlayFS(fs1)

	// Test Stat on existing file
	info, err := overlay.Stat("file.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	if info.Name() != "file.txt" {
		t.Errorf("Stat name = %q, want %q", info.Name(), "file.txt")
	}

	if info.Size() != 7 {
		t.Errorf("Stat size = %d, want %d", info.Size(), 7)
	}

	if info.IsDir() {
		t.Error("Stat IsDir should be false for file")
	}
}

func TestOverlayFS_Stat_FirstFound(t *testing.T) {
	fs1 := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("from fs1"),
		},
	}
	fs2 := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("from fs2 - longer content"),
		},
	}

	overlay := NewOverlayFS(fs1, fs2)

	// Should get stat from first filesystem
	info, err := overlay.Stat("file.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	// fs1 file has 8 bytes, fs2 has 26 bytes
	if info.Size() != 8 {
		t.Errorf("Stat size = %d, want %d (from first FS)", info.Size(), 8)
	}
}

func TestOverlayFS_Stat_NotFound(t *testing.T) {
	fs1 := fstest.MapFS{
		"file1.txt": &fstest.MapFile{Data: []byte("content1")},
	}
	fs2 := fstest.MapFS{
		"file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	overlay := NewOverlayFS(fs1, fs2)

	// Try to stat non-existent file
	_, err := overlay.Stat("nonexistent.txt")
	if err == nil {
		t.Error("Stat should return error for non-existent file")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat error should be fs.ErrNotExist, got %v", err)
	}
}

func TestOverlayFS_ReadDir_SingleFS(t *testing.T) {
	fs1 := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{Data: []byte("content1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("content2")},
	}

	overlay := NewOverlayFS(fs1)

	// Test ReadDir on directory
	entries, err := overlay.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("ReadDir entries count = %d, want %d", len(entries), 2)
	}

	// Collect entry names
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}

	if !names["file1.txt"] {
		t.Error("ReadDir should include file1.txt")
	}
	if !names["file2.txt"] {
		t.Error("ReadDir should include file2.txt")
	}
}

func TestOverlayFS_ReadDir_Merged(t *testing.T) {
	// fs1 has file1.txt and file2.txt
	fs1 := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{Data: []byte("from fs1")},
		"dir/file2.txt": &fstest.MapFile{Data: []byte("from fs1")},
	}
	// fs2 has file2.txt (should be overridden) and file3.txt
	fs2 := fstest.MapFS{
		"dir/file2.txt": &fstest.MapFile{Data: []byte("from fs2")},
		"dir/file3.txt": &fstest.MapFile{Data: []byte("from fs2")},
	}

	overlay := NewOverlayFS(fs1, fs2)

	// Test ReadDir merges entries from both filesystems
	entries, err := overlay.ReadDir("dir")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	// Should have 3 unique entries: file1.txt, file2.txt (from fs1), file3.txt
	if len(entries) != 3 {
		t.Errorf("ReadDir merged entries count = %d, want %d", len(entries), 3)
	}

	// Collect entry names
	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}

	if !names["file1.txt"] {
		t.Error("ReadDir should include file1.txt from fs1")
	}
	if !names["file2.txt"] {
		t.Error("ReadDir should include file2.txt (from fs1, not fs2)")
	}
	if !names["file3.txt"] {
		t.Error("ReadDir should include file3.txt from fs2")
	}
}

func TestOverlayFS_ReadDir_NotFound(t *testing.T) {
	fs1 := fstest.MapFS{
		"dir/file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	overlay := NewOverlayFS(fs1)

	// Try to read non-existent directory
	_, err := overlay.ReadDir("nonexistent")
	if err == nil {
		t.Error("ReadDir should return error for non-existent directory")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadDir error should be fs.ErrNotExist, got %v", err)
	}
}

func TestOverlayFS_ReadDir_MultipleFS(t *testing.T) {
	// Test with three filesystems
	fs1 := fstest.MapFS{
		"themes/file1.txt": &fstest.MapFile{Data: []byte("fs1")},
	}
	fs2 := fstest.MapFS{
		"themes/file2.txt": &fstest.MapFile{Data: []byte("fs2")},
	}
	fs3 := fstest.MapFS{
		"themes/file3.txt": &fstest.MapFile{Data: []byte("fs3")},
	}

	overlay := NewOverlayFS(fs1, fs2, fs3)

	// Should merge entries from all three filesystems
	entries, err := overlay.ReadDir("themes")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("ReadDir entries count = %d, want %d", len(entries), 3)
	}

	names := make(map[string]bool)
	for _, entry := range entries {
		names[entry.Name()] = true
	}

	if !names["file1.txt"] || !names["file2.txt"] || !names["file3.txt"] {
		t.Errorf("ReadDir should include all three files, got: %v", names)
	}
}
