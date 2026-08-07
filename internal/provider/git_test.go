package provider

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage"
)

// runCommand executes a command and returns combined output.
func runCommand(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func init() {
	// Register markdown MIME type for tests
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
}

func TestNewGitProvider(t *testing.T) {
	t.Parallel()

	// wantErrContains is on one row only: TestParseGitURL covers every rejection
	// message at its own layer, so what is left to prove here is that the wrap
	// keeps both halves — the sentinel callers classify on, and the reason.
	// Flattening to the sentinel made every malformed URL read "invalid git URL".
	tests := []struct {
		name            string
		gitURL          string
		defaultIndex    string
		dirIndex        bool
		wantErr         error
		wantErrContains string
	}{
		{
			name:         "valid git+https URL",
			gitURL:       "git+https://github.com/user/repo",
			defaultIndex: "README.md",
			dirIndex:     false,
		},
		{
			name:         "valid git+ssh URL",
			gitURL:       "git+ssh://git@github.com/org/docs#main",
			defaultIndex: "README.md",
			dirIndex:     true,
		},
		{
			name:         "valid git protocol URL",
			gitURL:       "git://gitlab.com/group/project",
			defaultIndex: "index.md",
			dirIndex:     false,
		},
		{
			name:            "invalid URL scheme",
			gitURL:          "https://github.com/user/repo",
			defaultIndex:    "README.md",
			dirIndex:        false,
			wantErr:         ErrInvalidGitURL,
			wantErrContains: "unsupported scheme",
		},
		{
			name:         "empty URL",
			gitURL:       "",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantErr:      ErrInvalidGitURL,
		},
		{
			name:         "empty defaultIndex",
			gitURL:       "git+https://github.com/user/repo",
			defaultIndex: "",
			dirIndex:     false,
			wantErr:      ErrEmptyDefaultIndex,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := NewGitProvider(tt.gitURL, tt.defaultIndex, tt.dirIndex, nil, GitProviderConfig{})

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewGitProvider() error = %v, want errors.Is %v", err, tt.wantErr)
				}
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("NewGitProvider() error = %q, want it to name the reason %q", err, tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewGitProvider() error = %v, want nil", err)
			}

			if p == nil {
				t.Fatal("NewGitProvider() returned nil, want non-nil")
			}

			if p.DefaultIndex() != tt.defaultIndex {
				t.Errorf("DefaultIndex() = %q, want %q", p.DefaultIndex(), tt.defaultIndex)
			}

			// Test Close doesn't panic
			if err := p.Close(); err != nil {
				t.Errorf("Close() error = %v, want nil", err)
			}
		})
	}
}

func TestGitProviderOptions(t *testing.T) {
	t.Parallel()

	t.Run("CloneTimeout", func(t *testing.T) {
		t.Parallel()
		p, err := NewGitProvider(
			"git+https://github.com/user/repo",
			"README.md",
			false,
			nil,
			GitProviderConfig{CloneTimeout: 30 * time.Second},
		)
		if err != nil {
			t.Fatalf("NewGitProvider() error = %v", err)
		}
		if p.cloneTimeout != 30*time.Second {
			t.Errorf("cloneTimeout = %v, want %v", p.cloneTimeout, 30*time.Second)
		}
	})

	t.Run("MaxFileSize", func(t *testing.T) {
		t.Parallel()
		p, err := NewGitProvider(
			"git+https://github.com/user/repo",
			"README.md",
			false,
			nil,
			GitProviderConfig{MaxFileSize: 100 * 1024 * 1024},
		)
		if err != nil {
			t.Fatalf("NewGitProvider() error = %v", err)
		}
		if p.maxFileSize != 100*1024*1024 {
			t.Errorf("maxFileSize = %v, want %v", p.maxFileSize, int64(100*1024*1024))
		}
	})

	t.Run("StorageFactory", func(t *testing.T) {
		t.Parallel()
		p, err := NewGitProvider(
			"git+https://github.com/user/repo",
			"README.md",
			false,
			nil,
			GitProviderConfig{StorageFactory: MemoryStorageFactory()},
		)
		if err != nil {
			t.Fatalf("NewGitProvider() error = %v", err)
		}
		if p == nil {
			t.Fatal("NewGitProvider() returned nil")
		}
	})
}

func TestGitProvider_ReadFileMocked(t *testing.T) {
	t.Parallel()

	// Create test repository
	repo := createTestRepo(t, map[string]string{
		"README.md":         "# Test\n\nHello",
		"docs/guide.md":     "# Guide",
		"docs/api/types.md": "# Types",
		"data.json":         `{"key": "value"}`,
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create provider with pre-populated state (bypassing lazy clone)
	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		dirIndex:     true,
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
		commitTime:   commitTime,
	}

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantMIME    string
		wantErr     error
	}{
		{
			name:        "read markdown file",
			path:        "/README.md",
			wantContent: "# Test\n\nHello",
			wantMIME:    "text/markdown",
		},
		{
			name:        "read nested file",
			path:        "/docs/guide.md",
			wantContent: "# Guide",
			wantMIME:    "text/markdown",
		},
		{
			name:        "read JSON file",
			path:        "/data.json",
			wantContent: `{"key": "value"}`,
			wantMIME:    "application/json",
		},
		{
			name:        "root returns README",
			path:        "/",
			wantContent: "# Test\n\nHello",
			wantMIME:    "text/markdown",
		},
		{
			name:    "not found",
			path:    "/nonexistent.md",
			wantErr: ErrNotFound,
		},
		{
			// Defense-in-depth: a relative parent-traversal path that survives
			// path.Clean must be rejected by the fs.ValidPath guard (REVIEW §9.9).
			name:    "parent traversal rejected",
			path:    "../secret.md",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, mimeType, err := p.ReadFile(t.Context(), tt.path)

			if tt.wantErr != nil {
				if err == nil {
					t.Error("ReadFile() error = nil, want error")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ReadFile() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadFile() error = %v, want nil", err)
			}

			if string(content) != tt.wantContent {
				t.Errorf("ReadFile() content = %q, want %q", string(content), tt.wantContent)
			}

			if tt.wantMIME != "" && !strings.Contains(mimeType, tt.wantMIME) {
				t.Errorf("ReadFile() mimeType = %q, want to contain %q", mimeType, tt.wantMIME)
			}
		})
	}
}

