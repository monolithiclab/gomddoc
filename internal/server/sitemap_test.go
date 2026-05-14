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

var sitemapTestTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

var sitemapTestFS = fstest.MapFS{
	"README.md":      {Data: []byte("---\ntitle: Home\ndescription: Welcome\n---\n# Home\n"), ModTime: sitemapTestTime},
	"docs/guide.md":  {Data: []byte("---\ntitle: Guide\ntags: [tutorial]\n---\n# Guide\n"), ModTime: sitemapTestTime},
	"docs/README.md": {Data: []byte("---\ntitle: Docs Index\n---\n# Docs\n"), ModTime: sitemapTestTime},
}

var sitemapNoindexFS = fstest.MapFS{
	"README.md":      {Data: []byte("---\ntitle: Home\n---\n# Home\n"), ModTime: sitemapTestTime},
	"docs/guide.md":  {Data: []byte("---\ntitle: Guide\n---\n# Guide\n"), ModTime: sitemapTestTime},
	"docs/draft.md":  {Data: []byte("---\ntitle: Draft\nrobots: noindex\n---\n# Draft\n"), ModTime: sitemapTestTime},
	"docs/hidden.md": {Data: []byte("---\ntitle: Hidden\nrobots: \"noindex, nofollow\"\n---\n# Hidden\n"), ModTime: sitemapTestTime},
}

func TestSitemapHandler(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)
	handler := NewSitemapHandler(idx, "https://docs.example.com", "README.md", prov, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("Content-Type = %q, want application/xml", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<urlset") {
		t.Error("response should contain <urlset>")
	}
	if !strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("response should contain guide.md URL")
	}
	// README.md paths should be normalized (stripped)
	if !strings.Contains(body, "https://docs.example.com/") {
		t.Error("response should contain root URL (README.md stripped)")
	}
	if !strings.Contains(body, "https://docs.example.com/docs/") {
		t.Error("response should contain docs/ URL (README.md stripped)")
	}
	if !strings.Contains(body, "<lastmod>2025-06-15</lastmod>") {
		t.Error("response should contain <lastmod> dates")
	}
}

func TestGenerateSitemap(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)
	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", prov, nil, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(data)
	if !strings.Contains(body, "<?xml version=") {
		t.Error("should contain XML declaration")
	}
	if !strings.Contains(body, "http://www.sitemaps.org/schemas/sitemap/0.9") {
		t.Error("should contain sitemap namespace")
	}
	if !strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("should contain guide.md URL")
	}
	if !strings.Contains(body, "<lastmod>2025-06-15</lastmod>") {
		t.Error("should contain <lastmod> with file modification date")
	}
}

func TestGenerateSitemap_EmptyDomain(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)
	data, err := GenerateSitemap(context.Background(), idx, "", "README.md", prov, nil, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(data)
	if strings.Contains(body, "<loc>") {
		t.Error("should not contain any <loc> entries when domain is empty")
	}
}

func TestGenerateSitemap_ExcludesNoindex(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(t, sitemapNoindexFS)
	prov := newMemoryProvider(sitemapNoindexFS, "README.md", false)
	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", prov, nil, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(data)

	// Pages without noindex should be present
	if !strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("should contain guide.md (no robots directive)")
	}
	if !strings.Contains(body, "https://docs.example.com/") {
		t.Error("should contain root URL (no robots directive)")
	}

	// Pages with noindex should be excluded
	if strings.Contains(body, "draft.md") {
		t.Error("should not contain draft.md (robots: noindex)")
	}
	if strings.Contains(body, "hidden.md") {
		t.Error("should not contain hidden.md (robots: noindex, nofollow)")
	}
}

func TestGenerateSitemap_NilProvider(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(t, sitemapTestFS)
	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", nil, nil, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(data)
	if strings.Contains(body, "<lastmod>") {
		t.Error("should not contain <lastmod> when provider is nil")
	}
	if !strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("should still contain URLs")
	}
}

func TestGenerateSitemap_WithResolver(t *testing.T) {
	t.Parallel()
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)

	// Build a resolver that strips .md extensions
	hasRenderer := func(mimeType string) bool {
		return mimeType == "text/markdown"
	}
	resolver := resolve.Build(sitemapTestFS, []string{".md"}, hasRenderer)

	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", prov, resolver, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
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
	if !strings.Contains(body, "https://docs.example.com/") {
		t.Error("should contain root URL from README.md")
	}
}

func TestGenerateSitemap_IncludesTagPages(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{
		"a.md": {Data: []byte("---\ntitle: A\ntags: [go, docs]\n---\n# A")},
		"b.md": {Data: []byte("---\ntitle: B\ntags: [docs]\n---\n# B")},
	}
	idx := buildTestIndex(t, files)

	prov := newMemoryProvider(files, "README.md", false)
	hasRenderer := func(mimeType string) bool {
		return mimeType == "text/markdown"
	}
	resolver := resolve.Build(files, []string{".md"}, hasRenderer)

	xml, err := GenerateSitemap(context.Background(), idx, "https://example.com", "README.md", prov, resolver, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(xml)
	for _, want := range []string{
		"https://example.com/tags/",
		"https://example.com/tags/go",
		"https://example.com/tags/docs",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q\n%s", want, body)
		}
	}
}
