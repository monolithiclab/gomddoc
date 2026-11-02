package server

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
)

// previewHandler serves preview's site and rebuilds it whenever a file under
// the content directory changes, so a browser reload shows the edit. Rendering
// already reads page bodies from disk on every request (DevMode); what goes stale
// without a rebuild is everything built at startup: navigation, titles, the page
// list (a new page answers 404), search, redirects, tags and the site config.
//
// The check runs on every request: a stat walk of a few hundred files costs
// about a millisecond, and preview serves one reader. Requests are serialized
// around it so one edit triggers one rebuild.
type previewHandler struct {
	dir   string
	build func() (http.Handler, error)

	mu      sync.Mutex
	stamp   uint64
	current http.Handler
}

// NewPreviewHandler wraps initial, built from dir when its TreeStamp was stamp,
// and calls build for a fresh handler each time the stamp changes. A failed
// build is shown as a 500 naming the error until the next change.
func NewPreviewHandler(dir string, stamp uint64, initial http.Handler, build func() (http.Handler, error)) http.Handler {
	return &previewHandler{dir: dir, build: build, stamp: stamp, current: initial}
}

func (p *previewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only a page load checks the tree: the assets, images and fetches it
	// triggers next read their files live, and would each repeat the walk.
	if dest := r.Header.Get("Sec-Fetch-Dest"); dest != "" && dest != "document" {
		p.mu.Lock()
		current := p.current
		p.mu.Unlock()
		current.ServeHTTP(w, r)
		return
	}
	p.handler().ServeHTTP(w, r)
}

// handler returns the handler for the tree as it is now, rebuilding it first
// when the tree changed since the last build.
func (p *previewHandler) handler() http.Handler {
	p.mu.Lock()
	defer p.mu.Unlock()

	stamp, err := TreeStamp(p.dir)
	if err != nil {
		slog.Warn("Preview cannot read the content directory", slog.Any("error", err))
		return p.current
	}
	if stamp == p.stamp {
		return p.current
	}
	p.stamp = stamp
	next, err := p.build()
	if err != nil {
		slog.Error("Preview rebuild failed", slog.Any("error", err))
		next = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", cacheRevalidate)
			http.Error(w, "gomddoc preview: "+err.Error(), http.StatusInternalServerError)
		})
	} else {
		slog.Info("Preview rebuilt after a change", slog.String("dir", p.dir))
	}
	p.current = next
	return next
}

// revalidateKey marks a dev-mode request: serveWithETag answers it with
// cacheRevalidate whatever the route's own policy.
type revalidateKey struct{}

// Revalidate marks every request so its response tells the browser to ask
// again on the next load (DevMode).
func Revalidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), revalidateKey{}, true)))
	})
}

func revalidates(r *http.Request) bool {
	v, _ := r.Context().Value(revalidateKey{}).(bool)
	return v
}

// TreeStamp hashes the path, size and modification time of every file under
// dir, .git excluded, so any edit, addition or removal changes it.
func TreeStamp(dir string) (uint64, error) {
	h := fnv.New64a()
	var buf [16]byte
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error { // #nosec G703 -- dir is the content directory the operator passed to preview
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(path))
		binary.LittleEndian.PutUint64(buf[:8], uint64(info.Size()))               // #nosec G115 -- only hashed
		binary.LittleEndian.PutUint64(buf[8:], uint64(info.ModTime().UnixNano())) // #nosec G115 -- only hashed
		_, _ = h.Write(buf[:])
		return nil
	})
	return h.Sum64(), err
}
