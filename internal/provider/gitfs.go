package provider

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// gitTreeFS adapts a go-git object.Tree to io/fs.FS.
// This enables serving theme overrides from Git repositories.
type gitTreeFS struct {
	tree    *object.Tree
	modTime time.Time
}

var _ fs.FS = (*gitTreeFS)(nil)

func (g *gitTreeFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return newGitDirFile(g.tree, ".", g.modTime), nil
	}

	// Try as file
	file, err := g.tree.File(name)
	if err == nil {
		return newGitBlobFile(file, g.modTime)
	}

	// Try as directory
	subtree, err := g.tree.Tree(name)
	if err == nil {
		return newGitDirFile(subtree, path.Base(name), g.modTime), nil
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

// gitDirFile implements fs.ReadDirFile for a git tree (directory).
type gitDirFile struct {
	info    *gitFileInfo
	tree    *object.Tree
	modTime time.Time
	entries []fs.DirEntry
	offset  int
}

func newGitDirFile(tree *object.Tree, name string, modTime time.Time) *gitDirFile {
	return &gitDirFile{
		info: &gitFileInfo{
			name:    name,
			mode:    fs.ModeDir | 0o755,
			modTime: modTime,
			isDir:   true,
		},
		tree:    tree,
		modTime: modTime,
	}
}

func (d *gitDirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *gitDirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: fs.ErrInvalid}
}
func (d *gitDirFile) Close() error { return nil }

func (d *gitDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		d.entries = make([]fs.DirEntry, 0, len(d.tree.Entries))
		for _, entry := range d.tree.Entries {
			if strings.HasPrefix(entry.Name, ".") {
				continue
			}
			isDir := !entry.Mode.IsFile()
			fileMode, _ := entry.Mode.ToOSFileMode()
			d.entries = append(d.entries, &gitDirEntry{
				name:     entry.Name,
				isDir:    isDir,
				fileMode: fileMode,
				modTime:  d.modTime,
			})
		}
	}

	if n <= 0 {
		entries := d.entries[d.offset:]
		d.offset = len(d.entries)
		if len(entries) == 0 {
			return nil, io.EOF
		}
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
	return remaining[:n], nil
}
