package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/resolve"
)

func TestBuildRedirectMap_RedirectFrom(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"docs/new-page.md": {Data: []byte("---\ntitle: New Page\nredirect_from:\n  - /old-page\n  - /legacy/page\n---\n# New Page\n")},
	}

	idx := buildTestIndex(t, files)
	redirects := BuildRedirectMap(idx)

	if redirects == nil {
		t.Fatal("expected non-nil redirect map")
	}

	tests := []struct {
		source string
		target string
	}{
		{"/old-page", "/docs/new-page.md"},
		{"/legacy/page", "/docs/new-page.md"},
	}

	for _, tc := range tests {
		got, ok := redirects[tc.source]
		if !ok {
			t.Errorf("redirect %q not found", tc.source)
			continue
		}
		if got != tc.target {
			t.Errorf("redirect %q = %q, want %q", tc.source, got, tc.target)
		}
	}
}

func TestBuildRedirectMap_MultiplePages(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"page-a.md": {Data: []byte("---\ntitle: A\nredirect_from:\n  - /old-a\n---\n# A\n")},
		"page-b.md": {Data: []byte("---\ntitle: B\nredirect_from:\n  - /old-b\n---\n# B\n")},
	}

	idx := buildTestIndex(t, files)
	redirects := BuildRedirectMap(idx)

	if redirects == nil {
		t.Fatal("expected non-nil redirect map")
	}
	if len(redirects) != 2 {
		t.Errorf("len = %d, want 2", len(redirects))
	}

	if _, ok := redirects["/old-a"]; !ok {
		t.Error("redirect /old-a not found")
	}
	if _, ok := redirects["/old-b"]; !ok {
		t.Error("redirect /old-b not found")
	}
}

func TestBuildRedirectMap_Empty(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"page.md": {Data: []byte("---\ntitle: Page\n---\n# Page\n")},
	}

	idx := buildTestIndex(t, files)
	redirects := BuildRedirectMap(idx)

	if redirects != nil {
		t.Errorf("expected nil redirect map, got %v", redirects)
	}
}

func TestBuildRedirectMap_NilIndex(t *testing.T) {
	t.Parallel()

	redirects := BuildRedirectMap(nil)
	if redirects != nil {
		t.Errorf("expected nil redirect map for nil index")
	}
}

func TestGenerateRedirectHTML(t *testing.T) {
	t.Parallel()

	html := string(GenerateRedirectHTML("/docs/new-page"))

	checks := []struct {
		name     string
		contains string
	}{
		{"doctype", "<!DOCTYPE html>"},
		{"meta refresh", `content="0; url=/docs/new-page"`},
		{"canonical", `rel="canonical" href="/docs/new-page"`},
		{"fallback link", `href="/docs/new-page">/docs/new-page</a>`},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.contains) {
			t.Errorf("%s: should contain %q", c.name, c.contains)
		}
	}
}

func TestExtensionRedirect(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"guide.md":      &fstest.MapFile{Data: []byte("# Guide")},
		"docs/intro.md": &fstest.MapFile{Data: []byte("# Intro")},
		"image.jpg":     &fstest.MapFile{Data: []byte{0xFF, 0xD8}},
	}

	resolver := resolve.Build(files, []string{".md"}, func(mimeType string) bool {
		return mimeType == "text/markdown"
	})

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantLoc    string
	}{
		{
			name:       "md extension redirects to clean URL",
			path:       "/guide.md",
			wantStatus: http.StatusMovedPermanently,
			wantLoc:    "/guide",
		},
		{
			name:       "nested md extension redirects",
			path:       "/docs/intro.md",
			wantStatus: http.StatusMovedPermanently,
			wantLoc:    "/docs/intro",
		},
		{
			name:       "no extension passes through",
			path:       "/guide",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-stripped extension passes through",
			path:       "/image.jpg",
			wantStatus: http.StatusOK,
		},
		{
			name:       "directory request passes through",
			path:       "/docs/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "root request passes through",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown md file passes through",
			path:       "/nonexistent.md",
			wantStatus: http.StatusOK,
		},
	}

	middleware := ExtensionRedirect(resolver, []string{".md"})
	handler := middleware(okHandler)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantLoc != "" {
				if loc := w.Header().Get("Location"); loc != tt.wantLoc {
					t.Errorf("Location = %q, want %q", loc, tt.wantLoc)
				}
			}
		})
	}
}

func TestExtensionRedirect_NilResolver(t *testing.T) {
	t.Parallel()

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ExtensionRedirect(nil, []string{".md"})
	handler := middleware(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/guide.md", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (nil resolver should pass through)", w.Code, http.StatusOK)
	}
}

func TestExtensionRedirect_EmptyStripExts(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"guide.md": &fstest.MapFile{Data: []byte("# Guide")},
	}
	resolver := resolve.Build(files, []string{".md"}, func(string) bool { return true })

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ExtensionRedirect(resolver, nil)
	handler := middleware(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/guide.md", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (empty strip exts should pass through)", w.Code, http.StatusOK)
	}
}
