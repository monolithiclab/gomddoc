// Package countfs wraps an fs.FS and counts the Open calls that pass through
// it, which is how a test tells a cached content walk apart from one repeated
// per call.
package countfs

import (
	"io/fs"
	"sync/atomic"
)

// FS counts Open calls against the filesystem it wraps. Only Open — a caller
// counting ReadFile or ReadDir traffic gets it only because io/fs falls back to
// Open, which a wrapped FS implementing fs.ReadFileFS would defeat.
//
// The embedded field must stay fs.FS, not the concrete type: promoting a
// ReadDir method would satisfy fs.ReadDirFS, and fs.ReadDir would then bypass
// Open entirely. A rebuild would walk the tree without touching the counter and
// every assertion resting on it would go permanently green — the failure mode
// this helper exists to catch, hiding inside the helper itself.
type FS struct {
	fs.FS
	opens atomic.Int64
}

// New returns an FS wrapping fsys with its counter at zero.
func New(fsys fs.FS) *FS {
	return &FS{FS: fsys}
}

// Open records the call and delegates.
func (c *FS) Open(name string) (fs.File, error) {
	c.opens.Add(1)
	return c.FS.Open(name)
}

// Opens reports how many times Open has been called. Safe to call while other
// goroutines are reading through the FS.
func (c *FS) Opens() int64 {
	return c.opens.Load()
}
