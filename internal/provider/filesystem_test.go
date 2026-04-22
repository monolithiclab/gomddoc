package provider

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestNewFilesystemProvider(t *testing.T) {
	tests := []struct {
		name         string
		dir          string
		defaultIndex string
		dirIndex     bool
		wantErr      bool
	}{
		{
			name:         "valid directory",
			dir:          ".",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantErr:      false,
		},
		{
			name:         "valid directory with dir listing",
			dir:          ".",
			defaultIndex: "index.html",
			dirIndex:     true,
			wantErr:      false,
		},
		// Note: os.DirFS doesn't validate directory existence at creation time,
		// only when accessing files. This is by design in the fs.FS interface.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewFilesystemProvider(tt.dir, tt.defaultIndex, tt.dirIndex)

			if tt.wantErr {
				if err == nil {
					t.Error("NewFilesystemProvider() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewFilesystemProvider() error = %v, want nil", err)
			}

			if provider == nil {
				t.Fatal("Provider should not be nil")
			}

			if provider.DefaultIndex() != tt.defaultIndex {
				t.Errorf("DefaultIndex() = %q, want %q", provider.DefaultIndex(), tt.defaultIndex)
			}
		})
	}
}

func TestFilesystemProvider_ReadFile(t *testing.T) {
	// Create test file
	testContent := "# Test Content"
	testFile := "test_provider.md"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	provider, err := NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}
	defer provider.Close()

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
			path:         testFile,
			wantContent:  testContent,
			wantMimeType: "text/markdown; charset=utf-8",
			wantErr:      false,
		},
		{
			name:         "read with leading slash",
			path:         "/" + testFile,
			wantContent:  testContent,
			wantMimeType: "text/markdown; charset=utf-8",
			wantErr:      false,
		},
		{
			name:    "read nonexistent file",
			path:    "nonexistent.md",
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
	// Create test directory structure
	testDir := "test_dir_provider"
	err := os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Create README.md in test directory
	readmeContent := "# Directory README"
	err = os.WriteFile(testDir+"/README.md", []byte(readmeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}

	// Create subdirectory without README
	emptyDir := testDir + "/empty"
	err = os.Mkdir(emptyDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create empty directory: %v", err)
	}

	t.Run("directory with README.md (dirIndex=false)", func(t *testing.T) {
		provider, err := NewFilesystemProvider(".", "README.md", false)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}
		defer provider.Close()

		content, mimeType, err := provider.ReadFile(testDir)
		if err != nil {
			t.Fatalf("ReadFile() error = %v, want nil", err)
		}

		if string(content) != readmeContent {
			t.Errorf("ReadFile() content = %q, want %q", string(content), readmeContent)
		}

		if mimeType != "text/markdown; charset=utf-8" {
			t.Errorf("ReadFile() mimeType = %q, want text/markdown; charset=utf-8", mimeType)
		}
	})

	t.Run("directory without README.md (dirIndex=false)", func(t *testing.T) {
		provider, err := NewFilesystemProvider(".", "README.md", false)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}
		defer provider.Close()

		_, _, err = provider.ReadFile(emptyDir)
		if err == nil {
			t.Error("ReadFile() error = nil, want error")
		}

		if !errors.Is(err, ErrDirListingDisabled) {
			t.Errorf("ReadFile() error = %v, want ErrDirListingDisabled", err)
		}
	})

	t.Run("directory without README.md (dirIndex=true)", func(t *testing.T) {
		provider, err := NewFilesystemProvider(".", "README.md", true)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}
		defer provider.Close()

		content, mimeType, err := provider.ReadFile(emptyDir)
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
	})

	t.Run("directory with files (dirIndex=true)", func(t *testing.T) {
		// Create some files in the empty directory
		testFile1 := emptyDir + "/file1.txt"
		testFile2 := emptyDir + "/file2.md"
		err = os.WriteFile(testFile1, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		err = os.WriteFile(testFile2, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		provider, err := NewFilesystemProvider(".", "README.md", true)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}
		defer provider.Close()

		content, mimeType, err := provider.ReadFile(emptyDir)
		if err != nil {
			t.Fatalf("ReadFile() error = %v, want nil", err)
		}

		if mimeType != "text/markdown; charset=utf-8" {
			t.Errorf("ReadFile() mimeType = %q, want text/markdown; charset=utf-8", mimeType)
		}

		// Check that content contains file listings
		contentStr := string(content)
		if !strings.Contains(contentStr, "file1.txt") {
			t.Error("ReadFile() directory listing should contain file1.txt")
		}
		if !strings.Contains(contentStr, "file2.md") {
			t.Error("ReadFile() directory listing should contain file2.md")
		}
	})
}

