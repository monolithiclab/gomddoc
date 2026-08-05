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

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
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
	mu   sync.Mutex
	tree *object.Tree
	// objects is the tree's own object database, held here rather than read
	// back from GitProvider so that the pair stays under one lock: cloneLocked
	// publishes them together and Close clears them together. object.Tree keeps
	// its storer unexported, so anything that wants an object by the hash a
	// walk already produced — a size without a decode, a subtree without a
	// second walk — needs this.
	objects storer.EncodedObjectStorer
	modTime time.Time
}

// resolveTreeNode resolves p against tree in a single path walk, returning
// either the blob as a *File or the subtree — never both.
//
// The obvious form, tree.File(p) with a fallback to tree.Tree(p) on
// ErrFileNotFound, pays a discarded object decode on every directory hit:
// File runs GetBlob against the subtree's own hash and only then fails the
// type check, which on the filesystem storer is a full packfile delta + zlib
// decode thrown away. Switching on the entry's mode picks the right accessor
// the first time, and both accessors take the hash the walk already produced —
// tree.Tree(p) would walk p a second time.
//
// Anything that is not a directory is tried as a blob, which is what the old
// code did: symlinks stay readable (their target is the blob's content) and
// submodule gitlinks still fail inside GetBlob.
//
// Must be called with gitTreeState.mu held — see gitTreeState.
func resolveTreeNode(tree *object.Tree, objects storer.EncodedObjectStorer, p string) (*object.File, *object.Tree, error) {
	entry, err := tree.FindEntry(p)
	if err != nil {
		return nil, nil, err
	}
	if entry.Mode == filemode.Dir {
		subtree, err := object.GetTree(objects, entry.Hash)
		return nil, subtree, err
	}
	file, err := tree.TreeEntryFile(entry)
	if err != nil {
		return nil, nil, err
	}
	// TreeEntryFile names the file after the entry, whose Name is the base name
	// alone; restore tree.File's contract so callers can trust file.Name.
	file.Name = p
	return file, nil, nil
}

// statTreeNode describes name from the tree entry and, for a blob, the object
// header — it never materialises an object. tree.File would decode a whole blob
// for a size the header already carries, and tree.Tree would decode a whole
// subtree just to prove one exists.
//
// Must be called with gitTreeState.mu held — see gitTreeState.
func statTreeNode(tree *object.Tree, objects storer.EncodedObjectStorer, name string, modTime time.Time) (*gitFileInfo, error) {
	entry, err := tree.FindEntry(name)
	if err != nil {
		return nil, err
	}
	if entry.Mode == filemode.Dir {
		return &gitFileInfo{
			name:    path.Base(name),
			mode:    fs.ModeDir | 0o755,
			modTime: modTime,
			isDir:   true,
		}, nil
	}

	// The size is asked of the storer rather than of tree.Size, which would
	// walk the path a second time: Tree.FindEntry only consults its subtree
	// cache from three segments up (its loop runs `for i := len(pathParts) - 1;
	// i > 1`), so a second call for "docs/guide.md" decodes the docs tree all
	// over again.
	size, err := objects.EncodedObjectSize(entry.Hash)
	if err != nil {
		return nil, err
	}
	fileMode, _ := entry.Mode.ToOSFileMode()
	return &gitFileInfo{
		name:    path.Base(name),
		size:    size,
		mode:    fileMode,
		modTime: modTime,
	}, nil
}

// blobBytes reads f's blob into one exact-size allocation.
//
// file.Contents() grows a bytes.Buffer, copies it into a string, and every
// caller here copied that string back into a []byte: ~3N allocated and ~4N
// copied for an N-byte page. f.Size is the blob's plaintext size, so the
// destination can be sized up front and filled once.
func blobBytes(f *object.File) ([]byte, error) {
	r, err := f.Reader()
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	buf := make([]byte, f.Size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// gitTreeFS adapts a go-git object.Tree to io/fs.FS.
// It shares state with the GitProvider via gitTreeState so that
// Close() invalidates all outstanding FS references.
type gitTreeFS struct {
	state *gitTreeState
}

var (
	_ fs.FS         = (*gitTreeFS)(nil)
	_ fs.StatFS     = (*gitTreeFS)(nil)
	_ fs.ReadFileFS = (*gitTreeFS)(nil)
)

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

	file, subtree, err := resolveTreeNode(tree, g.state.objects, name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if subtree != nil {
		return newGitDirFile(subtree, path.Base(name), modTime), nil
	}
	return newGitBlobFile(file, modTime)
}

// Stat answers without opening the file. Without it fs.Stat falls back to Open
// plus File.Stat, and Open decodes the whole blob: sitemap and feed generation
// stat every page in the site for a ModTime that is the commit timestamp, the
// same constant for every file in the tree.
func (g *gitTreeFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	g.state.mu.Lock()
	defer g.state.mu.Unlock()

	if g.state.tree == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrClosed}
	}

	if name == "." {
		return &gitFileInfo{
			name:    ".",
			mode:    fs.ModeDir | 0o755,
			modTime: g.state.modTime,
			isDir:   true,
		}, nil
	}

	info, err := statTreeNode(g.state.tree, g.state.objects, name, g.state.modTime)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return info, nil
}

// ReadFile hands back blobBytes' exact-size slice. The io/fs fallback opens the
// file and copies it into a second buffer, which is the copy blobBytes exists
// to avoid — and the metadata and search index builds read every file in the
// repository through here.
func (g *gitTreeFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}

	g.state.mu.Lock()
	defer g.state.mu.Unlock()

	if g.state.tree == nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrClosed}
	}

	file, subtree, err := resolveTreeNode(g.state.tree, g.state.objects, name)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	if subtree != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	return blobBytes(file)
}

// gitBlobFile implements fs.File for a git blob (regular file).
type gitBlobFile struct {
	info   *gitFileInfo
	reader *bytes.Reader
}

// newGitBlobFile wraps a blob as an fs.File.
func newGitBlobFile(file *object.File, modTime time.Time) (*gitBlobFile, error) {
	content, err := blobBytes(file)
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
		reader: bytes.NewReader(content),
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
