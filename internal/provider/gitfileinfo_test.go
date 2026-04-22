package provider

import (
	"io/fs"
	"testing"
	"time"
)

func TestGitFileInfo_Interface(t *testing.T) {
	t.Parallel()

	// Compile-time interface check is done in the source file,
	// but this test verifies runtime behavior
	var _ fs.FileInfo = (*gitFileInfo)(nil)
}

func TestGitFileInfo_File(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	fi := &gitFileInfo{
		name:    "readme.md",
		size:    1024,
		mode:    0644,
		modTime: modTime,
		isDir:   false,
	}

	if fi.Name() != "readme.md" {
		t.Errorf("Name() = %q, want %q", fi.Name(), "readme.md")
	}

	if fi.Size() != 1024 {
		t.Errorf("Size() = %d, want %d", fi.Size(), 1024)
	}

	if fi.Mode() != 0644 {
		t.Errorf("Mode() = %v, want %v", fi.Mode(), fs.FileMode(0644))
	}

	if !fi.ModTime().Equal(modTime) {
		t.Errorf("ModTime() = %v, want %v", fi.ModTime(), modTime)
	}

	if fi.IsDir() {
		t.Error("IsDir() = true, want false for a file")
	}

	if fi.Sys() != nil {
		t.Errorf("Sys() = %v, want nil", fi.Sys())
	}
}

func TestGitFileInfo_Directory(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2024, 2, 20, 14, 0, 0, 0, time.UTC)
	fi := &gitFileInfo{
		name:    "docs",
		size:    0,
		mode:    fs.ModeDir | 0755,
		modTime: modTime,
		isDir:   true,
	}

	if fi.Name() != "docs" {
		t.Errorf("Name() = %q, want %q", fi.Name(), "docs")
	}

	if fi.Size() != 0 {
		t.Errorf("Size() = %d, want %d for directory", fi.Size(), 0)
	}

	expectedMode := fs.ModeDir | 0755
	if fi.Mode() != expectedMode {
		t.Errorf("Mode() = %v, want %v", fi.Mode(), expectedMode)
	}

	if !fi.ModTime().Equal(modTime) {
		t.Errorf("ModTime() = %v, want %v", fi.ModTime(), modTime)
	}

	if !fi.IsDir() {
		t.Error("IsDir() = false, want true for a directory")
	}

	if fi.Sys() != nil {
		t.Errorf("Sys() = %v, want nil", fi.Sys())
	}
}

func TestGitDirEntry_Interface(t *testing.T) {
	t.Parallel()

	// Compile-time interface check is done in the source file,
	// but this test verifies runtime behavior
	var _ fs.DirEntry = (*gitDirEntry)(nil)
}

func TestGitDirEntry_File(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC)
	entry := &gitDirEntry{
		name:     "main.go",
		isDir:    false,
		fileMode: 0644,
		size:     2048,
		modTime:  modTime,
	}

	if entry.Name() != "main.go" {
		t.Errorf("Name() = %q, want %q", entry.Name(), "main.go")
	}

	if entry.IsDir() {
		t.Error("IsDir() = true, want false for a file")
	}

	if entry.Type() != 0 {
		t.Errorf("Type() = %v, want 0 for regular file", entry.Type())
	}

	// Test Info() method
	info, err := entry.Info()
	if err != nil {
		t.Fatalf("Info() error = %v, want nil", err)
	}

	if info.Name() != "main.go" {
		t.Errorf("Info().Name() = %q, want %q", info.Name(), "main.go")
	}

	if info.Size() != 2048 {
		t.Errorf("Info().Size() = %d, want %d", info.Size(), 2048)
	}

	if info.Mode() != 0644 {
		t.Errorf("Info().Mode() = %v, want %v", info.Mode(), fs.FileMode(0644))
	}

	if info.IsDir() {
		t.Error("Info().IsDir() = true, want false")
	}

	if !info.ModTime().Equal(modTime) {
		t.Errorf("Info().ModTime() = %v, want %v", info.ModTime(), modTime)
	}
}

func TestGitDirEntry_Directory(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2024, 4, 5, 12, 30, 0, 0, time.UTC)
	entry := &gitDirEntry{
		name:     "internal",
		isDir:    true,
		fileMode: fs.ModeDir | 0755,
		size:     0,
		modTime:  modTime,
	}

	if entry.Name() != "internal" {
		t.Errorf("Name() = %q, want %q", entry.Name(), "internal")
	}

	if !entry.IsDir() {
		t.Error("IsDir() = false, want true for a directory")
	}

	if entry.Type() != fs.ModeDir {
		t.Errorf("Type() = %v, want %v for directory", entry.Type(), fs.ModeDir)
	}

	// Test Info() method
	info, err := entry.Info()
	if err != nil {
		t.Fatalf("Info() error = %v, want nil", err)
	}

	if info.Name() != "internal" {
		t.Errorf("Info().Name() = %q, want %q", info.Name(), "internal")
	}

	if info.Size() != 0 {
		t.Errorf("Info().Size() = %d, want %d for directory", info.Size(), 0)
	}

	expectedMode := fs.ModeDir | 0755
	if info.Mode() != expectedMode {
		t.Errorf("Info().Mode() = %v, want %v", info.Mode(), expectedMode)
	}

	if !info.IsDir() {
		t.Error("Info().IsDir() = false, want true")
	}
}

func TestGitDirEntry_InfoNeverFails(t *testing.T) {
	t.Parallel()

	// The Info() method should never fail for gitDirEntry
	// since all data is already available
	entry := &gitDirEntry{
		name:     "test.txt",
		isDir:    false,
		fileMode: 0644,
		size:     100,
		modTime:  time.Now(),
	}

	info, err := entry.Info()
	if err != nil {
		t.Errorf("Info() error = %v, want nil", err)
	}
	if info == nil {
		t.Error("Info() returned nil, want non-nil")
	}
}