func TestGitProvider_StatMocked(t *testing.T) {
	t.Parallel()

	// Create test repository
	repo := createTestRepo(t, map[string]string{
		"README.md":     "# Test",
		"docs/guide.md": "# Guide",
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create provider with pre-populated state
	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		dirIndex:     true,
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
		commitTime:   commitTime,
	}

	tests := []struct {
		name      string
		path      string
		wantIsDir bool
		wantErr   error
	}{
		{
			name:      "file exists",
			path:      "/README.md",
			wantIsDir: false,
		},
		{
			name:      "nested file exists",
			path:      "/docs/guide.md",
			wantIsDir: false,
		},
		{
			name:      "directory exists",
			path:      "/docs",
			wantIsDir: true,
		},
		{
			name:      "root directory",
			path:      "/",
			wantIsDir: true,
		},
		{
			name:    "not found",
			path:    "/nonexistent.md",
			wantErr: ErrNotFound,
		},
		{
			// Defense-in-depth: parent-traversal path rejected by fs.ValidPath
			// guard (REVIEW §9.9).
			name:    "parent traversal rejected",
			path:    "../secret.md",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := p.Stat(t.Context(), tt.path)

			if tt.wantErr != nil {
				if err == nil {
					t.Error("Stat() error = nil, want error")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Stat() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Stat() error = %v, want nil", err)
			}

			if info == nil {
				t.Fatal("Stat() returned nil, want non-nil")
			}

			if info.IsDir() != tt.wantIsDir {
				t.Errorf("Stat().IsDir() = %v, want %v", info.IsDir(), tt.wantIsDir)
			}
		})
	}
}

func TestGitProvider_DirectoryListingMocked(t *testing.T) {
	t.Parallel()

	// Create test repository with directory structure
	repo := createTestRepo(t, map[string]string{
		"docs/a.md":    "# A",
		"docs/b.md":    "# B",
		"docs/.hidden": "hidden file",
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("directory listing enabled", func(t *testing.T) {
		p := &GitProvider{
			parsedURL: &ParsedGitURL{
				Ref:    "HEAD",
				Subdir: "",
			},
			defaultIndex: "README.md", // doesn't exist in docs
			dirIndex:     true,
			maxFileSize:  defaultMaxFileSize,
			repo:         repo,
			treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
			commitTime:   commitTime,
		}

		content, mimeType, err := p.ReadFile(t.Context(), "/docs")
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		if mimeType != "text/markdown; charset=utf-8" {
			t.Errorf("mimeType = %q, want %q", mimeType, "text/markdown; charset=utf-8")
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "a.md") {
			t.Error("listing should contain a.md")
		}
		if !strings.Contains(contentStr, "b.md") {
			t.Error("listing should contain b.md")
		}
		if strings.Contains(contentStr, ".hidden") {
			t.Error("listing should not contain .hidden")
		}
	})

	t.Run("directory listing disabled", func(t *testing.T) {
		p := &GitProvider{
			parsedURL: &ParsedGitURL{
				Ref:    "HEAD",
				Subdir: "",
			},
			defaultIndex: "README.md", // doesn't exist in docs
			dirIndex:     false,
			maxFileSize:  defaultMaxFileSize,
			repo:         repo,
			treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
			commitTime:   commitTime,
		}

		_, _, err := p.ReadFile(t.Context(), "/docs")
		if err == nil {
			t.Error("ReadFile() error = nil, want error")
		}
		if !errors.Is(err, ErrDirListingDisabled) {
			t.Errorf("error = %v, want ErrDirListingDisabled", err)
		}
	})
}

func TestGitProvider_LFSPointerDetection(t *testing.T) {
	t.Parallel()

	lfsContent := "version https://git-lfs.github.com/spec/v1\noid sha256:abc123\nsize 12345\n"

	// Create test repository with LFS pointer
	repo := createTestRepo(t, map[string]string{
		"large-file.bin": lfsContent,
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		dirIndex:     false,
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
		commitTime:   commitTime,
	}

	_, _, err := p.ReadFile(t.Context(), "/large-file.bin")
	if err == nil {
		t.Error("ReadFile() error = nil, want error for LFS pointer")
	}
	if !errors.Is(err, ErrGitLFSNotSupported) {
		t.Errorf("error = %v, want ErrGitLFSNotSupported", err)
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"/", "."},
		{"", "."},
		{"/path/to/file", "path/to/file"},
		{"path/to/file", "path/to/file"},
		{"/./path/../to/file", "to/file"},
		{"//double//slashes//", "double/slashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizePath(tt.input)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsLFSPointer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "LFS pointer",
			content: "version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 123\n",
			want:    true,
		},
		{
			name:    "regular markdown",
			content: "# Hello World\n\nThis is content.",
			want:    false,
		},
		{
			name:    "empty string",
			content: "",
			want:    false,
		},
		{
			name:    "similar but not LFS",
			content: "version: 1.0.0\nsome other content",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isLFSPointer([]byte(tt.content))
			if got != tt.want {
				t.Errorf("isLFSPointer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyCloneError(t *testing.T) {
	t.Parallel()

	// Use a real parsed URL for testing
	parsed, _ := parseGitURL("git+https://github.com/user/repo")
	p := &GitProvider{
		parsedURL: parsed,
	}

	tests := []struct {
		name     string
		inputErr error
		wantErr  error
	}{
		{
			name:     "authentication required sentinel",
			inputErr: transport.ErrAuthenticationRequired,
			wantErr:  ErrGitAuthFailed,
		},
		{
			name:     "authentication required wrapped",
			inputErr: fmt.Errorf("clone failed: %w", transport.ErrAuthenticationRequired),
			wantErr:  ErrGitAuthFailed,
		},
		{
			name:     "authorization failed sentinel",
			inputErr: transport.ErrAuthorizationFailed,
			wantErr:  ErrGitAuthFailed,
		},
		{
			name:     "username prompt string fallback",
			inputErr: errors.New("could not read Username"),
			wantErr:  ErrGitAuthFailed,
		},
		{
			name:     "connection refused string fallback",
			inputErr: errors.New("connection refused"),
			wantErr:  ErrGitConnectFailed,
		},
		{
			name:     "repository not found sentinel",
			inputErr: transport.ErrRepositoryNotFound,
			wantErr:  ErrNotFound,
		},
		{
			name:     "repository not found wrapped",
			inputErr: fmt.Errorf("fetch: %w", transport.ErrRepositoryNotFound),
			wantErr:  ErrNotFound,
		},
		{
			name:     "reference not found sentinel",
			inputErr: plumbing.ErrReferenceNotFound,
			wantErr:  ErrGitRefNotFound,
		},
		{
			name:     "reference not found wrapped",
			inputErr: fmt.Errorf("resolve: %w", plumbing.ErrReferenceNotFound),
			wantErr:  ErrGitRefNotFound,
		},
		{
			name:     "unknown error passes through",
			inputErr: errors.New("some other error"),
			wantErr:  nil, // check original error is preserved
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resultErr := p.classifyCloneError(tt.inputErr)

			if tt.wantErr != nil {
				if !errors.Is(resultErr, tt.wantErr) {
					t.Errorf("classifyCloneError() = %v, want %v", resultErr, tt.wantErr)
				}
			} else {
				// For unknown errors, the original error should be preserved
				if !errors.Is(resultErr, tt.inputErr) {
					t.Errorf("classifyCloneError() should preserve original error, got %v", resultErr)
				}
			}
		})
	}
}

func TestGitProvider_Close(t *testing.T) {
	t.Parallel()

	p, err := NewGitProvider("git+https://github.com/user/repo", "README.md", false, nil, GitProviderConfig{})
	if err != nil {
		t.Fatalf("NewGitProvider() error = %v", err)
	}

	// Close should work
	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Close should be idempotent
	if err := p.Close(); err != nil {
		t.Errorf("Close() second call error = %v, want nil", err)
	}
}

// Ensure GitProvider implements Provider interface
var _ Provider = (*GitProvider)(nil)

func TestGitProvider_ImplementsProvider(t *testing.T) {
	t.Parallel()

	// Compile-time check
	var _ Provider = (*GitProvider)(nil)
}

func TestResolveAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		protocol string
		wantNil  bool
		wantErr  bool
	}{
		{"git protocol is anonymous", "git", true, false},
		{"https protocol is anonymous", "https", true, false},
		{"http protocol is anonymous", "http", true, false},
		{"ssh protocol requires key file", "ssh", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := parseGitURL("git+https://github.com/user/repo")
			if err != nil {
				t.Fatalf("parseGitURL error: %v", err)
			}

			// Override protocol for testing
			parsed.Endpoint.Protocol = tt.protocol

			auth, err := resolveAuth(parsed.Endpoint, "")
			if tt.wantErr {
				if err == nil {
					t.Error("resolveAuth() error = nil, want error")
				}
				return
			}

			if tt.wantNil {
				if err != nil {
					t.Errorf("resolveAuth() error = %v, want nil", err)
				}
				if auth != nil {
					t.Errorf("resolveAuth() = %v, want nil", auth)
				}
			}
		})
	}
}

func TestMemoryStorageFactory(t *testing.T) {
	t.Parallel()

	factory := MemoryStorageFactory()
	store, err := factory()
	if err != nil {
		t.Errorf("MemoryStorageFactory() error = %v", err)
	}
	if store == nil {
		t.Error("MemoryStorageFactory() returned nil storage")
	}
}

func TestDiskStorageFactory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dir     func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "creates directory and returns valid storage",
			dir: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "git-cache")
			},
			wantErr: false,
		},
		{
			name: "works with existing directory",
			dir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name: "works with nested path",
			dir: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "a", "b", "c")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := tt.dir(t)
			factory := DiskStorageFactory(dir)
			stor, err := factory()

			if tt.wantErr {
				if err == nil {
					t.Error("DiskStorageFactory() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("DiskStorageFactory() error = %v, want nil", err)
			}
			if stor == nil {
				t.Fatal("DiskStorageFactory() returned nil storage")
			}

			// Verify directory was created
			info, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("directory not created: %v", err)
			}
			if !info.IsDir() {
				t.Error("path is not a directory")
			}
		})
	}
}

