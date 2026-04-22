package provider

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// Default configuration values for GitProvider
const (
	defaultCloneTimeout = 60 * time.Second
	defaultMaxFileSize  = 50 * 1024 * 1024 // 50 MB
)

// StorageFactory creates storage backends for Git repositories.
// This abstraction enables switching between memory and disk-based storage.
type StorageFactory func() (storage.Storer, error)

// MemoryStorageFactory returns a factory that creates in-memory storage.
// This is the default for ephemeral, fast-startup deployments.
func MemoryStorageFactory() StorageFactory {
	return func() (storage.Storer, error) {
		return memory.NewStorage(), nil
	}
}

// DiskStorageFactory returns a factory that creates disk-based storage
// at the given directory path. The directory is created if it does not exist.
// This is suitable for large repositories where in-memory storage would cause
// out-of-memory errors.
func DiskStorageFactory(dir string) StorageFactory {
	return func() (storage.Storer, error) {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create storage dir: %w", err)
		}
		return filesystem.NewStorage(osfs.New(dir), cache.NewObjectLRUDefault()), nil
	}
}

// GitProviderConfig holds optional configuration for GitProvider.
// Zero values use defaults: memory storage, 60s clone timeout, 50MB max file size.
type GitProviderConfig struct {
	StorageFactory StorageFactory // Custom storage backend (default: memory)
	CloneTimeout   time.Duration  // Clone operation timeout (default: 60s)
	MaxFileSize    int64          // Maximum file size to serve (default: 50MB)
	SSHKeyFile     string         // Path to SSH private key file
}

// GitProvider implements Provider for remote Git repositories.
// It clones the repository into storage on first access and serves
// files from the object database.
//
// Storage Strategy:
// The provider uses storage.Storer interface, allowing both memory and
// disk-based backends. Memory storage (default) provides:
//   - Fast startup (no disk I/O)
//   - Ephemeral data (cleared on restart)
//   - Suitable for small-to-medium repositories
type GitProvider struct {
	// Configuration (immutable after construction)
	parsedURL       *ParsedGitURL
	defaultIndex    string
	dirIndex        bool
	excludePatterns []string
	cloneTimeout    time.Duration
	maxFileSize     int64
	sshKeyFile      string
	storageFactory  StorageFactory

	// Authentication (nil for anonymous)
	auth transport.AuthMethod

	// Lazily initialized state (protected by mu)
	mu         sync.RWMutex
	closed     bool
	storage    storage.Storer
	repo       *git.Repository
	tree       *object.Tree
	commitTime time.Time
	commitHash plumbing.Hash

	// Shared state for gitTreeFS instances returned by RootFS().
	// When Close() nils fsState.tree, all outstanding FS references
	// become invalid, breaking the reference chain to the git object graph.
	fsState gitFSState
}

// NewGitProvider creates a Git provider from a git:// URL.
//
// The provider clones lazily on first ReadFile/Stat call, not at construction.
// This allows fast startup and proper error reporting with HTTP context.
//
// Errors:
//   - ErrInvalidGitURL: URL is malformed or uses unsupported scheme
func NewGitProvider(gitURL, defaultIndex string, dirIndex bool, excludePatterns []string, cfg GitProviderConfig) (*GitProvider, error) {
	if defaultIndex == "" {
		return nil, ErrEmptyDefaultIndex
	}

	parsed, err := parseGitURL(gitURL)
	if err != nil {
		return nil, &PathError{Op: "parse", Path: gitURL, Err: ErrInvalidGitURL}
	}

	g := &GitProvider{
		parsedURL:       parsed,
		defaultIndex:    defaultIndex,
		dirIndex:        dirIndex,
		excludePatterns: excludePatterns,
		cloneTimeout:    defaultCloneTimeout,
		maxFileSize:     defaultMaxFileSize,
		storageFactory:  MemoryStorageFactory(),
		auth:            nil, // Authentication set up in ensureCloned
	}

	if cfg.StorageFactory != nil {
		g.storageFactory = cfg.StorageFactory
	}
	if cfg.CloneTimeout > 0 {
		g.cloneTimeout = cfg.CloneTimeout
	}
	if cfg.MaxFileSize > 0 {
		g.maxFileSize = cfg.MaxFileSize
	}
	if cfg.SSHKeyFile != "" {
		g.sshKeyFile = cfg.SSHKeyFile
	}

	return g, nil
}

