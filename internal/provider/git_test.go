package provider

import (
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

func init() {
	// Register markdown MIME type for tests
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
}

// createTestRepo creates an in-memory Git repository with the specified files.
// Files are specified as a map of path -> content.
func createTestRepo(t *testing.T, files map[string]string) *git.Repository {
	t.Helper()

	stor := memory.NewStorage()
	fs := memfs.New()

	repo, err := git.Init(stor, fs)
	if err != nil {
		t.Fatalf("Failed to init test repo: %v", err)
	}

	// Create worktree
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Add files
	for path, content := range files {
		// Create file in worktree filesystem
		f, err := wt.Filesystem.Create(path)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
		_, err = f.Write([]byte(content))
		if err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
		f.Close()

		// Add to staging
		_, err = wt.Add(path)
		if err != nil {
			t.Fatalf("Failed to add file %s: %v", path, err)
		}
	}

	// Create commit
	_, err = wt.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	return repo
}

// mustGetTree gets the tree from a repository's HEAD commit.
func mustGetTree(t *testing.T, repo *git.Repository) *object.Tree {
	t.Helper()

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("Failed to get commit: %v", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	return tree
}

func TestNewGitProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		gitURL       string
		defaultIndex string
		dirIndex     bool
		wantErr      bool
	}{
		{
			name:         "valid git+https URL",
			gitURL:       "git+https://github.com/user/repo",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantErr:      false,
		},
		{
			name:         "valid git+ssh URL",
			gitURL:       "git+ssh://git@github.com/org/docs#main",
			defaultIndex: "README.md",
			dirIndex:     true,
			wantErr:      false,
		},
		{
			name:         "valid git protocol URL",
			gitURL:       "git://gitlab.com/group/project",
			defaultIndex: "index.md",
			dirIndex:     false,
			wantErr:      false,
		},
		{
			name:         "invalid URL scheme",
			gitURL:       "https://github.com/user/repo",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantErr:      true,
		},
		{
			name:         "empty URL",
			gitURL:       "",
			defaultIndex: "README.md",
			dirIndex:     false,
			wantErr:      true,
		},
		{
			name:         "empty defaultIndex",
			gitURL:       "git+https://github.com/user/repo",
			defaultIndex: "",
			dirIndex:     false,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := NewGitProvider(tt.gitURL, tt.defaultIndex, tt.dirIndex)

			if tt.wantErr {
				if err == nil {
					t.Error("NewGitProvider() error = nil, want error")
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

	t.Run("WithCloneTimeout", func(t *testing.T) {
		t.Parallel()
		p, err := NewGitProvider(
			"git+https://github.com/user/repo",
			"README.md",
			false,
			WithCloneTimeout(30*time.Second),
		)
		if err != nil {
			t.Fatalf("NewGitProvider() error = %v", err)
		}
		if p.cloneTimeout != 30*time.Second {
			t.Errorf("cloneTimeout = %v, want %v", p.cloneTimeout, 30*time.Second)
		}
	})

	t.Run("WithMaxFileSize", func(t *testing.T) {
		t.Parallel()
		p, err := NewGitProvider(
			"git+https://github.com/user/repo",
			"README.md",
			false,
			WithMaxFileSize(100*1024*1024),
		)
		if err != nil {
			t.Fatalf("NewGitProvider() error = %v", err)
		}
		if p.maxFileSize != 100*1024*1024 {
			t.Errorf("maxFileSize = %v, want %v", p.maxFileSize, int64(100*1024*1024))
		}
	})

	t.Run("WithStorageFactory", func(t *testing.T) {
		t.Parallel()
		// Just ensure the option is applied without error
		p, err := NewGitProvider(
			"git+https://github.com/user/repo",
			"README.md",
			false,
			WithStorageFactory(MemoryStorageFactory()),
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
		tree:         tree,
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

			if tt.wantMIME != "" && !containsString(mimeType, tt.wantMIME) {
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
		tree:         tree,
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
			tree:         tree,
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
		if !containsString(contentStr, "a.md") {
			t.Error("listing should contain a.md")
		}
		if !containsString(contentStr, "b.md") {
			t.Error("listing should contain b.md")
		}
		if containsString(contentStr, ".hidden") {
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
			tree:         tree,
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
		tree:         tree,
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
			got := isLFSPointer(tt.content)
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

	p, err := NewGitProvider("git+https://github.com/user/repo", "README.md", false)
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

// containsString checks if str contains substr
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 ||
		(len(str) > 0 && len(substr) > 0 && findSubstring(str, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
	storage, err := factory()
	if err != nil {
		t.Errorf("MemoryStorageFactory() error = %v", err)
	}
	if storage == nil {
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
		tree:         tree,
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
		tree:         docsTree, // Use the subdirectory tree
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