func TestDiskStorageFactory_MultipleCalls(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "git-cache")
	factory := DiskStorageFactory(dir)

	// Call factory multiple times — should succeed each time
	for i := range 3 {
		stor, err := factory()
		if err != nil {
			t.Fatalf("call %d: DiskStorageFactory() error = %v", i, err)
		}
		if stor == nil {
			t.Fatalf("call %d: DiskStorageFactory() returned nil storage", i)
		}
	}
}

func TestGitProvider_FileSizeLimit(t *testing.T) {
	t.Parallel()

	// Create test repository with a file
	repo := createTestRepo(t, map[string]string{
		"file.txt": "small content",
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Set a very small file size limit
	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		dirIndex:     false,
		maxFileSize:  1, // 1 byte limit
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
		commitTime:   commitTime,
	}

	_, _, err := p.ReadFile(t.Context(), "/file.txt")
	if err == nil {
		t.Error("ReadFile() error = nil, want error for file too large")
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("error = %v, want ErrFileTooLarge", err)
	}
}

func TestGitProvider_SubdirMocked(t *testing.T) {
	t.Parallel()

	// Create test repository with subdirectory structure
	repo := createTestRepo(t, map[string]string{
		"README.md":         "# Root README",
		"docs/README.md":    "# Docs README",
		"docs/guide.md":     "# Guide",
		"docs/api/types.md": "# Types",
	})

	head, _ := repo.Head()
	commit, _ := repo.CommitObject(head.Hash())
	rootTree, _ := commit.Tree()

	// Get the docs subdirectory tree
	docsTree, err := rootTree.Tree("docs")
	if err != nil {
		t.Fatalf("Failed to get docs tree: %v", err)
	}

	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create provider with subdir
	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "docs",
		},
		defaultIndex: "README.md",
		dirIndex:     true,
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: docsTree, objects: repo.Storer, modTime: commitTime}, // Use the subdirectory tree
		commitTime:   commitTime,
		commitHash:   head.Hash(),
	}

	t.Run("root returns docs README", func(t *testing.T) {
		content, _, err := p.ReadFile(t.Context(), "/")
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(content) != "# Docs README" {
			t.Errorf("content = %q, want %q", string(content), "# Docs README")
		}
	})

	t.Run("can access files in subdir", func(t *testing.T) {
		content, _, err := p.ReadFile(t.Context(), "/guide.md")
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(content) != "# Guide" {
			t.Errorf("content = %q, want %q", string(content), "# Guide")
		}
	})

	t.Run("can access nested subdirectory", func(t *testing.T) {
		content, _, err := p.ReadFile(t.Context(), "/api/types.md")
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(content) != "# Types" {
			t.Errorf("content = %q, want %q", string(content), "# Types")
		}
	})
}