// DefaultIndex returns the configured default index file.
func (g *GitProvider) DefaultIndex() string {
	return g.defaultIndex
}

// RootFS returns the content root as an fs.FS backed by the cloned tree.
// Triggers a clone if the repository has not been cloned yet.
func (g *GitProvider) RootFS(ctx context.Context) (fs.FS, error) {
	if err := g.ensureCloned(ctx); err != nil {
		return nil, err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return &gitTreeFS{state: &g.fsState}, nil
}

// Close releases resources held by the provider.
// This clears cached state, allowing GC to reclaim memory.
func (g *GitProvider) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.closed = true
	g.repo = nil
	g.tree = nil
	g.storage = nil

	// Invalidate all outstanding gitTreeFS references so they
	// return fs.ErrClosed and break the reference chain to the storer.
	g.fsState.mu.Lock()
	g.fsState.tree = nil
	g.fsState.mu.Unlock()

	return nil
}

// ensureCloned performs lazy initialization of the repository.
// Safe for concurrent calls - only the first caller clones.
func (g *GitProvider) ensureCloned(ctx context.Context) error {
	// Fast path: already cloned
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return &PathError{Op: "read", Path: "", Err: ErrProviderClosed}
	}
	if g.repo != nil {
		g.mu.RUnlock()
		return nil
	}
	g.mu.RUnlock()

	// Slow path: need to clone
	g.mu.Lock()
	defer g.mu.Unlock()

	// Double-check after acquiring write lock
	if g.closed {
		return &PathError{Op: "read", Path: "", Err: ErrProviderClosed}
	}
	if g.repo != nil {
		return nil
	}

	return g.cloneLocked(ctx)
}

// cloneLocked performs the actual clone operation.
// Must be called with g.mu held for writing.
func (g *GitProvider) cloneLocked(ctx context.Context) error {
	stor, err := g.storageFactory()
	if err != nil {
		return &PathError{Op: "storage", Path: g.parsedURL.Endpoint.String(), Err: err}
	}
	g.storage = stor

	// Set up authentication if needed
	if err := g.setupAuthLocked(); err != nil {
		return err
	}

	cloneOpts := &git.CloneOptions{
		URL:          g.parsedURL.Endpoint.String(),
		Auth:         g.auth,
		Depth:        1,          // Shallow clone
		SingleBranch: true,       // Only fetch requested ref
		Tags:         git.NoTags, // Skip tags for speed
	}

	// Set the reference name if not HEAD
	if g.parsedURL.Ref != "HEAD" {
		// Try as branch first
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(g.parsedURL.Ref)
	}

	ctx, cancel := context.WithTimeout(ctx, g.cloneTimeout)
	defer cancel()

	repo, err := git.CloneContext(ctx, g.storage, nil, cloneOpts)
	if err != nil {
		return g.classifyCloneError(err)
	}
	g.repo = repo

	// Resolve the reference to a commit
	if err := g.resolveCommitLocked(); err != nil {
		return err
	}

	// Cache the tree for fast file access
	return g.cacheTreeLocked()
}

// setupAuthLocked configures authentication based on the endpoint protocol.
// Must be called with g.mu held for writing.
func (g *GitProvider) setupAuthLocked() error {
	auth, err := resolveAuth(g.parsedURL.Endpoint, g.sshKeyFile)
	if err != nil {
		return &PathError{Op: "auth", Path: g.parsedURL.Endpoint.String(), Err: err}
	}
	g.auth = auth
	return nil
}

