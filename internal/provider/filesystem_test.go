package provider

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNewFilesystemProvider(t *testing.T) {
	// Simple test to ensure the wrapper function works
	// We use "." as a safe directory that always exists
	p, err := NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("NewFilesystemProvider failed: %v", err)
	}
	if p == nil {
		t.Fatal("NewFilesystemProvider returned nil")
	}
}

func TestNewFilesystemProviderFromFS(t *testing.T) {
	// Test using in-memory filesystem
	mapFS := fstest.MapFS{
		"test.txt": &fstest.MapFile{Data: []byte("content")},
	}

	p, err := NewFilesystemProviderFromFS(mapFS, "README.md", false)
	if err != nil {
		t.Fatalf("NewFilesystemProviderFromFS failed: %v", err)
	}
	if p == nil {
		t.Fatal("NewFilesystemProviderFromFS returned nil")
	}
}

func TestFilesystemProvider_ReadFile(t *testing.T) {
	testContent := "Test Content"
	mapFS := fstest.MapFS{
		"test_provider.txt": &fstest.MapFile{Data: []byte(testContent)},
	}

	provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		wantContent  string
		wantMimeType string
		wantErr      bool
		checkErr     func(error) bool
	}{
		{
			name:         "read existing file",
			path:         "test_provider.txt",
			wantContent:  testContent,
			wantMimeType: "text/plain; charset=utf-8",
			wantErr:      false,
		},
		{
			name:         "read with leading slash",
			path:         "/test_provider.txt",
			wantContent:  testContent,
			wantMimeType: "text/plain; charset=utf-8",
			wantErr:      false,
		},
		{
			name:    "read nonexistent file",
			path:    "nonexistent.txt",
			wantErr: true,
			checkErr: func(err error) bool {
				return errors.Is(err, ErrNotFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, mimeType, err := provider.ReadFile(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("ReadFile() error = nil, want error")
					return
				}
				if tt.checkErr != nil && !tt.checkErr(err) {
					t.Errorf("ReadFile() error = %v, want error matching checkErr", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadFile() error = %v, want nil", err)
			}

			if string(content) != tt.wantContent {
				t.Errorf("ReadFile() content = %q, want %q", string(content), tt.wantContent)
			}

			if mimeType != tt.wantMimeType {
				t.Errorf("ReadFile() mimeType = %q, want %q", mimeType, tt.wantMimeType)
			}
		})
	}
}

func TestFilesystemProvider_ReadFile_Directory(t *testing.T) {
	readmeContent := "Directory README"
	mapFS := fstest.MapFS{
		"test_dir_provider/README.txt":       &fstest.MapFile{Data: []byte(readmeContent)},
		"test_dir_provider/empty/":           &fstest.MapFile{Mode: 0755 | 040000}, // Directory
		"test_dir_provider/empty/file1.txt":  &fstest.MapFile{Data: []byte("test")},
		"test_dir_provider/empty/file2.html": &fstest.MapFile{Data: []byte("test")},
	}

	t.Run("directory with README.txt (dirIndex=false)", func(t *testing.T) {
		provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		content, mimeType, err := provider.ReadFile("test_dir_provider")
		if err != nil {
			t.Fatalf("ReadFile() error = %v, want nil", err)
		}

		if string(content) != readmeContent {
			t.Errorf("ReadFile() content = %q, want %q", string(content), readmeContent)
		}

		if mimeType != "text/plain; charset=utf-8" {
			t.Errorf("ReadFile() mimeType = %q, want text/plain; charset=utf-8", mimeType)
		}
	})

	t.Run("directory without README.txt (dirIndex=false)", func(t *testing.T) {
		provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		// fstest.MapFS implicitly handles directories if files exist within them
		// checking "test_dir_provider/empty" which has files but no README
		_, _, err = provider.ReadFile("test_dir_provider/empty")
		if err == nil {
			t.Error("ReadFile() error = nil, want error")
		}

		if !errors.Is(err, ErrDirListingDisabled) {
			t.Errorf("ReadFile() error = %v, want ErrDirListingDisabled", err)
		}
	})

	t.Run("directory without README.txt (dirIndex=true)", func(t *testing.T) {
		provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", true)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		content, mimeType, err := provider.ReadFile("test_dir_provider/empty")
		if err != nil {
			t.Fatalf("ReadFile() error = %v, want nil", err)
		}

		if mimeType != "text/markdown; charset=utf-8" {
			t.Errorf("ReadFile() mimeType = %q, want text/markdown; charset=utf-8", mimeType)
		}

		// Check that content contains directory listing markdown
		contentStr := string(content)
		if !strings.Contains(contentStr, "# Index") {
			t.Error("ReadFile() directory listing should contain heading")
		}

		// Check for file listings
		if !strings.Contains(contentStr, "file1.txt") {
			t.Error("ReadFile() directory listing should contain file1.txt")
		}
		if !strings.Contains(contentStr, "file2.html") {
			t.Error("ReadFile() directory listing should contain file2.html")
		}
	})
}

func TestFilesystemProvider_PathCleaning(t *testing.T) {
	mapFS := fstest.MapFS{
		"path_test.txt": &fstest.MapFile{Data: []byte("Path Test")},
	}

	provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"normal path", "path_test.txt"},
		{"path with leading slash", "/path_test.txt"},
		{"path with dots", "./path_test.txt"},
		{"path with double slash", "//path_test.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, mimeType, err := provider.ReadFile(tt.path)
			if err != nil {
				t.Errorf("ReadFile() error = %v, want nil", err)
			}
			if !strings.Contains(string(content), "Path Test") {
				t.Error("ReadFile() should read the correct file content")
			}
			if mimeType != "text/plain; charset=utf-8" {
				t.Errorf("ReadFile() mimeType = %q, want text/plain; charset=utf-8", mimeType)
			}
		})
	}
}