func init() {
	// Suppress log output during tests
	// (moved from individual test functions for cleaner output)
}

// Ensure we import plumbing for the test
var _ = plumbing.HEAD

// Ensure we import fs for interface checks
var _ fs.FileInfo = (*gitFileInfo)(nil)

func TestGitProviderOptions_SSHKeyFile(t *testing.T) {
	t.Parallel()

	p, err := NewGitProvider(
		"git+https://github.com/user/repo",
		"README.md",
		false,
		nil,
		GitProviderConfig{SSHKeyFile: "/path/to/key"},
	)
	if err != nil {
		t.Fatalf("NewGitProvider() error = %v", err)
	}
	if p.sshKeyFile != "/path/to/key" {
		t.Errorf("sshKeyFile = %q, want %q", p.sshKeyFile, "/path/to/key")
	}
}

func TestGitProvider_RootFS_Mocked(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"README.md": "# Hello",
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		commitTime:   commitTime,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
	}

	rootFS, err := p.RootFS(t.Context())
	if err != nil {
		t.Fatalf("RootFS() error = %v", err)
	}
	if rootFS == nil {
		t.Fatal("RootFS() returned nil")
	}

	// Verify we can read files through the returned fs.FS
	data, err := fs.ReadFile(rootFS, "README.md")
	if err != nil {
		t.Fatalf("ReadFile via RootFS error = %v", err)
	}
	if string(data) != "# Hello" {
		t.Errorf("content = %q, want %q", string(data), "# Hello")
	}
}

func TestGitProvider_EnsureCloned_AlreadyClosed(t *testing.T) {
	t.Parallel()

	p, err := NewGitProvider("git+https://github.com/user/repo", "README.md", false, nil, GitProviderConfig{})
	if err != nil {
		t.Fatalf("NewGitProvider() error = %v", err)
	}
	_ = p.Close()

	_, _, err = p.ReadFile(t.Context(), "/README.md")
	if err == nil {
		t.Error("ReadFile() on closed provider should return error")
	}

	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("error should be *PathError, got %T: %v", err, err)
	}
}

func TestGitProvider_EnsureCloned_AlreadyCloned(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"README.md": "# Test",
	})

	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
		commitTime:   commitTime,
	}

	// ensureCloned should return immediately (fast path)
	err := p.ensureCloned(t.Context())
	if err != nil {
		t.Errorf("ensureCloned() on already-cloned provider = %v, want nil", err)
	}
}

