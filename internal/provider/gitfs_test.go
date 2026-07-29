package provider

import (
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"
)

func TestGitTreeFS_Open_File(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"readme.md":     "# Hello",
		"docs/guide.md": "# Guide",
	})
	tree := mustGetTree(t, repo)
	modTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: modTime}}

	f, err := gfs.Open("readme.md")
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	defer f.Close()

	// Read content
	buf := make([]byte, 64)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Read() error = %v", err)
	}
	if string(buf[:n]) != "# Hello" {
		t.Errorf("Read() content = %q, want %q", string(buf[:n]), "# Hello")
	}

	// Verify Stat
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Name() != "readme.md" {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), "readme.md")
	}
	if info.Size() != int64(len("# Hello")) {
		t.Errorf("Stat().Size() = %d, want %d", info.Size(), len("# Hello"))
	}
	if info.IsDir() {
		t.Error("Stat().IsDir() = true, want false")
	}
	if info.ModTime() != modTime {
		t.Errorf("Stat().ModTime() = %v, want %v", info.ModTime(), modTime)
	}
}

func TestGitTreeFS_Open_Directory(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"docs/guide.md": "# Guide",
		"docs/api.md":   "# API",
	})
	tree := mustGetTree(t, repo)
	modTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: modTime}}

	f, err := gfs.Open("docs")
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Error("Stat().IsDir() = false, want true")
	}
	if info.Name() != "docs" {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), "docs")
	}
}

func TestGitTreeFS_Open_Root(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"readme.md": "# Hello",
	})
	tree := mustGetTree(t, repo)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: time.Now()}}

	f, err := gfs.Open(".")
	if err != nil {
		t.Fatalf("Open(\".\") error = %v, want nil", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Error("Stat().IsDir() = false, want true for root")
	}
	if info.Name() != "." {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), ".")
	}
}

func TestGitTreeFS_Open_NotExist(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"readme.md": "# Hello",
	})
	tree := mustGetTree(t, repo)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: time.Now()}}

	_, err := gfs.Open("nonexistent.md")
	if err == nil {
		t.Fatal("Open() error = nil, want error")
	}

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error type = %T, want *fs.PathError", err)
	}
	if !errors.Is(pathErr.Err, fs.ErrNotExist) {
		t.Errorf("PathError.Err = %v, want fs.ErrNotExist", pathErr.Err)
	}
}

func TestGitTreeFS_Open_InvalidPath(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"readme.md": "# Hello",
	})
	tree := mustGetTree(t, repo)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: time.Now()}}

	tests := []struct {
		name string
		path string
	}{
		{"parent traversal", "../foo"},
		{"absolute path", "/readme.md"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := gfs.Open(tt.path)
			if err == nil {
				t.Fatal("Open() error = nil, want error")
			}

			var pathErr *fs.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("error type = %T, want *fs.PathError", err)
			}
			if !errors.Is(pathErr.Err, fs.ErrInvalid) {
				t.Errorf("PathError.Err = %v, want fs.ErrInvalid", pathErr.Err)
			}
		})
	}
}

func TestGitBlobFile_ReadStatClose(t *testing.T) {
	t.Parallel()

	content := "hello world content"
	repo := createTestRepo(t, map[string]string{
		"file.txt": content,
	})
	tree := mustGetTree(t, repo)
	modTime := time.Date(2024, 3, 15, 8, 30, 0, 0, time.UTC)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: modTime}}

	f, err := gfs.Open("file.txt")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Read full content
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != content {
		t.Errorf("Read content = %q, want %q", string(data), content)
	}

	// Stat
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Name() != "file.txt" {
		t.Errorf("Name() = %q, want %q", info.Name(), "file.txt")
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Size() = %d, want %d", info.Size(), len(content))
	}
	if info.IsDir() {
		t.Error("IsDir() = true, want false")
	}
	if info.ModTime() != modTime {
		t.Errorf("ModTime() = %v, want %v", info.ModTime(), modTime)
	}

	// Close
	if err := f.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestGitDirFile_ReadError(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"docs/guide.md": "# Guide",
	})
	tree := mustGetTree(t, repo)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: time.Now()}}

	f, err := gfs.Open("docs")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer f.Close()

	buf := make([]byte, 64)
	_, err = f.Read(buf)
	if err == nil {
		t.Fatal("Read() on directory error = nil, want error")
	}

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error type = %T, want *fs.PathError", err)
	}
	if !errors.Is(pathErr.Err, fs.ErrInvalid) {
		t.Errorf("PathError.Err = %v, want fs.ErrInvalid", pathErr.Err)
	}
}

