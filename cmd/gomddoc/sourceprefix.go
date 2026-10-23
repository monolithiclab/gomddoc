package main

import (
	"os"
	"path/filepath"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// sourcePrefix returns the content directory's path relative to the root of
// the git work tree holding it, slash-separated: "docs" for `gomddoc serve
// docs` run in a repository. Page source paths carry it so a harness working
// from the repository root opens the right file. It is "" when dir is the work
// tree root, is not inside one, or is a git URL (whose paths are already
// relative to the tree).
func sourcePrefix(dir string) string {
	if dir == "" || config.IsGitURL(dir) {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	// A linked worktree or a submodule has a .git file, not a directory.
	for top := abs; ; {
		if _, err := os.Stat(filepath.Join(top, ".git")); err == nil {
			rel, err := filepath.Rel(top, abs)
			if err != nil || rel == "." {
				return ""
			}
			return filepath.ToSlash(rel)
		}
		parent := filepath.Dir(top)
		if parent == top {
			return ""
		}
		top = parent
	}
}