func TestGitProvider_EnsureCloned_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"README.md": "# Test",
	})
	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	p := &GitProvider{
		parsedURL: &ParsedGitURL{
			Ref:    "HEAD",
			Subdir: "",
		},
		defaultIndex: "README.md",
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
		commitTime:   commitTime,
	}

	// Concurrent ensureCloned calls on already-cloned provider should all succeed
	var wg sync.WaitGroup
	errs := make([]error, 50)
	for i := range 50 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = p.ensureCloned(t.Context())
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: ensureCloned() = %v, want nil", i, err)
		}
	}
}

// TestGitProvider_ConcurrentReadsAreRaceFree hammers every tree-reading entry
// point at once. Serving git reads under a read lock is a concurrent map
// write — see gitTreeState for why go-git lookups are writes.
//
// TestGitProvider_EnsureCloned_ConcurrentAccess does not cover this: every
// goroutine there returns at ensureCloned's fast path without touching a tree.
func TestGitProvider_ConcurrentReadsAreRaceFree(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"README.md":          "# Root",
		"docs/guide.md":      "# Guide",
		"docs/api/types.md":  "# Types",
		"docs/api/errors.md": "# Errors",
	})
	tree := mustGetTree(t, repo)
	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	p := &GitProvider{
		parsedURL:    &ParsedGitURL{Ref: "HEAD"},
		defaultIndex: "README.md",
		dirIndex:     true,
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		commitTime:   commitTime,
		treeState:    gitTreeState{tree: tree, objects: repo.Storer, modTime: commitTime},
	}

	rootFS, err := p.RootFS(t.Context())
	if err != nil {
		t.Fatalf("RootFS() error = %v", err)
	}

	// Nested paths are required: FindEntry only populates its subtree cache
	// while descending, so a flat repo would not reproduce the race. The
	// directory entry exercises the tree-lookup path alongside the blob one.
	paths := []string{"/docs/api/types.md", "/docs/api/errors.md", "/docs/guide.md", "/docs", "/README.md"}

	const goroutines = 40
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := range goroutines {
		wg.Go(func() {
			reqPath := paths[i%len(paths)]
			if _, _, err := p.ReadFile(t.Context(), reqPath); err != nil {
				errs[i] = fmt.Errorf("ReadFile(%s): %w", reqPath, err)
				return
			}
			if _, err := p.Stat(t.Context(), reqPath); err != nil {
				errs[i] = fmt.Errorf("Stat(%s): %w", reqPath, err)
				return
			}
			// RootFS shares the same tree, so it must share the same lock.
			if _, err := rootFS.Open(normalizePath(reqPath)); err != nil {
				errs[i] = fmt.Errorf("Open(%s): %w", reqPath, err)
			}
		})
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// TestGitProvider_NilTreeIsAnErrorNotAPanic covers the state a failed ref
// resolution used to leave behind: a repo published as "cloned" with no cached
// tree, which made every later request dereference nil. cloneLocked now rolls
// back, but Close can still land between ensureCloned and the tree lock, so
// the guard has to stay.
func TestGitProvider_NilTreeIsAnErrorNotAPanic(t *testing.T) {
	t.Parallel()

	p := &GitProvider{
		parsedURL:    &ParsedGitURL{Ref: "HEAD"},
		defaultIndex: "README.md",
		maxFileSize:  defaultMaxFileSize,
		repo:         createTestRepo(t, map[string]string{"README.md": "# Root"}),
	}

	if _, _, err := p.ReadFile(t.Context(), "/README.md"); !errors.Is(err, ErrProviderClosed) {
		t.Errorf("ReadFile() error = %v, want %v", err, ErrProviderClosed)
	}
	if _, err := p.Stat(t.Context(), "/README.md"); !errors.Is(err, ErrProviderClosed) {
		t.Errorf("Stat() error = %v, want %v", err, ErrProviderClosed)
	}
}

func TestGitProvider_EnsureCloned_DoubleCheckAfterClose(t *testing.T) {
	t.Parallel()

	// Tests the slow path: fast path sees repo == nil, acquires write lock,
	// then double-check sees closed == true.
	p, err := NewGitProvider("git+https://github.com/user/repo", "README.md", false, nil, GitProviderConfig{})
	if err != nil {
		t.Fatalf("NewGitProvider() error = %v", err)
	}

	// Close sets closed = true
	_ = p.Close()

	// ensureCloned should fail on the double-check
	err = p.ensureCloned(t.Context())
	if !errors.Is(err, ErrProviderClosed) {
		t.Errorf("ensureCloned() on closed provider = %v, want ErrProviderClosed", err)
	}
}

func TestGitProvider_ResolveCommitLocked(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{
		"README.md": "# Hello",
	})

	head, _ := repo.Head()

	t.Run("resolves HEAD", func(t *testing.T) {
		t.Parallel()

		p := &GitProvider{
			parsedURL: &ParsedGitURL{Ref: "HEAD"},
			repo:      repo,
		}

		err := p.resolveCommitLocked()
		if err != nil {
			t.Fatalf("resolveCommitLocked() error = %v", err)
		}
		if p.commitHash != head.Hash() {
			t.Errorf("commitHash = %v, want %v", p.commitHash, head.Hash())
		}
		if p.commitTime.IsZero() {
			t.Error("commitTime should not be zero")
		}
	})

	t.Run("resolves by commit hash", func(t *testing.T) {
		t.Parallel()

		p := &GitProvider{
			parsedURL: &ParsedGitURL{Ref: head.Hash().String()},
			repo:      repo,
		}

		err := p.resolveCommitLocked()
		if err != nil {
			t.Fatalf("resolveCommitLocked() error = %v", err)
		}
		if p.commitHash != head.Hash() {
			t.Errorf("commitHash = %v, want %v", p.commitHash, head.Hash())
		}
	})

	t.Run("returns error for nonexistent ref", func(t *testing.T) {
		t.Parallel()

		p := &GitProvider{
			parsedURL: &ParsedGitURL{Ref: "nonexistent-branch"},
			repo:      repo,
		}

		err := p.resolveCommitLocked()
		if !errors.Is(err, ErrGitRefNotFound) {
			t.Errorf("resolveCommitLocked() error = %v, want ErrGitRefNotFound", err)
		}
	})
}

