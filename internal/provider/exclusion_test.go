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

func TestIsExcludedPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		patterns []string
		excluded bool
	}{
		// No patterns — nothing excluded
		{"empty patterns", "docs/guide.md", nil, false},
		{"empty patterns slice", "docs/guide.md", []string{}, false},

		// Segment-level glob patterns (no "/" in pattern)
		{"glob match extension", "docs/notes.bak", []string{"*.bak"}, true},
		{"glob match nested", "a/b/notes.bak", []string{"*.bak"}, true},
		{"glob no match", "docs/notes.md", []string{"*.bak"}, false},
		{"exact filename", "TODO.md", []string{"TODO.md"}, true},
		{"exact filename nested", "docs/TODO.md", []string{"TODO.md"}, true},
		{"exact filename no match", "docs/README.md", []string{"TODO.md"}, false},
		{"question mark glob", "docs/file1.md", []string{"file?.md"}, true},
		{"character class", "docs/fileA.md", []string{"file[A-Z].md"}, true},
		{"dir name as segment", "drafts/secret.md", []string{"drafts"}, true},
		{"dir name nested", "docs/drafts/secret.md", []string{"drafts"}, true},

		// Full path patterns (contain "/")
		{"dir prefix", "drafts/secret.md", []string{"drafts/"}, true},
		{"dir prefix nested", "drafts/sub/deep.md", []string{"drafts/"}, true},
		{"dir prefix no match", "docs/drafts.md", []string{"drafts/"}, false},
		{"dir prefix exact dir", "drafts", []string{"drafts/"}, true},
		{"path glob", "docs/internal/secret.md", []string{"docs/internal/*.md"}, true},
		{"path glob no match", "other/internal/secret.md", []string{"docs/internal/*.md"}, false},

		// Multiple patterns
		{"multi first matches", "TODO.md", []string{"TODO.md", "*.bak"}, true},
		{"multi second matches", "notes.bak", []string{"TODO.md", "*.bak"}, true},
		{"multi none match", "README.md", []string{"TODO.md", "*.bak"}, false},

		// Edge cases
		{"empty path", "", []string{"*.md"}, false},
		{"leading slash", "/docs/TODO.md", []string{"TODO.md"}, true},
		{"empty pattern in list", "docs/file.md", []string{""}, false},
		{"root path", "/", []string{"*.md"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsExcludedPath(tt.path, tt.patterns)
			if got != tt.excluded {
				t.Errorf("IsExcludedPath(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.excluded)
			}
		})
	}
}
