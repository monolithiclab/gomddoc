package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// previewSite is a temp content tree and a build func that serves its index.md
// as read at build time, so a response shows which build answered it.
type previewSite struct {
	dir    string
	builds int
	fail   error
}

func newPreviewSite(t *testing.T) *previewSite {
	t.Helper()
	s := &previewSite{dir: t.TempDir()}
	s.write(t, "index.md", "v1")
	return s
}

func (s *previewSite) write(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(s.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (s *previewSite) build() (http.Handler, error) {
	s.builds++
	if s.fail != nil {
		return nil, s.fail
	}
	body, err := os.ReadFile(filepath.Join(s.dir, "index.md"))
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWithETag(w, r, body, mimePlain, cacheDynamic)
	}), nil
}

// handler builds the site once, as setupServer does, and wraps it.
func (s *previewSite) handler(t *testing.T) http.Handler {
	t.Helper()
	stamp, err := TreeStamp(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := s.build()
	if err != nil {
		t.Fatal(err)
	}
	return NewPreviewHandler(s.dir, stamp, initial, s.build)
}

func get(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec
}

func TestPreviewHandler_RebuildsOnlyWhenTheTreeChanges(t *testing.T) {
	t.Parallel()
	s := newPreviewSite(t)
	h := s.handler(t)

	if body := get(h).Body.String(); body != "v1" || s.builds != 1 {
		t.Fatalf("first request: body %q after %d builds, want v1 after 1", body, s.builds)
	}

	s.write(t, "index.md", "version two")
	if body := get(h).Body.String(); body != "version two" {
		t.Errorf("after an edit: body %q, want %q", body, "version two")
	}
	get(h)
	if s.builds != 2 {
		t.Errorf("builds = %d, want 2: one rebuild per change, none for an unchanged tree", s.builds)
	}

	// A new file anywhere in the tree (a page, a theme asset) is a change too.
	s.write(t, "guide/new.md", "x")
	get(h)
	if s.builds != 3 {
		t.Errorf("builds = %d after adding a file, want 3", s.builds)
	}
}

// A broken edit (invalid config, say) shows its error until the next change
// fixes it, rather than the last good build.
func TestPreviewHandler_ShowsTheBuildErrorUntilFixed(t *testing.T) {
	t.Parallel()
	s := newPreviewSite(t)
	h := s.handler(t)

	s.fail = errors.New("config: bad yaml")
	s.write(t, "index.md", "broken")
	rec := get(h)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "config: bad yaml") {
		t.Errorf("broken build: %d %q, want 500 naming the error", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != cacheRevalidate {
		t.Errorf("error Cache-Control = %q, want %q", cc, cacheRevalidate)
	}

	s.fail = nil
	s.write(t, "index.md", "fixed!")
	if body := get(h).Body.String(); body != "fixed!" {
		t.Errorf("after the fix: body %q, want %q", body, "fixed!")
	}
}

// The tree is checked on page loads only: the subresources a page pulls in read
// their files live, and would each repeat the walk.
func TestPreviewHandler_SubresourcesDoNotCheckTheTree(t *testing.T) {
	t.Parallel()
	s := newPreviewSite(t)
	h := s.handler(t)

	s.write(t, "index.md", "version two")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Sec-Fetch-Dest", "style")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Body.String() != "v1" || s.builds != 1 {
		t.Errorf("stylesheet fetch: body %q after %d builds, want the current build (v1) and no rebuild", rec.Body.String(), s.builds)
	}

	req.Header.Set("Sec-Fetch-Dest", "document")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Body.String() != "version two" {
		t.Errorf("page load: body %q, want %q", rec.Body.String(), "version two")
	}
}

// The browser must ask again on every load in dev mode: a cached page would
// hide the edit on link navigation and history, and a year-long asset cache
// hides theme edits.
func TestRevalidate(t *testing.T) {
	t.Parallel()
	h := Revalidate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWithETag(w, r, []byte("body"), mimePlain, cacheImmutable)
	}))

	rec := get(h)
	if cc := rec.Header().Get("Cache-Control"); cc != cacheRevalidate {
		t.Errorf("Cache-Control = %q, want %q", cc, cacheRevalidate)
	}

	// The ETag still answers a revalidation with 304.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec304 := httptest.NewRecorder()
	h.ServeHTTP(rec304, req)
	if rec304.Code != http.StatusNotModified {
		t.Errorf("revalidation: status %d, want 304", rec304.Code)
	}
}

func TestTreeStamp(t *testing.T) {
	t.Parallel()
	s := newPreviewSite(t)
	stamp := func() uint64 {
		t.Helper()
		v, err := TreeStamp(s.dir)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	base := stamp()
	if stamp() != base {
		t.Fatal("stamp differs between two reads of an unchanged tree")
	}
	s.write(t, ".git/index", "git's own bookkeeping")
	if stamp() != base {
		t.Error(".git changes the stamp; git's own writes are not site edits")
	}
	s.write(t, "index.md", "longer content")
	edited := stamp()
	if edited == base {
		t.Error("an edited file leaves the stamp unchanged")
	}
	if err := os.Remove(filepath.Join(s.dir, "index.md")); err != nil {
		t.Fatal(err)
	}
	if stamp() == edited {
		t.Error("a deleted file leaves the stamp unchanged")
	}
	if _, err := TreeStamp(filepath.Join(s.dir, "missing")); err == nil {
		t.Error("TreeStamp of a missing directory: want an error")
	}
}
