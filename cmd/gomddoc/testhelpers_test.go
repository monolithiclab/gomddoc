package main

import (
	"os"
	"path/filepath"
	"testing"
)

// lockedDir creates a fresh directory with the given permission bits and
// restores 0750 at cleanup, so t.TempDir()'s own RemoveAll can still delete it.
//
// mode is a parameter because the tests want two different failures out of the
// same setup: 0444 makes writes inside the directory fail, 0o000 makes os.Stat
// on a path *below* it fail with something other than fs.ErrNotExist.
func lockedDir(t *testing.T, mode os.FileMode) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0750) })

	return dir
}
