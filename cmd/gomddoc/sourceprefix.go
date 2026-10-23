package main

import (
	"os"
	"path/filepath"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// sourcePrefix returns the content directory's path relative to the root of
// the git work tree holding it, slash-separated: "docs" for `gomddoc serve
// docs` run in a repository. Page source paths carry it so a harness working
// from the repository root opens the right file. It is "" when dir is the work
// tree root or is not inside one. For a git URL it is the fragment's subdir
// ("docs/api" for #main:docs/api), the provider's root inside the tree.
func sourcePrefix(dir string) string {
	if config.IsGitURL(dir) {
		return provider.GitURLSubdir(dir)
	}
	if dir == "" {
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