func TestFilesystemProvider_MimeTypeDetection(t *testing.T) {
	// Tests use only built-in MIME types (no custom registration required)
	// We create a map with all needed files
	mapFS := fstest.MapFS{
		"test.html":    &fstest.MapFile{Data: []byte("<html></html>")},
		"test.txt":     &fstest.MapFile{Data: []byte("plain text")},
		"test.json":    &fstest.MapFile{Data: []byte("{}")},
		"test.css":     &fstest.MapFile{Data: []byte("body {}")},
		"test.js":      &fstest.MapFile{Data: []byte("console.log()")},
		"test.unknown": &fstest.MapFile{Data: []byte("data")},
		"testfile":     &fstest.MapFile{Data: []byte("data")},
	}

	provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name         string
		filename     string
		wantMimeType string
	}{
		{
			name:         "html file",
			filename:     "test.html",
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "text file",
			filename:     "test.txt",
			wantMimeType: "text/plain; charset=utf-8",
		},
		{
			name:         "json file",
			filename:     "test.json",
			wantMimeType: "application/json",
		},
		{
			name:         "css file",
			filename:     "test.css",
			wantMimeType: "text/css; charset=utf-8",
		},
		{
			name:         "javascript file",
			filename:     "test.js",
			wantMimeType: "text/javascript; charset=utf-8",
		},
		{
			name:         "unknown extension",
			filename:     "test.unknown",
			wantMimeType: "application/octet-stream",
		},
		{
			name:         "no extension",
			filename:     "testfile",
			wantMimeType: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mimeType, err := provider.ReadFile(tt.filename)
			if err != nil {
				t.Fatalf("ReadFile() error = %v, want nil", err)
			}

			if mimeType != tt.wantMimeType {
				t.Errorf("ReadFile() mimeType = %q, want %q", mimeType, tt.wantMimeType)
			}
		})
	}
}

func TestFilesystemProvider_Stat(t *testing.T) {
	mapFS := fstest.MapFS{
		"stat_test.txt": &fstest.MapFile{Data: []byte("test")},
	}

	provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		checkErr func(error) bool
	}{
		{
			name:    "existing file",
			path:    "stat_test.txt",
			wantErr: false,
		},
		{
			name:    "existing file with leading slash",
			path:    "/stat_test.txt",
			wantErr: false,
		},
		{
			name:    "nonexistent file",
			path:    "nonexistent.txt",
			wantErr: true,
			checkErr: func(err error) bool {
				return errors.Is(err, ErrNotFound)
			},
		},
		{
			name:    "current directory",
			path:    "/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := provider.Stat(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("Stat() error = nil, want error")
					return
				}
				if tt.checkErr != nil && !tt.checkErr(err) {
					t.Errorf("Stat() error = %v, want error matching checkErr", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Stat() error = %v, want nil", err)
			}

			if info == nil {
				t.Error("Stat() info = nil, want non-nil")
			}
		})
	}
}

func TestFilesystemProvider_ErrorWrapping(t *testing.T) {
	mapFS := fstest.MapFS{
		"test_err_wrap/file.txt": &fstest.MapFile{Data: []byte("content")},
	}

	provider, err := NewFilesystemProviderFromFS(mapFS, "README.txt", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	t.Run("ReadFile wraps ErrNotFound", func(t *testing.T) {
		_, _, err := provider.ReadFile("nonexistent.txt")
		if err == nil {
			t.Fatal("ReadFile() error = nil, want error")
		}

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("ReadFile() error should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("ReadFile wraps ErrDirListingDisabled", func(t *testing.T) {
		// "test_err_wrap" is a directory because it contains files in mapFS
		_, _, err := provider.ReadFile("test_err_wrap")
		if err == nil {
			t.Fatal("ReadFile() error = nil, want error")
		}

		if !errors.Is(err, ErrDirListingDisabled) {
			t.Errorf("ReadFile() error should wrap ErrDirListingDisabled, got: %v", err)
		}
	})

	t.Run("Stat wraps ErrNotFound", func(t *testing.T) {
		_, err := provider.Stat("nonexistent.txt")
		if err == nil {
			t.Fatal("Stat() error = nil, want error")
		}

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Stat() error should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("PathError contains operation and path info", func(t *testing.T) {
		_, _, err := provider.ReadFile("nonexistent.txt")
		if err == nil {
			t.Fatal("ReadFile() error = nil, want error")
		}

		var pathErr *PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("error should be *PathError, got %T", err)
		}

		if pathErr.Op != "read" {
			t.Errorf("PathError.Op = %q, want %q", pathErr.Op, "read")
		}

		if pathErr.Path != "nonexistent.txt" {
			t.Errorf("PathError.Path = %q, want %q", pathErr.Path, "nonexistent.txt")
		}

		// Verify Error() method returns formatted string
		errStr := pathErr.Error()
		if !strings.Contains(errStr, "read") || !strings.Contains(errStr, "nonexistent.txt") {
			t.Errorf("PathError.Error() = %q, should contain op and path", errStr)
		}
	})
}