func TestGitProvider_CacheTreeLocked(t *testing.T) {
	t.Parallel()

	t.Run("caches root tree", func(t *testing.T) {
		t.Parallel()

		repo := createTestRepo(t, map[string]string{
			"README.md":     "# Hello",
			"docs/guide.md": "# Guide",
		})
		head, _ := repo.Head()

		p := &GitProvider{
			parsedURL:  &ParsedGitURL{Ref: "HEAD", Subdir: ""},
			repo:       repo,
			commitHash: head.Hash(),
			commitTime: time.Now(),
		}

		err := p.cacheTreeLocked()
		if err != nil {
			t.Fatalf("cacheTreeLocked() error = %v", err)
		}
		if p.treeState.tree == nil {
			t.Fatal("treeState.tree should not be nil after cacheTreeLocked")
		}
		if p.treeState.modTime != p.commitTime {
			t.Errorf("treeState.modTime = %v, want %v", p.treeState.modTime, p.commitTime)
		}

		// Verify we can read files through the cached tree
		_, fileErr := p.treeState.tree.File("README.md")
		if fileErr != nil {
			t.Errorf("tree.File(\"README.md\") error = %v", fileErr)
		}
	})

	t.Run("caches subdir tree", func(t *testing.T) {
		t.Parallel()

		repo := createTestRepo(t, map[string]string{
			"README.md":     "# Root",
			"docs/guide.md": "# Guide",
		})
		head, _ := repo.Head()

		p := &GitProvider{
			parsedURL:  &ParsedGitURL{Ref: "HEAD", Subdir: "docs"},
			repo:       repo,
			commitHash: head.Hash(),
			commitTime: time.Now(),
		}

		err := p.cacheTreeLocked()
		if err != nil {
			t.Fatalf("cacheTreeLocked() error = %v", err)
		}

		// Tree should be the docs subtree — guide.md is at root
		_, fileErr := p.treeState.tree.File("guide.md")
		if fileErr != nil {
			t.Errorf("tree.File(\"guide.md\") error = %v", fileErr)
		}

		// Root README should NOT be accessible
		_, fileErr = p.treeState.tree.File("README.md")
		if fileErr == nil {
			t.Error("tree.File(\"README.md\") should fail for subdir tree")
		}
	})

	t.Run("returns error for nonexistent subdir", func(t *testing.T) {
		t.Parallel()

		repo := createTestRepo(t, map[string]string{
			"README.md": "# Hello",
		})
		head, _ := repo.Head()

		p := &GitProvider{
			parsedURL:  &ParsedGitURL{Ref: "HEAD", Subdir: "nonexistent"},
			repo:       repo,
			commitHash: head.Hash(),
			commitTime: time.Now(),
		}

		err := p.cacheTreeLocked()
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("cacheTreeLocked() error = %v, want ErrNotFound", err)
		}
	})

	// The other half of the mapping: the subdir exists in the tree, but the
	// object behind it cannot be decoded. Flattening this to ErrNotFound made
	// a corrupt repository report as a typo in the URL fragment — and, since
	// ErrNotFound is a 404, made it look like a client error.
	t.Run("keeps a storer fault distinct from a missing subdir", func(t *testing.T) {
		t.Parallel()

		repo := createTestRepo(t, map[string]string{
			"README.md":     "# Root",
			"docs/guide.md": "# Guide",
		})
		head, _ := repo.Head()

		docs, err := mustGetTree(t, repo).FindEntry("docs")
		if err != nil {
			t.Fatalf("FindEntry(\"docs\") error = %v", err)
		}

		// Bare: cacheTreeLocked never touches a worktree.
		faulty, err := git.Open(&faultyStorer{Storer: repo.Storer, fail: docs.Hash}, nil)
		if err != nil {
			t.Fatalf("git.Open() error = %v", err)
		}

		p := &GitProvider{
			parsedURL:  &ParsedGitURL{Ref: "HEAD", Subdir: "docs"},
			repo:       faulty,
			commitHash: head.Hash(),
			commitTime: time.Now(),
		}

		err = p.cacheTreeLocked()
		if !errors.Is(err, errStorerFault) {
			t.Errorf("cacheTreeLocked() error = %v, want the storer's own error", err)
		}
		if errors.Is(err, ErrNotFound) {
			t.Errorf("cacheTreeLocked() error = %v, want it NOT classified as ErrNotFound", err)
		}
	})
}

