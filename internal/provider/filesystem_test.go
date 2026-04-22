package provider

import (
	"os"
	"strings"
	"testing"
)

func TestNewFilesystemProvider(t *testing.T) {
	provider, err := NewFilesystemProvider(".", "README.md")
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	if provider.DefaultIndex() != "README.md" {
		t.Errorf("Expected default index 'README.md', got %q", provider.DefaultIndex())
	}
}

func TestFilesystemProviderReadFile(t *testing.T) {
	// Create test file
	testContent := "# Test Content"
	err := os.WriteFile("test_provider.md", []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("test_provider.md")

	provider, err := NewFilesystemProvider(".", "README.md")
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}

	// Test reading existing file
	content, err := provider.ReadFile("test_provider.md")
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content %q, got %q", testContent, string(content))
	}

	// Test reading non-existent file
	_, err = provider.ReadFile("nonexistent.md")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.IsNotExist error, got: %v", err)
	}
}

func TestFilesystemProviderPathCleaning(t *testing.T) {
	// Create test file
	err := os.WriteFile("path_test.md", []byte("# Path Test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove("path_test.md")

	provider, err := NewFilesystemProvider(".", "README.md")
	if err != nil {
		t.Fatalf("Failed to create filesystem provider: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{"normal path", "path_test.md", false},
		{"path with leading slash", "/path_test.md", false},
		{"path with dots", "./path_test.md", false},
		{"path with double slash", "//path_test.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := provider.ReadFile(tt.path)
			if tt.shouldErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.shouldErr && !strings.Contains(string(content), "Path Test") {
				t.Error("should have read the correct file content")
			}
		})
	}
}
