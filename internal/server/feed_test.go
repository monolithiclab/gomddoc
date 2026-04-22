package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/monolithiclab/gomddoc/internal/resolve"
)

var feedTestTime = time.Date(2025, 7, 10, 14, 0, 0, 0, time.UTC)

var feedTestFS = fstest.MapFS{
	"README.md":      {Data: []byte("---\ntitle: Home\ndescription: Welcome home\n---\n# Home\n"), ModTime: feedTestTime},
	"docs/guide.md":  {Data: []byte("---\ntitle: Guide\ndescription: Getting started\n---\n# Guide\n"), ModTime: feedTestTime.Add(-24 * time.Hour)},
	"docs/README.md": {Data: []byte("---\ntitle: Docs Index\n---\n# Docs\n"), ModTime: feedTestTime.Add(-48 * time.Hour)},
}

func TestGenerateFeed(t *testing.T) {
	t.Parallel()

	idx := buildTestIndex(t, feedTestFS)
	prov := newMemoryProvider(feedTestFS, "README.md", false)

	data, err := GenerateFeed(context.Background(), idx, "https://docs.example.com", "README.md", prov, "Test Site", nil, "")
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	body := string(data)

	checks := []struct {
		name     string
		contains string
	}{
		{"xml declaration", "<?xml version="},
		{"atom namespace", "http://www.w3.org/2005/Atom"},
		{"feed title", "<title>Test Site</title>"},
		{"self link", `rel="self"`},
		{"alternate link", `rel="alternate"`},
		{"guide entry", "https://docs.example.com/docs/guide.md"},
		{"root entry", "https://docs.example.com/"},
		{"description as summary", "<summary>Welcome home</summary>"},
		{"updated time", "2025-07-10T14:00:00Z"},
	}

	for _, c := range checks {
		if !strings.Contains(body, c.contains) {
			t.Errorf("%s: response should contain %q", c.name, c.contains)
		}
	}
}

func TestGenerateFeed_ExcludesNoindex(t *testing.T) {
	t.Parallel()

	idx := buildTestIndex(t, sitemapNoindexFS)
	prov := newMemoryProvider(sitemapNoindexFS, "README.md", false)

	data, err := GenerateFeed(context.Background(), idx, "https://docs.example.com", "README.md", prov, "Test Site", nil, "")
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	body := string(data)

	if !strings.Contains(body, "guide.md") {
		t.Error("should contain guide.md (no robots directive)")
	}
	if strings.Contains(body, "draft.md") {
		t.Error("should not contain draft.md (robots: noindex)")
	}
	if strings.Contains(body, "hidden.md") {
		t.Error("should not contain hidden.md (robots: noindex, nofollow)")
	}
}

func TestGenerateFeed_LimitsEntries(t *testing.T) {
	t.Parallel()

	// Create more than feedMaxEntries files.
	files := make(fstest.MapFS)
	for i := range feedMaxEntries + 5 {
		name := "page" + strings.Repeat("x", i) + ".md"
		files[name] = &fstest.MapFile{
			Data:    []byte("---\ntitle: Page\n---\n# Page\n"),
			ModTime: feedTestTime.Add(time.Duration(-i) * time.Hour),
		}
	}

	idx := buildTestIndex(t, files)
	data, err := GenerateFeed(context.Background(), idx, "https://example.com", "README.md", nil, "Test", nil, "")
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	body := string(data)
	count := strings.Count(body, "<entry>")
	if count > feedMaxEntries {
		t.Errorf("entries = %d, want <= %d", count, feedMaxEntries)
	}
}

func TestGenerateFeed_EmptyIndex(t *testing.T) {
	t.Parallel()

	emptyFS := fstest.MapFS{}
	idx := buildTestIndex(t, emptyFS)

	data, err := GenerateFeed(context.Background(), idx, "https://example.com", "README.md", nil, "Empty Site", nil, "")
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	body := string(data)
	if !strings.Contains(body, "<title>Empty Site</title>") {
		t.Error("should contain feed title even with no entries")
	}
	if strings.Contains(body, "<entry>") {
		t.Error("should not contain any entries")
	}
}

func TestFeedHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	idx := buildTestIndex(t, feedTestFS)
	prov := newMemoryProvider(feedTestFS, "README.md", false)
	handler := NewFeedHandler(idx, "https://docs.example.com", "README.md", prov, "Test Site", nil, "")

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/atom+xml") {
		t.Errorf("Content-Type = %q, want application/atom+xml", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<feed") {
		t.Error("response should contain <feed>")
	}
}

func TestGenerateFeed_WithResolver(t *testing.T) {
	t.Parallel()

	idx := buildTestIndex(t, feedTestFS)
	prov := newMemoryProvider(feedTestFS, "README.md", false)

	// Build a resolver that strips .md extensions
	hasRenderer := func(mimeType string) bool {
		return mimeType == "text/markdown"
	}
	resolver := resolve.Build(feedTestFS, []string{".md"}, hasRenderer)

	data, err := GenerateFeed(context.Background(), idx, "https://docs.example.com", "README.md", prov, "Test Site", resolver, "")
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	body := string(data)

	// Should contain extensionless URL for guide.md
	if !strings.Contains(body, "https://docs.example.com/docs/guide") {
		t.Error("should contain extensionless URL docs/guide")
	}

	// Should NOT contain .md URL
	if strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("should not contain .md URL when resolver provides clean path")
	}

	// Default index files should resolve to directory URL, not extensionless
	if strings.Contains(body, "https://docs.example.com/README") {
		t.Error("root README.md should map to / not /README")
	}
	if strings.Contains(body, "https://docs.example.com/docs/README") {
		t.Error("docs/README.md should map to /docs/ not /docs/README")
	}
}
