package server

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/resolve"
)

func TestBuildRedirectMap_RedirectFrom(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"docs/new-page.md": {Data: []byte("---\ntitle: New Page\nredirect_from:\n  - /old-page\n  - /legacy/page\n---\n# New Page\n")},
	}

	idx := buildTestIndex(t, files)
	redirects, _ := BuildRedirectMap(idx, nil, "")

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
	redirects, _ := BuildRedirectMap(idx, nil, "")

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

func TestBuildRedirectMap_BasePath(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"guide.md": {Data: []byte("---\ntitle: Guide\nredirect_from:\n  - /old-guide\n---\n# Guide\n")},
	}
	idx := buildTestIndex(t, files)
	resolver := resolve.Build(files, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: func(string) bool { return true }})

	redirects, _ := BuildRedirectMap(idx, resolver, "/fr-FR")

	// The source stays content-root-relative — stripPathPrefix removes /fr-FR
	// before the handler looks the request path up — while the target must be an
	// absolute site path, or the browser lands on the default language's page.
	want := URLRedirectMap{"/old-guide": "/fr-FR/guide"}
	if !maps.Equal(redirects, want) {
		t.Errorf("redirects = %v, want %v", redirects, want)
	}
}

func TestBuildRedirectMap_Empty(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"page.md": {Data: []byte("---\ntitle: Page\n---\n# Page\n")},
	}

	idx := buildTestIndex(t, files)
	redirects, _ := BuildRedirectMap(idx, nil, "")

	if redirects != nil {
		t.Errorf("expected nil redirect map, got %v", redirects)
	}
}

func TestBuildRedirectMap_NilIndex(t *testing.T) {
	t.Parallel()

	redirects, _ := BuildRedirectMap(nil, nil, "")
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

	// Verify XSS is prevented by HTML escaping.
	xss := string(GenerateRedirectHTML(`"><script>alert(1)</script>`))
	if strings.Contains(xss, "<script>") {
		t.Error("GenerateRedirectHTML should escape HTML in target URL")
	}
	if !strings.Contains(xss, "&lt;script&gt;") {
		t.Error("GenerateRedirectHTML should contain escaped script tag")
	}
}

func TestExtensionRedirect(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"guide.md":      &fstest.MapFile{Data: []byte("# Guide")},
		"docs/intro.md": &fstest.MapFile{Data: []byte("# Intro")},
		"image.jpg":     &fstest.MapFile{Data: []byte{0xFF, 0xD8}},
	}

	resolver := resolve.Build(files, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: func(mimeType string) bool {
		return mimeType == "text/markdown"
	}})

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

	middleware := ExtensionRedirect(resolver, []string{".md"}, "")
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

	middleware := ExtensionRedirect(nil, []string{".md"}, "")
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
	resolver := resolve.Build(files, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: func(string) bool { return true }})

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ExtensionRedirect(resolver, nil, "")
	handler := middleware(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/guide.md", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (empty strip exts should pass through)", w.Code, http.StatusOK)
	}
}

// redirectMap drops BuildRedirectMap's findings where a test only needs the map.
func redirectMap(m URLRedirectMap, _ []diag.Finding) URLRedirectMap { return m }

func TestBuildRedirectMap_Findings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   fstest.MapFS
		want    []string // "code file key"
		wantMap URLRedirectMap
	}{
		{"string instead of list", fstest.MapFS{
			"a.md": {Data: []byte("---\nredirect_from: /old\n---\n# A\n")},
		}, []string{"content.frontmatter-type a.md redirect_from"}, nil},
		{"non-string item", fstest.MapFS{
			"a.md": {Data: []byte("---\nredirect_from: [/old, 3]\n---\n# A\n")},
		}, []string{"content.frontmatter-type a.md redirect_from"}, URLRedirectMap{"/old": "/a.md"}},
		{"two pages claim one source", fstest.MapFS{
			"a.md": {Data: []byte("---\nredirect_from: [/old]\n---\n# A\n")},
			"b.md": {Data: []byte("---\nredirect_from: [/old]\n---\n# B\n")},
		}, []string{"content.redirect-conflict b.md /old"}, URLRedirectMap{"/old": "/b.md"}},
		{"source is a real page", fstest.MapFS{
			"a.md": {Data: []byte("---\nredirect_from: [/b.md]\n---\n# A\n")},
			"b.md": {Data: []byte("---\ntitle: B\n---\n# B\n")}, // indexed: no resolver in this table
		}, []string{"content.redirect-conflict a.md /b.md"}, URLRedirectMap{"/b.md": "/a.md"}},
		{"clean", fstest.MapFS{
			"a.md": {Data: []byte("---\nredirect_from: [/old]\n---\n# A\n")},
		}, nil, URLRedirectMap{"/old": "/a.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m, findings := BuildRedirectMap(buildTestIndex(t, tt.files), nil, "")
			var got []string
			for _, f := range findings {
				got = append(got, f.Code+" "+f.File+" "+f.Key)
				if f.Message == "" || f.Fix == "" {
					t.Errorf("finding without message or fix: %+v", f)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("findings = %q, want %q", got, tt.want)
			}
			// The map itself is what it always was.
			if !maps.Equal(m, tt.wantMap) {
				t.Errorf("map = %v, want %v", m, tt.wantMap)
			}
		})
	}
}

// TestBuildRedirectMap_SourceIsCleanURL: with a resolver, a source naming a
// page's clean URL is a conflict too.
func TestBuildRedirectMap_SourceIsCleanURL(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{
		"a.md": {Data: []byte("---\nredirect_from: [/b]\n---\n# A\n")},
		"b.md": {Data: []byte("# B\n")},
	}
	resolver := resolve.Build(files, resolve.BuildOptions{
		StripExtensions: []string{".md"},
		HasRenderer:     func(string) bool { return true },
	})
	_, findings := BuildRedirectMap(buildTestIndex(t, files), resolver, "")
	if len(findings) != 1 || findings[0].Code != "content.redirect-conflict" || findings[0].Key != "/b" {
		t.Errorf("findings = %+v", findings)
	}
}
