package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourcePrefix(t *testing.T) {
	t.Parallel()

	// mkdirs creates root/rel, plus a .git directory (or file, as in a linked
	// worktree) at root/gitAt when gitAt is set.
	mkdirs := func(t *testing.T, rel, gitAt string, gitFile bool) string {
		t.Helper()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, rel), 0o750); err != nil {
			t.Fatal(err)
		}
		if gitAt != "" {
			dotGit := filepath.Join(root, gitAt, ".git")
			var err error
			if gitFile {
				err = os.WriteFile(dotGit, []byte("gitdir: /elsewhere\n"), 0o600)
			} else {
				err = os.Mkdir(dotGit, 0o750)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(root, rel)
	}

	tests := []struct {
		name    string
		rel     string
		gitAt   string
		gitFile bool
		want    string
	}{
		{name: "repo root", rel: "repo", gitAt: "repo", want: ""},
		{name: "subdirectory", rel: "repo/docs", gitAt: "repo", want: "docs"},
		{name: "nested subdirectory", rel: "repo/a/b", gitAt: "repo", want: "a/b"},
		{name: "worktree .git file", rel: "repo/docs", gitAt: "repo", gitFile: true, want: "docs"},
		// Assumes no ancestor of t.TempDir() holds a .git (true of the OS temp dir).
		{name: "no work tree", rel: "plain/docs", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := mkdirs(t, tt.rel, tt.gitAt, tt.gitFile)
			if got := sourcePrefix(dir); got != tt.want {
				t.Errorf("sourcePrefix(%q) = %q, want %q", dir, got, tt.want)
			}
		})
	}

	for url, want := range map[string]string{
		"git+https://example.com/repo.git":               "",
		"git+https://example.com/monorepo#main:docs/api": "docs/api",
		"git+https://example.com/monorepo#main:/docs/":   "docs",
	} {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			if got := sourcePrefix(url); got != want {
				t.Errorf("sourcePrefix(%q) = %q, want %q", url, got, want)
			}
		})
	}
}