func TestGitDirFile_ReadDir(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"docs/alpha.md": "# Alpha",
		"docs/beta.md":  "# Beta",
		"docs/.hidden":  "secret",
		"docs/.gitkeep": "",
		"docs/gamma.md": "# Gamma",
	})
	tree := mustGetTree(t, repo)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: time.Now()}}

	t.Run("n<=0 returns all non-dotfile entries", func(t *testing.T) {
		f, err := gfs.Open("docs")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer f.Close()

		dirFile, ok := f.(fs.ReadDirFile)
		if !ok {
			t.Fatal("opened directory does not implement fs.ReadDirFile")
		}

		entries, err := dirFile.ReadDir(-1)
		if err != nil {
			t.Fatalf("ReadDir(-1) error = %v", err)
		}

		// Should not contain dotfiles
		for _, e := range entries {
			if e.Name()[0] == '.' {
				t.Errorf("ReadDir returned dotfile %q, want dotfiles filtered", e.Name())
			}
		}

		// Should contain alpha, beta, gamma (3 non-dotfile entries)
		if len(entries) != 3 {
			t.Errorf("ReadDir() returned %d entries, want 3", len(entries))
		}

		// Second call with n<=0 should return empty slice with nil error
		// per io/fs.ReadDirFile spec: n<=0 returns all remaining entries with nil error.
		entries2, err := dirFile.ReadDir(0)
		if err != nil {
			t.Errorf("second ReadDir(0) error = %v, want nil", err)
		}
		if len(entries2) != 0 {
			t.Errorf("second ReadDir(0) returned %d entries, want 0", len(entries2))
		}
	})

	t.Run("n>0 returns batches", func(t *testing.T) {
		f, err := gfs.Open("docs")
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer f.Close()

		dirFile, ok := f.(fs.ReadDirFile)
		if !ok {
			t.Fatal("opened directory does not implement fs.ReadDirFile")
		}

		// Read 2 entries at a time from 3 total non-dotfile entries
		batch1, err := dirFile.ReadDir(2)
		if err != nil {
			t.Fatalf("ReadDir(2) first batch error = %v", err)
		}
		if len(batch1) != 2 {
			t.Errorf("first batch length = %d, want 2", len(batch1))
		}

		batch2, err := dirFile.ReadDir(2)
		if err != nil {
			t.Fatalf("ReadDir(2) second batch error = %v", err)
		}
		if len(batch2) != 1 {
			t.Errorf("second batch length = %d, want 1", len(batch2))
		}

		// No more entries
		_, err = dirFile.ReadDir(2)
		if !errors.Is(err, io.EOF) {
			t.Errorf("ReadDir(2) after exhausted error = %v, want io.EOF", err)
		}
	})
}

func TestGitDirFile_ReadDir_Sequential(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"a.md": "# A",
		"b.md": "# B",
		"c.md": "# C",
		"d.md": "# D",
	})
	tree := mustGetTree(t, repo)
	gfs := &gitTreeFS{state: &gitTreeState{tree: tree, modTime: time.Now()}}

	// Open root directory
	f, err := gfs.Open(".")
	if err != nil {
		t.Fatalf("Open(\".\") error = %v", err)
	}
	defer f.Close()

	dirFile, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("root does not implement fs.ReadDirFile")
	}

	// Read one at a time and collect names
	var names []string
	for {
		entries, err := dirFile.ReadDir(1)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadDir(1) error = %v", err)
		}
		for _, e := range entries {
			names = append(names, e.Name())
		}
	}

	if len(names) != 4 {
		t.Errorf("collected %d entries, want 4", len(names))
	}

	// Verify all files are present (order may vary by git tree)
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range []string{"a.md", "b.md", "c.md", "d.md"} {
		if !nameSet[want] {
			t.Errorf("missing entry %q in ReadDir results", want)
		}
	}
}

func TestGitTreeFS_Open_AfterClose(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"readme.md": "# Hello",
	})
	tree := mustGetTree(t, repo)
	state := &gitTreeState{tree: tree, modTime: time.Now()}
	gfs := &gitTreeFS{state: state}

	// Verify it works before close
	f, err := gfs.Open("readme.md")
	if err != nil {
		t.Fatalf("Open() before close error = %v, want nil", err)
	}
	f.Close()

	// Simulate provider close: nil the tree under lock
	state.mu.Lock()
	state.tree = nil
	state.mu.Unlock()

	// All subsequent Open calls should return fs.ErrClosed
	_, err = gfs.Open("readme.md")
	if err == nil {
		t.Fatal("Open() after close error = nil, want error")
	}

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error type = %T, want *fs.PathError", err)
	}
	if !errors.Is(pathErr.Err, fs.ErrClosed) {
		t.Errorf("PathError.Err = %v, want fs.ErrClosed", pathErr.Err)
	}

	// Root path should also fail
	_, err = gfs.Open(".")
	if err == nil {
		t.Fatal("Open(\".\") after close error = nil, want error")
	}
	if !errors.As(err, &pathErr) {
		t.Fatalf("error type = %T, want *fs.PathError", err)
	}
	if !errors.Is(pathErr.Err, fs.ErrClosed) {
		t.Errorf("PathError.Err = %v, want fs.ErrClosed", pathErr.Err)
	}
}
