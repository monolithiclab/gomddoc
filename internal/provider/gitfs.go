package provider

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// gitTreeState owns the cached git tree and is the single synchronisation point
// for every post-clone read of the git object graph, shared by GitProvider and
// by every gitTreeFS instance RootFS hands out.
//
// The lock is exclusive rather than an RWMutex on purpose, for two reasons:
//
//   - object.Tree memoises lookups by writing unsynchronised maps
//     (Tree.FindEntry writes t.t, Tree.entry writes t.m), so File and Tree are
//     *writes* to the receiver.
//   - the storer underneath is not safe for concurrent use either. Both
//     go-git storage backends mutate on read, and filesystem.ObjectStorage
//     shares one packfile handle it seeks on.
//
// Concurrent "readers" therefore hit a concurrent map write, which the runtime
// turns into an unrecoverable throw — no middleware can recover from it. Note
// that this rules out the obvious refactor of dropping the shared tree and
// re-deriving commit.Tree() per request: that races in the storer instead, and
// costs a fresh decode of every tree along the path. The only safe way to
// parallelise git reads is N independent repo handles, not a finer lock.
//
// When the provider is closed, the tree reference is nilled under the lock,
// which prevents use-after-close and breaks the reference chain so the git
// object graph (tree → storer) can be garbage collected.
type gitTreeState struct {
	mu      sync.Mutex
	tree    *object.Tree
	modTime time.Time
}

// gitTreeFS adapts a go-git object.Tree to io/fs.FS.
// It shares state with the GitProvider via gitTreeState so that
// Close() invalidates all outstanding FS references.
type gitTreeFS struct {
	state *gitTreeState
}

var _ fs.FS = (*gitTreeFS)(nil)

func (g *gitTreeFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	g.state.mu.Lock()
	defer g.state.mu.Unlock()

	if g.state.tree == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrClosed}
	}

	tree := g.state.tree
	modTime := g.state.modTime

	if name == "." {
		return newGitDirFile(tree, ".", modTime), nil
	}

	// Try as file
	file, err := tree.File(name)
	if err == nil {
		return newGitBlobFile(file, modTime)
	}

	// Try as directory
	subtree, err := tree.Tree(name)
	if err == nil {
		return newGitDirFile(subtree, path.Base(name), modTime), nil
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// gitBlobFile implements fs.File for a git blob (regular file).
type gitBlobFile struct {
	info   *gitFileInfo
	reader *bytes.Reader
}

func newGitBlobFile(file *object.File, modTime time.Time) (*gitBlobFile, error) {
	content, err := file.Contents()
	if err != nil {
		return nil, err
	}
	fileMode, _ := file.Mode.ToOSFileMode()
	return &gitBlobFile{
		info: &gitFileInfo{
			name:    path.Base(file.Name),
			size:    file.Size,
			mode:    fileMode,
			modTime: modTime,
		},
		reader: bytes.NewReader([]byte(content)),
	}, nil
}

func (f *gitBlobFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *gitBlobFile) Read(b []byte) (int, error) { return f.reader.Read(b) }
func (f *gitBlobFile) Close() error               { return nil }

// treeDirEntries converts a tree's entries to fs.DirEntry, skipping hidden
// files. Must be called with gitTreeState.mu held when tree is the shared root
// tree. Sizes are omitted: reporting one would mean loading every blob.
func treeDirEntries(tree *object.Tree, modTime time.Time) []fs.DirEntry {
	entries := make([]fs.DirEntry, 0, len(tree.Entries))
	for _, entry := range tree.Entries {
		if strings.HasPrefix(entry.Name, ".") {
			continue
		}
		fileMode, _ := entry.Mode.ToOSFileMode()
		entries = append(entries, &gitDirEntry{
			name:     entry.Name,
			isDir:    !entry.Mode.IsFile(),
			fileMode: fileMode,
			modTime:  modTime,
		})
	}
	return entries
}

// gitDirFile implements fs.ReadDirFile for a git tree (directory).
//
// It holds the materialised entries rather than the tree itself: Open may hand
// back the shared root tree, and the handle outlives the lock that guards it.
// Copying also lets Close() release the git object graph while a handle is
// still open.
type gitDirFile struct {
	info    *gitFileInfo
	entries []fs.DirEntry
	offset  int
}

// newGitDirFile must be called with gitTreeState.mu held.
func newGitDirFile(tree *object.Tree, name string, modTime time.Time) *gitDirFile {
	return &gitDirFile{
		info: &gitFileInfo{
			name:    name,
			mode:    fs.ModeDir | 0o755,
			modTime: modTime,
			isDir:   true,
		},
		entries: treeDirEntries(tree, modTime),
	}
}

func (d *gitDirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *gitDirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: fs.ErrInvalid}
}
func (d *gitDirFile) Close() error { return nil }

func (d *gitDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		entries := slices.Clone(d.entries[d.offset:])
		d.offset = len(d.entries)
		return entries, nil
	}

	remaining := d.entries[d.offset:]
	if len(remaining) == 0 {
		return nil, io.EOF
	}
	if n > len(remaining) {
		n = len(remaining)
	}
	d.offset += n
	return slices.Clone(remaining[:n]), nil
}