func TestGitProvider_SetupAuthLocked(t *testing.T) {
	t.Parallel()

	t.Run("HTTPS requires no auth", func(t *testing.T) {
		t.Parallel()

		parsed, _ := parseGitURL("git+https://github.com/user/repo")
		p := &GitProvider{parsedURL: parsed}

		err := p.setupAuthLocked()
		if err != nil {
			t.Fatalf("setupAuthLocked() error = %v", err)
		}
		if p.auth != nil {
			t.Error("auth should be nil for HTTPS")
		}
	})

	t.Run("SSH without key file returns error", func(t *testing.T) {
		t.Parallel()

		parsed, _ := parseGitURL("git+ssh://git@github.com/user/repo")
		p := &GitProvider{parsedURL: parsed, sshKeyFile: ""}

		err := p.setupAuthLocked()
		if err == nil {
			t.Fatal("setupAuthLocked() error = nil, want error for SSH without key")
		}
	})
}

func TestGitProvider_CloneLocked_StorageFactoryError(t *testing.T) {
	t.Parallel()

	parsed, _ := parseGitURL("git+https://github.com/user/repo")
	p := &GitProvider{
		parsedURL: parsed,
		storageFactory: func() (storage.Storer, error) {
			return nil, fmt.Errorf("storage creation failed")
		},
		cloneTimeout: defaultCloneTimeout,
	}

	err := p.cloneLocked(t.Context())
	if err == nil {
		t.Fatal("cloneLocked() error = nil, want error")
	}

	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error type = %T, want *PathError", err)
	}
	if pathErr.Op != "storage" {
		t.Errorf("PathError.Op = %q, want %q", pathErr.Op, "storage")
	}
}

func TestFilesystemProvider_Stat_Directory(t *testing.T) {
	t.Parallel()

	mapFS := fstest.MapFS{
		"docs/file.md": &fstest.MapFile{Data: []byte("# Doc")},
	}

	p, err := NewFilesystemProviderFromFS(mapFS, "README.md", false, nil)
	if err != nil {
		t.Fatalf("NewFilesystemProviderFromFS() error = %v", err)
	}

	info, err := p.Stat(t.Context(), "docs")
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Error("Stat(\"docs\") should report IsDir=true")
	}
}

func TestFilesystemProvider_DefaultIndex(t *testing.T) {
	t.Parallel()

	mapFS := fstest.MapFS{
		"readme.txt": &fstest.MapFile{Data: []byte("hello")},
	}

	p, err := NewFilesystemProviderFromFS(mapFS, "readme.txt", false, nil)
	if err != nil {
		t.Fatalf("NewFilesystemProviderFromFS() error = %v", err)
	}

	if p.DefaultIndex() != "readme.txt" {
		t.Errorf("DefaultIndex() = %q, want %q", p.DefaultIndex(), "readme.txt")
	}
}

