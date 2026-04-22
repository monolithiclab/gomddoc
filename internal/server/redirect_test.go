package server

import (
	"strings"
	"testing"
	"testing/fstest"
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
