package server

import (
	"context"
	"encoding/xml"
	"maps"
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

	// A body cached for the process's lifetime must revalidate; it used to be
	// written with a bare Set+WriteHeader+Write (REVIEW.md §10.7).
	assertRevalidates(t, revalidationCase{
		Handler:   handler,
		Path:      "/sitemap.xml",
		WantType:  mimeXML,
		WantCache: cacheDynamic,
	})

	// The handler is a lazyBytes wrapper over GenerateSitemap, whose output
	// TestGenerateSitemap pins exhaustively; assert only that this response is that
	// document, for this site. Delimited with <loc></loc> because every URL in the
	// fixture is a prefix of another.
	body := w.Body.String()
	if !strings.Contains(body, "<urlset") {
		t.Error("response should contain <urlset>")
	}
	if !strings.Contains(body, "<loc>https://docs.example.com/</loc>") {
		t.Errorf("response missing the root URL\n%s", body)
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
	if !strings.HasPrefix(body, xml.Header) {
		t.Error("should start with the XML declaration")
	}

	var parsed urlSet
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, body)
	}
	if parsed.XMLNS != sitemapNS {
		t.Errorf("xmlns = %q, want %q", parsed.XMLNS, sitemapNS)
	}
	got := make(map[string]string, len(parsed.URLs))
	for _, u := range parsed.URLs {
		got[u.Loc] = u.LastMod
	}
	// Exhaustive rather than a Contains sweep. Note "/docs" has no trailing slash
	// while the tag index "/tags/" does — build mode writes docs/index.html and so
	// serves the former at "/docs/". Tracked in REVIEW.md §10.2.
	want := map[string]string{
		"https://docs.example.com/":              "2025-06-15",
		"https://docs.example.com/docs":          "2025-06-15",
		"https://docs.example.com/docs/guide.md": "2025-06-15",
		"https://docs.example.com/tags/":         "", // tag index, no lastmod
		"https://docs.example.com/tags/tutorial": "",
	}
	if !maps.Equal(got, want) {
		t.Errorf("urls = %v, want %v", got, want)
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

	var parsed urlSet
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid XML: %v", err)
	}
	if len(parsed.URLs) != 0 {
		t.Errorf("urls = %v, want none when domain is empty", parsed.URLs)
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

	// Pages without noindex should be present. Delimited: the root URL is a prefix
	// of every other entry, so an undelimited Contains can never fail.
	if !strings.Contains(body, "<loc>https://docs.example.com/docs/guide.md</loc>") {
		t.Errorf("should contain guide.md (no robots directive)\n%s", body)
	}
	if !strings.Contains(body, "<loc>https://docs.example.com/</loc>") {
		t.Errorf("should contain root URL (no robots directive)\n%s", body)
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
	resolver := resolve.Build(sitemapTestFS, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: hasRenderer})

	data, err := GenerateSitemap(context.Background(), idx, "https://docs.example.com", "README.md", prov, resolver, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(data)

	// Delimited: the extensionless URL is a prefix of the .md one, and the root URL
	// is a prefix of both, so undelimited Contains checks here cannot fail.
	if !strings.Contains(body, "<loc>https://docs.example.com/docs/guide</loc>") {
		t.Errorf("should contain extensionless URL docs/guide\n%s", body)
	}
	if strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("should not contain .md URL when resolver provides clean path")
	}
	if !strings.Contains(body, "<loc>https://docs.example.com/</loc>") {
		t.Errorf("should contain root URL from README.md\n%s", body)
	}

	// Default index files fold to their directory URL, not the extensionless name.
	if strings.Contains(body, "https://docs.example.com/README") {
		t.Error("root README.md should map to / not /README")
	}
	if strings.Contains(body, "https://docs.example.com/docs/README") {
		t.Error("docs/README.md should map to /docs not /docs/README")
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
	resolver := resolve.Build(files, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: hasRenderer})

	xmlData, err := GenerateSitemap(context.Background(), idx, "https://example.com", "README.md", prov, resolver, "")
	if err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	body := string(xmlData)
	for _, want := range []string{
		"<loc>https://example.com/tags/</loc>",
		"<loc>https://example.com/tags/go</loc>",
		"<loc>https://example.com/tags/docs</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q\n%s", want, body)
		}
	}
}

func TestGenerateSitemapIndex(t *testing.T) {
	t.Parallel()

	data, err := GenerateSitemapIndex("example.com", []string{"fr", "de"})
	if err != nil {
		t.Fatalf("GenerateSitemapIndex: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		xml.Header,
		"<sitemapindex",
		"<loc>https://example.com/sitemap.xml</loc>",
		"<loc>https://example.com/fr/sitemap.xml</loc>",
		"<loc>https://example.com/de/sitemap.xml</loc>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sitemap-index missing %q\ngot:\n%s", want, out)
		}
	}

	// Must be well-formed XML (the whole point of using xml.Marshal).
	var parsed sitemapIndex
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid XML: %v", err)
	}
	if len(parsed.Sitemaps) != 3 {
		t.Errorf("got %d sitemap entries, want 3", len(parsed.Sitemaps))
	}
}
