package provider

import (
	"testing"
)

func TestIsGitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid Git URLs
		{"git protocol", "git://github.com/user/repo", true},
		{"git+ssh protocol", "git+ssh://git@github.com/user/repo", true},
		{"git+https protocol", "git+https://github.com/user/repo", true},
		{"git with .git suffix", "git://github.com/user/repo.git", true},
		{"git+ssh with fragment", "git+ssh://git@github.com/org/docs#main", true},
		{"git+https with fragment and subdir", "git+https://github.com/user/mono#main:docs", true},

		// Not Git URLs
		{"empty string", "", false},
		{"plain https", "https://github.com/user/repo", false},
		{"plain ssh", "ssh://git@github.com/user/repo", false},
		{"file path", "/path/to/dir", false},
		{"relative path", "./docs", false},
		{"windows path", "C:\\Users\\docs", false},
		{"http URL", "http://github.com/user/repo", false},
		{"ftp URL", "ftp://example.com/file", false},
		{"partial match git", "git-something://host/path", false},
		{"partial match prefix", "notgit://host/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsGitURL(tt.input)
			if got != tt.want {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseGitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantRef    string
		wantSubdir string
		wantErr    bool
	}{
		// Basic URLs
		{
			name:       "simple git+https",
			input:      "git+https://github.com/user/repo",
			wantRef:    "HEAD",
			wantSubdir: "",
		},
		{
			name:       "simple git+ssh",
			input:      "git+ssh://git@github.com/org/docs",
			wantRef:    "HEAD",
			wantSubdir: "",
		},
		{
			name:       "simple git protocol",
			input:      "git://gitlab.com/group/project",
			wantRef:    "HEAD",
			wantSubdir: "",
		},

		// With .git suffix
		{
			name:       "with .git suffix",
			input:      "git+https://github.com/user/repo.git",
			wantRef:    "HEAD",
			wantSubdir: "",
		},

		// With branch reference
		{
			name:       "with branch",
			input:      "git+ssh://git@github.com/org/docs#main",
			wantRef:    "main",
			wantSubdir: "",
		},
		{
			name:       "with develop branch",
			input:      "git+https://github.com/user/docs#develop",
			wantRef:    "develop",
			wantSubdir: "",
		},

		// With tag reference
		{
			name:       "with tag",
			input:      "git://gitlab.com/group/project.git#v2.0.0",
			wantRef:    "v2.0.0",
			wantSubdir: "",
		},

		// With commit hash
		{
			name:       "with short commit hash",
			input:      "git+https://github.com/user/docs#a1b2c3d",
			wantRef:    "a1b2c3d",
			wantSubdir: "",
		},
		{
			name:       "with full commit hash",
			input:      "git+https://github.com/user/docs#a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			wantRef:    "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			wantSubdir: "",
		},

		// With subdirectory
		{
			name:       "with branch and subdir",
			input:      "git+https://github.com/user/monorepo#main:docs",
			wantRef:    "main",
			wantSubdir: "docs",
		},
		{
			name:       "with branch and nested subdir",
			input:      "git+https://github.com/user/mono#main:docs/api",
			wantRef:    "main",
			wantSubdir: "docs/api",
		},
		{
			name:       "with subdir leading slash stripped",
			input:      "git+https://github.com/user/mono#main:/docs/",
			wantRef:    "main",
			wantSubdir: "docs",
		},

		// Empty fragment parts
		{
			name:       "empty ref defaults to HEAD",
			input:      "git+https://github.com/user/repo#",
			wantRef:    "HEAD",
			wantSubdir: "",
		},
		{
			name:       "empty ref with subdir",
			input:      "git+https://github.com/user/repo#:docs",
			wantRef:    "HEAD",
			wantSubdir: "docs",
		},

		// With port
		{
			name:       "ssh with custom port",
			input:      "git+ssh://git@git.company.com:2222/team/docs",
			wantRef:    "HEAD",
			wantSubdir: "",
		},

		// Error cases
		{
			name:    "empty URL",
			input:   "",
			wantErr: true,
		},
		{
			name:    "unsupported scheme https",
			input:   "https://github.com/user/repo",
			wantErr: true,
		},
		{
			name:    "unsupported scheme ssh",
			input:   "ssh://git@github.com/user/repo",
			wantErr: true,
		},
		{
			name:    "unsupported scheme http",
			input:   "http://github.com/user/repo",
			wantErr: true,
		},
		{
			name:    "missing host",
			input:   "git+ssh:///path/only",
			wantErr: true,
		},
		{
			name:    "file path",
			input:   "/path/to/repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := parseGitURL(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseGitURL(%q) error = nil, want error", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseGitURL(%q) error = %v, want nil", tt.input, err)
			}

			if parsed == nil {
				t.Fatalf("parseGitURL(%q) returned nil, want non-nil", tt.input)
			}

			if parsed.Ref != tt.wantRef {
				t.Errorf("parseGitURL(%q).Ref = %q, want %q", tt.input, parsed.Ref, tt.wantRef)
			}

			if parsed.Subdir != tt.wantSubdir {
				t.Errorf("parseGitURL(%q).Subdir = %q, want %q", tt.input, parsed.Subdir, tt.wantSubdir)
			}

			if parsed.Endpoint == nil {
				t.Errorf("parseGitURL(%q).Endpoint = nil, want non-nil", tt.input)
			}
		})
	}
}

func TestParseGitURL_EndpointProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantProtocol string
	}{
		{
			name:         "git+ssh becomes ssh",
			input:        "git+ssh://git@github.com/org/docs",
			wantProtocol: "ssh",
		},
		{
			name:         "git+https becomes https",
			input:        "git+https://github.com/user/repo",
			wantProtocol: "https",
		},
		{
			name:         "git stays git",
			input:        "git://github.com/user/repo",
			wantProtocol: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := parseGitURL(tt.input)
			if err != nil {
				t.Fatalf("parseGitURL(%q) error = %v", tt.input, err)
			}

			if parsed.Endpoint.Protocol != tt.wantProtocol {
				t.Errorf("parseGitURL(%q).Endpoint.Protocol = %q, want %q",
					tt.input, parsed.Endpoint.Protocol, tt.wantProtocol)
			}
		})
	}
}
