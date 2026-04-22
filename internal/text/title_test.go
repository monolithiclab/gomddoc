package text

import "testing"

func TestDeriveTitle(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/", "Home"},
		{".", "Home"},
		{"", "Home"},
		{"/docs/my-page.md", "My Page"},
		{"/docs/hello_world.md", "Hello World"},
		{"/README.md", "Readme"},
		{"/docs/getting-started.html", "Getting Started"},
		{"/simple", "Simple"},
		{"/multi-word-title.md", "Multi Word Title"},
		{"/mixed_hyphens-and_underscores.md", "Mixed Hyphens And Underscores"},
		{"/deeply/nested/path/document.md", "Document"},
		{"/UPPERCASE.md", "Uppercase"},
		{"/no-extension", "No Extension"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DeriveTitle(tt.path)
			if got != tt.want {
				t.Errorf("DeriveTitle(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