// resolveCommitLocked resolves the ref to a commit hash.
// Must be called with g.mu held for writing.
func (g *GitProvider) resolveCommitLocked() error {
	// Try to resolve as revision (branch, tag, or commit hash)
	hash, err := g.repo.ResolveRevision(plumbing.Revision(g.parsedURL.Ref))
	if err != nil {
		// Fallback: try HEAD if ref resolution fails
		if g.parsedURL.Ref == "HEAD" {
			head, err := g.repo.Head()
			if err != nil {
				return &PathError{Op: "resolve", Path: "HEAD", Err: ErrGitRefNotFound}
			}
			g.commitHash = head.Hash()
		} else {
			return &PathError{Op: "resolve", Path: g.parsedURL.Ref, Err: ErrGitRefNotFound}
		}
	} else {
		g.commitHash = *hash
	}

	// Get commit for timestamp
	commit, err := g.repo.CommitObject(g.commitHash)
	if err != nil {
		return &PathError{Op: "commit", Path: g.commitHash.String(), Err: err}
	}
	g.commitTime = commit.Author.When

	return nil
}

// cacheTreeLocked caches the root tree (or subdir tree) for fast access.
// Must be called with g.mu held for writing.
func (g *GitProvider) cacheTreeLocked() error {
	commit, err := g.repo.CommitObject(g.commitHash)
	if err != nil {
		return &PathError{Op: "commit", Path: g.commitHash.String(), Err: err}
	}

	tree, err := commit.Tree()
	if err != nil {
		return &PathError{Op: "tree", Path: g.commitHash.String(), Err: err}
	}

	// If subdir specified, navigate to it
	if g.parsedURL.Subdir != "" {
		subtree, err := tree.Tree(g.parsedURL.Subdir)
		if err != nil {
			return &PathError{Op: "subdir", Path: g.parsedURL.Subdir, Err: ErrNotFound}
		}
		tree = subtree
	}

	g.tree = tree

	// Update shared FS state so gitTreeFS instances see the new tree.
	g.fsState.mu.Lock()
	g.fsState.tree = tree
	g.fsState.modTime = g.commitTime
	g.fsState.mu.Unlock()

	return nil
}

// classifyCloneError maps clone errors to appropriate sentinel errors.
// Uses errors.Is() against go-git's typed errors where available,
// with string fallbacks only for errors go-git does not export.
func (g *GitProvider) classifyCloneError(err error) error {
	endpoint := g.parsedURL.Endpoint.String()

	switch {
	case errors.Is(err, transport.ErrAuthenticationRequired),
		errors.Is(err, transport.ErrAuthorizationFailed):
		return &PathError{Op: "clone", Path: endpoint, Err: ErrGitAuthFailed}

	// go-git does not export a sentinel for "could not read Username"
	case strings.Contains(err.Error(), "could not read Username"):
		return &PathError{Op: "clone", Path: endpoint, Err: ErrGitAuthFailed}

	case errors.Is(err, transport.ErrRepositoryNotFound):
		return &PathError{Op: "clone", Path: endpoint, Err: ErrNotFound}

	case errors.Is(err, plumbing.ErrReferenceNotFound):
		return &PathError{Op: "clone", Path: endpoint, Err: ErrGitRefNotFound}

	// go-git does not export a sentinel for connection errors
	case strings.Contains(err.Error(), "connection refused"):
		return &PathError{Op: "clone", Path: endpoint, Err: ErrGitConnectFailed}

	default:
		return &PathError{Op: "clone", Path: endpoint, Err: err}
	}
}

