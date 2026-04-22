package common

import "testing"

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"git protocol", "git://github.com/user/repo", true},
		{"git+ssh protocol", "git+ssh://git@github.com/user/repo", true},
		{"git+https protocol", "git+https://github.com/user/repo", true},
		{"http protocol", "http://github.com/user/repo", false},
		{"https protocol", "https://github.com/user/repo", false},
		{"ssh protocol", "ssh://git@github.com/user/repo", false},
		{"file path", "/path/to/repo", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitURL(tt.input); got != tt.want {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
