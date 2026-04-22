package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

var sitemapTestTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

var sitemapTestFS = fstest.MapFS{
	"README.md":      {Data: []byte("---\ntitle: Home\ndescription: Welcome\n---\n# Home\n"), ModTime: sitemapTestTime},
	"docs/guide.md":  {Data: []byte("---\ntitle: Guide\ntags: [tutorial]\n---\n# Guide\n"), ModTime: sitemapTestTime},
	"docs/README.md": {Data: []byte("---\ntitle: Docs Index\n---\n# Docs\n"), ModTime: sitemapTestTime},
}

func TestSitemapHandler(t *testing.T) {
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)
	handler := NewSitemapHandler(idx, "https://docs.example.com", "README.md", prov)

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
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)
	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", prov)
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
	idx := buildTestIndex(t, sitemapTestFS)
	prov := newMemoryProvider(sitemapTestFS, "README.md", false)
	data, err := GenerateSitemap(context.Background(), idx, "", "README.md", prov)
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(data)
	if strings.Contains(body, "<loc>") {
		t.Error("should not contain any <loc> entries when domain is empty")
	}
}

func TestGenerateSitemap_NilProvider(t *testing.T) {
	idx := buildTestIndex(t, sitemapTestFS)
	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", nil)
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