// ReadFile reads a file at the given path and returns its content with MIME type.
func (g *GitProvider) ReadFile(ctx context.Context, requestPath string) ([]byte, string, error) {
	if err := g.ensureCloned(ctx); err != nil {
		return nil, "", err
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	cleanPath := normalizePath(requestPath)

	// Handle root directory
	if cleanPath == "." {
		return g.handleDirectoryLocked(".", requestPath)
	}

	// Try to get as file first
	file, err := g.tree.File(cleanPath)
	if err == nil {
		return g.readFileLocked(file, requestPath)
	}

	// Try as directory
	if errors.Is(err, object.ErrFileNotFound) {
		_, treeErr := g.tree.Tree(cleanPath)
		if treeErr == nil {
			return g.handleDirectoryLocked(cleanPath, requestPath)
		}
	}

	return nil, "", &PathError{Op: "read", Path: requestPath, Err: ErrNotFound}
}

// readFileLocked reads the content of a file object.
// Must be called with g.mu held for reading.
func (g *GitProvider) readFileLocked(file *object.File, requestPath string) ([]byte, string, error) {
	// Check file size limit
	if file.Size > g.maxFileSize {
		return nil, "", &PathError{
			Op:   "read",
			Path: requestPath,
			Err:  ErrFileTooLarge,
		}
	}

	content, err := file.Contents()
	if err != nil {
		return nil, "", &PathError{Op: "read", Path: requestPath, Err: err}
	}

	// Check for Git LFS pointer (LFS pointers are small)
	if file.Size < 200 && isLFSPointer(content) {
		return nil, "", &PathError{
			Op:   "read",
			Path: requestPath,
			Err:  ErrGitLFSNotSupported,
		}
	}

	mimeType := negotiate.DetectMIME(requestPath)
	return []byte(content), mimeType, nil
}

// handleDirectoryLocked processes directory requests.
// Must be called with g.mu held for reading.
func (g *GitProvider) handleDirectoryLocked(dirPath, requestPath string) ([]byte, string, error) {
	// Determine which tree to use
	var dirTree *object.Tree
	if dirPath == "." {
		dirTree = g.tree
	} else {
		var err error
		dirTree, err = g.tree.Tree(dirPath)
		if err != nil {
			return nil, "", &PathError{Op: "read", Path: requestPath, Err: ErrNotFound}
		}
	}

	return handleDirectory(requestPath, g.defaultIndex, g.dirIndex, g.excludePatterns,
		func() ([]byte, error) {
			indexFile, err := dirTree.File(g.defaultIndex)
			if err != nil {
				return nil, err
			}
			content, err := indexFile.Contents()
			if err != nil {
				return nil, err
			}
			return []byte(content), nil
		},
		func() ([]fs.DirEntry, error) { return g.listDirectoryLocked(dirTree) },
	)
}

// listDirectoryLocked creates fs.DirEntry slice from a tree.
// Must be called with g.mu held for reading.
func (g *GitProvider) listDirectoryLocked(tree *object.Tree) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry

	for _, entry := range tree.Entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name, ".") {
			continue
		}

		isDir := !entry.Mode.IsFile()
		fileMode, _ := entry.Mode.ToOSFileMode()
		entries = append(entries, &gitDirEntry{
			name:     entry.Name,
			isDir:    isDir,
			fileMode: fileMode,
			size:     0, // Size requires loading blob, skip for listing
			modTime:  g.commitTime,
		})
	}

	return entries, nil
}

// Stat returns a FileInfo describing the named file.
func (g *GitProvider) Stat(ctx context.Context, requestPath string) (fs.FileInfo, error) {
	if err := g.ensureCloned(ctx); err != nil {
		return nil, err
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	cleanPath := normalizePath(requestPath)

	// Root directory
	if cleanPath == "." {
		return &gitFileInfo{
			name:    "/",
			size:    0,
			mode:    fs.ModeDir | 0755,
			modTime: g.commitTime,
			isDir:   true,
		}, nil
	}

	// Try as file
	file, err := g.tree.File(cleanPath)
	if err == nil {
		fileMode, _ := file.Mode.ToOSFileMode()
		return &gitFileInfo{
			name:    path.Base(cleanPath),
			size:    file.Size,
			mode:    fileMode,
			modTime: g.commitTime,
			isDir:   false,
		}, nil
	}

	// Try as directory
	_, err = g.tree.Tree(cleanPath)
	if err == nil {
		return &gitFileInfo{
			name:    path.Base(cleanPath),
			size:    0,
			mode:    fs.ModeDir | 0755,
			modTime: g.commitTime,
			isDir:   true,
		}, nil
	}

	return nil, &PathError{Op: "stat", Path: requestPath, Err: ErrNotFound}
}

// normalizePath cleans and normalizes a request path for use with git trees.
func normalizePath(requestPath string) string {
	if requestPath == "/" || requestPath == "" {
		return "."
	}
	clean := path.Clean(requestPath)
	return strings.TrimPrefix(clean, "/")
}

// isLFSPointer checks if content is a Git LFS pointer file.
func isLFSPointer(content string) bool {
	return strings.HasPrefix(content, "version https://git-lfs.github.com/")
}
