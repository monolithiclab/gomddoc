package provider

import (
	"io/fs"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
)

// createTestRepo creates an in-memory Git repository with the specified files.
// Files are specified as a map of path -> content.
func createTestRepo(t testing.TB, files map[string]string) *git.Repository {
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
func mustGetTree(t testing.TB, repo *git.Repository) *object.Tree {
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

// newTestTreeState pairs a tree with the storer that resolves it, which is the
// invariant gitTreeState carries in production (cloneLocked publishes both,
// Close clears both). Building the struct by hand is how a test ends up with a
// tree and a nil object storer — a state the provider never reaches, and a nil
// dereference the first time the code under test asks for an object by hash.
func newTestTreeState(tb testing.TB, repo *git.Repository, modTime time.Time) *gitTreeState {
	tb.Helper()
	return &gitTreeState{tree: mustGetTree(tb, repo), objects: repo.Storer, modTime: modTime}
}

// countingStorer separates the two ways go-git touches the object database:
// EncodedObject materialises an object (on a real packfile-backed repo, a
// delta + zlib decode), while EncodedObjectSize reads the header only.
//
// Under memory storage both are map lookups, so this counts *calls*, not
// nanoseconds — the calls are the invariant. Whether a decode happens is a
// property of the code under test; how much it costs is a property of the
// storer, and the deployed one is filesystem.ObjectStorage.
type countingStorer struct {
	storage.Storer
	objects atomic.Int64
	sizes   atomic.Int64
}

func (c *countingStorer) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	c.objects.Add(1)
	return c.Storer.EncodedObject(t, h)
}

func (c *countingStorer) EncodedObjectSize(h plumbing.Hash) (int64, error) {
	c.sizes.Add(1)
	return c.Storer.EncodedObjectSize(h)
}

// gitProviderOver builds a GitProvider whose tree resolves objects through
// storer — the repo's own, or a wrapper such as countingStorer. The tree is
// re-derived rather than taken from mustGetTree so that storer, not the repo's,
// is the one baked into it.
func gitProviderOver(tb testing.TB, repo *git.Repository, storer storage.Storer) *GitProvider {
	tb.Helper()

	head, err := repo.Head()
	if err != nil {
		tb.Fatalf("Head: %v", err)
	}
	commit, err := object.GetCommit(storer, head.Hash())
	if err != nil {
		tb.Fatalf("GetCommit: %v", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		tb.Fatalf("Tree: %v", err)
	}

	commitTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	return &GitProvider{
		parsedURL:    &ParsedGitURL{Ref: "HEAD"},
		defaultIndex: "README.md",
		dirIndex:     true,
		maxFileSize:  defaultMaxFileSize,
		repo:         repo,
		treeState:    gitTreeState{tree: tree, objects: storer, modTime: commitTime},
		commitTime:   commitTime,
	}
}

// newCountingProvider builds a provider whose tree reads all go through a
// countingStorer. The counters are zeroed once the root tree is decoded, so a
// delta measures only what the call under test did.
func newCountingProvider(tb testing.TB, files map[string]string) (*GitProvider, *countingStorer) {
	tb.Helper()

	repo := createTestRepo(tb, files)
	counting := &countingStorer{Storer: repo.Storer}
	p := gitProviderOver(tb, repo, counting)

	counting.objects.Store(0)
	counting.sizes.Store(0)
	return p, counting
}

// benchPage is a markdown page of realistic size (~8 KB), which is what the
// blob-read path is sized for.
var benchPage = strings.Repeat("lorem ipsum dolor sit amet consectetur ", 210)

// benchProvider builds a provider serving benchPage. No counting wrapper: its
// atomics would be measured alongside the work.
func benchProvider(tb testing.TB) *GitProvider {
	tb.Helper()

	repo := createTestRepo(tb, map[string]string{
		"README.md":     "# Test",
		"docs/guide.md": benchPage,
	})
	return gitProviderOver(tb, repo, repo.Storer)
}

// benchRootFS is benchProvider's content seen through io/fs, the surface the
// index builds and the sitemap read.
func benchRootFS(tb testing.TB) fs.FS {
	tb.Helper()

	fsys, err := benchProvider(tb).RootFS(tb.Context())
	if err != nil {
		tb.Fatalf("RootFS: %v", err)
	}
	return fsys
}
