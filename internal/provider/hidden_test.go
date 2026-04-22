package provider

import "testing"

func TestIsHiddenPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		hidden bool
	}{
		{"regular file", "guide/getting-started.md", false},
		{"regular nested", "docs/api/reference.md", false},
		{"root index", "README.md", false},
		{"leading slash", "/guide/config.md", false},
		{"empty", "", false},
		{"slash only", "/", false},

		// Hidden paths
		{"dotfile", ".env", true},
		{"dotdir", ".git/config", true},
		{"dotdir nested", ".gomddoc/config.yml", true},
		{"hidden in subdir", "docs/.secret/notes.md", true},
		{"leading slash dotfile", "/.env", true},
		{"leading slash dotdir", "/.git/HEAD", true},
		{"dotfile deep", "a/b/.hidden", true},

		// .well-known exception (RFC 8615)
		{"well-known root", ".well-known/security.txt", false},
		{"well-known nested", ".well-known/acme-challenge/token", false},

		// Tricky cases
		{"dot only segment", "docs/./file.md", true},
		{"double dot segment", "docs/../.env", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsHiddenPath(tt.path)
			if got != tt.hidden {
				t.Errorf("IsHiddenPath(%q) = %v, want %v", tt.path, got, tt.hidden)
			}
		})
	}
}