func TestFilesystemProvider_Close(t *testing.T) {
	t.Parallel()

	mapFS := fstest.MapFS{}
	p, err := NewFilesystemProviderFromFS(mapFS, "README.md", false, nil)
	if err != nil {
		t.Fatalf("NewFilesystemProviderFromFS() error = %v", err)
	}

	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestSetupSSHAuth_NoKeyFile(t *testing.T) {
	t.Parallel()

	_, err := setupSSHAuth("git", "")
	if err == nil {
		t.Fatal("setupSSHAuth() error = nil, want error for empty key file")
	}
	if !strings.Contains(err.Error(), "key file") {
		t.Errorf("error should mention key file, got: %v", err)
	}
}

func TestSetupSSHAuth_DefaultUser(t *testing.T) {
	t.Parallel()

	// With empty user and no key file, should still fail on missing key file
	// but the user default ("git") is set internally
	_, err := setupSSHAuth("", "")
	if err == nil {
		t.Fatal("setupSSHAuth() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "key file") {
		t.Errorf("error should mention key file, got: %v", err)
	}
}

func TestSetupSSHAuth_WithValidKey(t *testing.T) {
	// Generate a temporary SSH key for testing
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "id_ed25519")

	// Create a valid ed25519 private key in PEM format using ssh-keygen
	cmd := fmt.Sprintf("ssh-keygen -t ed25519 -f %s -N '' -q", keyFile)
	if err := os.WriteFile(keyFile, nil, 0600); err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	// Remove the placeholder and generate a real key
	os.Remove(keyFile)
	if output, err := runCommand(t, "sh", "-c", cmd); err != nil {
		t.Skipf("ssh-keygen not available: %v (output: %s)", err, output)
	}

	// Create a known_hosts file
	knownHostsFile := filepath.Join(tmpDir, "known_hosts")
	if err := os.WriteFile(knownHostsFile, []byte("github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n"), 0600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	t.Setenv("SSH_KNOWN_HOSTS", knownHostsFile)

	auth, err := setupSSHAuth("git", keyFile)
	if err != nil {
		t.Fatalf("setupSSHAuth() error = %v", err)
	}
	if auth == nil {
		t.Fatal("setupSSHAuth() returned nil auth")
	}
}

func TestSetupSSHAuth_NonexistentKeyFile(t *testing.T) {
	// Create a known_hosts file so we get past the host key callback
	tmpDir := t.TempDir()
	knownHostsFile := filepath.Join(tmpDir, "known_hosts")
	if err := os.WriteFile(knownHostsFile, []byte("github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n"), 0600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}
	t.Setenv("SSH_KNOWN_HOSTS", knownHostsFile)

	// setupSSHAuth should succeed (key is read lazily via callback),
	// but the auth object should be non-nil
	auth, err := setupSSHAuth("git", "/nonexistent/key")
	if err != nil {
		t.Fatalf("setupSSHAuth() error = %v (key read is lazy)", err)
	}
	if auth == nil {
		t.Fatal("setupSSHAuth() returned nil auth")
	}
}

func TestResolveAuth_UnknownProtocol(t *testing.T) {
	t.Parallel()

	parsed, err := parseGitURL("git+https://github.com/user/repo")
	if err != nil {
		t.Fatalf("parseGitURL error: %v", err)
	}
	parsed.Endpoint.Protocol = "ftp"

	auth, err := resolveAuth(parsed.Endpoint, "")
	if err != nil {
		t.Errorf("resolveAuth() error = %v, want nil", err)
	}
	if auth != nil {
		t.Errorf("resolveAuth() = %v, want nil for unknown protocol", auth)
	}
}

func TestCreateHostKeyCallback_WithKnownHosts(t *testing.T) {
	// Create a temporary known_hosts file
	knownHostsFile := filepath.Join(t.TempDir(), "known_hosts")
	// Write a valid (but fake) known_hosts entry
	err := os.WriteFile(knownHostsFile, []byte("github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n"), 0600)
	if err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	// Set SSH_KNOWN_HOSTS to our temp file
	t.Setenv("SSH_KNOWN_HOSTS", knownHostsFile)

	callback, err := createHostKeyCallback()
	if err != nil {
		t.Fatalf("createHostKeyCallback() error = %v", err)
	}
	if callback == nil {
		t.Fatal("createHostKeyCallback() returned nil callback")
	}
}

func TestCreateHostKeyCallback_InvalidPath(t *testing.T) {
	t.Setenv("SSH_KNOWN_HOSTS", "/nonexistent/path/known_hosts")

	_, err := createHostKeyCallback()
	if err == nil {
		t.Fatal("createHostKeyCallback() error = nil, want error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "known_hosts") {
		t.Errorf("error should mention known_hosts, got: %v", err)
	}
}

func TestNewProvider_GitURL(t *testing.T) {
	t.Parallel()

	p, err := NewProvider("git+https://github.com/user/repo", "README.md", false, nil)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	defer p.Close()

	// Should be a GitProvider
	if _, ok := p.(*GitProvider); !ok {
		t.Errorf("NewProvider() returned %T, want *GitProvider", p)
	}
}

func TestNewProvider_FilesystemPath(t *testing.T) {
	t.Parallel()

	p, err := NewProvider(".", "README.md", false, nil)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	defer p.Close()

	// Should be a FilesystemProvider
	if _, ok := p.(*FilesystemProvider); !ok {
		t.Errorf("NewProvider() returned %T, want *FilesystemProvider", p)
	}
}

func TestNewProvider_WithGitConfig(t *testing.T) {
	t.Parallel()

	cfg := GitProviderConfig{CloneTimeout: 30 * time.Second}
	p, err := NewProvider("git+https://github.com/user/repo", "README.md", false, nil, cfg)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	defer p.Close()

	gp, ok := p.(*GitProvider)
	if !ok {
		t.Fatalf("NewProvider() returned %T, want *GitProvider", p)
	}
	if gp.cloneTimeout != 30*time.Second {
		t.Errorf("cloneTimeout = %v, want 30s", gp.cloneTimeout)
	}
}

// TestGitProvider_StorerFaultIsNotNotFound covers the request-path half of the
// same mapping. ReadFile and Stat flattened every resolveTreeNode/statTreeNode
// failure to ErrNotFound, so a corrupt packfile served a themed 404 on every
// page instead of a 500 — the operator's only signal that the repository, not
// the URL, is the problem.
func TestGitProvider_StorerFaultIsNotNotFound(t *testing.T) {
	t.Parallel()

	repo := createTestRepo(t, map[string]string{"docs/guide.md": "# Guide"})

	guide, err := mustGetTree(t, repo).FindEntry("docs/guide.md")
	if err != nil {
		t.Fatalf("FindEntry() error = %v", err)
	}

	// gitProviderOver, not a literal: the tree must be re-derived over the
	// faulty storer, or TreeEntryFile resolves the blob through the repo's own
	// and never sees the fault.
	p := gitProviderOver(t, repo, &faultyStorer{Storer: repo.Storer, fail: guide.Hash})

	t.Run("ReadFile", func(t *testing.T) {
		_, _, err := p.ReadFile(t.Context(), "/docs/guide.md")
		assertStorerFault(t, err)
	})

	t.Run("Stat", func(t *testing.T) {
		_, err := p.Stat(t.Context(), "/docs/guide.md")
		assertStorerFault(t, err)
	})

	t.Run("a genuinely missing path is still ErrNotFound", func(t *testing.T) {
		if _, _, err := p.ReadFile(t.Context(), "/nope.md"); !errors.Is(err, ErrNotFound) {
			t.Errorf("ReadFile() error = %v, want ErrNotFound", err)
		}
		if _, err := p.Stat(t.Context(), "/nope.md"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Stat() error = %v, want ErrNotFound", err)
		}
	})
}

func assertStorerFault(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errStorerFault) {
		t.Errorf("error = %v, want the storer's own error", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want it NOT classified as ErrNotFound", err)
	}
}
