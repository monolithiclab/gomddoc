package provider

import (
	"errors"
	"testing"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dir          string
		defaultIndex string
		dirIndex     bool
		wantType     string
		wantErr      bool
		wantErrType  error
	}{
		{
			name:         "filesystem provider for current dir",
			dir:          ".",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantType:     "*provider.FilesystemProvider",
		},
		{
			name:         "git provider for git+https URL",
			dir:          "git+https://github.com/user/repo",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantType:     "*provider.GitProvider",
		},
		{
			name:         "git provider for git+ssh URL",
			dir:          "git+ssh://git@github.com/org/docs#main",
			defaultIndex: "README.md",
			dirIndex:     true,
			wantType:     "*provider.GitProvider",
		},
		{
			name:         "git provider for git protocol URL",
			dir:          "git://gitlab.com/group/project",
			defaultIndex: "index.md",
			dirIndex:     false,
			wantType:     "*provider.GitProvider",
		},
		{
			name:         "git provider with fragment",
			dir:          "git+https://github.com/user/mono#main:docs",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantType:     "*provider.GitProvider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := NewProvider(tt.dir, tt.defaultIndex, tt.dirIndex)

			if tt.wantErr {
				if err == nil {
					t.Error("NewProvider() error = nil, want error")
				}
				if tt.wantErrType != nil && !errors.Is(err, tt.wantErrType) {
					t.Errorf("NewProvider() error = %v, want %v", err, tt.wantErrType)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewProvider() error = %v, want nil", err)
			}

			if p == nil {
				t.Fatal("NewProvider() returned nil, want non-nil")
			}

			// Check provider type
			gotType := getTypeName(p)
			if gotType != tt.wantType {
				t.Errorf("NewProvider() type = %s, want %s", gotType, tt.wantType)
			}

			// Verify DefaultIndex is set correctly
			if p.DefaultIndex() != tt.defaultIndex {
				t.Errorf("DefaultIndex() = %q, want %q", p.DefaultIndex(), tt.defaultIndex)
			}

			// Clean up
			p.Close()
		})
	}
}

func TestNewProvider_FilesystemProviderBehavior(t *testing.T) {
	t.Parallel()

	// Create a provider for current directory (should be FilesystemProvider)
	p, err := NewProvider(".", "README.md", false)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	defer p.Close()

	// Verify it can stat the current directory
	info, err := p.Stat(t.Context(), "/")
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if !info.IsDir() {
		t.Error("Stat('/') should return a directory")
	}
}

func TestNewProvider_GitProviderBehavior(t *testing.T) {
	t.Parallel()

	// Create a Git provider (won't actually clone until ReadFile/Stat is called)
	p, err := NewProvider("git+https://github.com/user/repo#main:docs", "README.md", true)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	defer p.Close()

	// Verify configuration is correct
	if p.DefaultIndex() != "README.md" {
		t.Errorf("DefaultIndex() = %q, want %q", p.DefaultIndex(), "README.md")
	}

	// Git provider is lazy - Close should work without having cloned
	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

// getTypeName returns the type name as a string for comparison
func getTypeName(p Provider) string {
	switch p.(type) {
	case *FilesystemProvider:
		return "*provider.FilesystemProvider"
	case *GitProvider:
		return "*provider.GitProvider"
	default:
		return "unknown"
	}
}