func TestFilesystemProvider_PathCleaning(t *testing.T) {
	// Create test file
	testFile := "path_test.md"
	err := os.WriteFile(testFile, []byte("# Path Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	provider, err := NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}
	defer provider.Close()

	tests := []struct {
		name string
		path string
	}{
		{"normal path", testFile},
		{"path with leading slash", "/" + testFile},
		{"path with dots", "./" + testFile},
		{"path with double slash", "//" + testFile},
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
			if mimeType != "text/markdown; charset=utf-8" {
				t.Errorf("ReadFile() mimeType = %q, want text/markdown; charset=utf-8", mimeType)
			}
		})
	}
}

func TestFilesystemProvider_MimeTypeDetection(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		content      string
		wantMimeType string
	}{
		{
			name:         "markdown file",
			filename:     "test.md",
			content:      "# Test",
			wantMimeType: "text/markdown; charset=utf-8",
		},
		{
			name:         "html file",
			filename:     "test.html",
			content:      "<html></html>",
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "text file",
			filename:     "test.txt",
			content:      "plain text",
			wantMimeType: "text/plain; charset=utf-8",
		},
		{
			name:         "json file",
			filename:     "test.json",
			content:      "{}",
			wantMimeType: "application/json",
		},
		{
			name:         "unknown extension",
			filename:     "test.unknown",
			content:      "data",
			wantMimeType: "application/octet-stream",
		},
		{
			name:         "no extension",
			filename:     "testfile",
			content:      "data",
			wantMimeType: "application/octet-stream",
		},
	}

	provider, err := NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			err := os.WriteFile(tt.filename, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			defer os.Remove(tt.filename)

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
	// Create test file
	testFile := "stat_test.md"
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	provider, err := NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		checkErr func(error) bool
	}{
		{
			name:    "existing file",
			path:    testFile,
			wantErr: false,
		},
		{
			name:    "existing file with leading slash",
			path:    "/" + testFile,
			wantErr: false,
		},
		{
			name:    "nonexistent file",
			path:    "nonexistent.md",
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
	provider, err := NewFilesystemProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	t.Run("ReadFile wraps ErrNotFound", func(t *testing.T) {
		_, _, err := provider.ReadFile("nonexistent.md")
		if err == nil {
			t.Fatal("ReadFile() error = nil, want error")
		}

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("ReadFile() error should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("ReadFile wraps ErrDirListingDisabled", func(t *testing.T) {
		// Create test directory without README
		testDir := "test_err_wrap"
		err := os.Mkdir(testDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		defer os.RemoveAll(testDir)

		_, _, err = provider.ReadFile(testDir)
		if err == nil {
			t.Fatal("ReadFile() error = nil, want error")
		}

		if !errors.Is(err, ErrDirListingDisabled) {
			t.Errorf("ReadFile() error should wrap ErrDirListingDisabled, got: %v", err)
		}
	})

	t.Run("Stat wraps ErrNotFound", func(t *testing.T) {
		_, err := provider.Stat("nonexistent.md")
		if err == nil {
			t.Fatal("Stat() error = nil, want error")
		}

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Stat() error should wrap ErrNotFound, got: %v", err)
		}
	})

	t.Run("PathError contains operation and path info", func(t *testing.T) {
		_, _, err := provider.ReadFile("nonexistent.md")
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

		if pathErr.Path != "nonexistent.md" {
			t.Errorf("PathError.Path = %q, want %q", pathErr.Path, "nonexistent.md")
		}

		// Verify Error() method returns formatted string
		errStr := pathErr.Error()
		if !strings.Contains(errStr, "read") || !strings.Contains(errStr, "nonexistent.md") {
			t.Errorf("PathError.Error() = %q, should contain op and path", errStr)
		}
	})
}
