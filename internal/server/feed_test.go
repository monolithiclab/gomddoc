package server

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"slices"
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

	if !strings.HasPrefix(string(data), xml.Header) {
		t.Error("output should start with the XML declaration")
	}

	// Unmarshal rather than substring-match: every URL in this fixture is a prefix
	// of another ("https://docs.example.com/" is a prefix of every other one), so
	// Contains checks pass on the wrong element.
	var feed atomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, data)
	}

	if feed.Title != "Test Site" {
		t.Errorf("feed title = %q, want %q", feed.Title, "Test Site")
	}
	if feed.XMLNS != atomNS {
		t.Errorf("xmlns = %q, want %q", feed.XMLNS, atomNS)
	}

	wantLinks := []atomLink{
		{Rel: "self", Href: "https://docs.example.com/feed.xml", Type: "application/atom+xml"},
		{Rel: "alternate", Href: "https://docs.example.com/"},
	}
	if !slices.Equal(feed.Links, wantLinks) {
		t.Errorf("feed links = %+v, want %+v", feed.Links, wantLinks)
	}

	// Newest first: README (14:00) > guide (-24h) > docs/README (-48h). The last
	// one is "/docs", not "/docs/" — a folded index URL carries no trailing slash
	// in serve mode, which does not match what build mode writes (REVIEW.md §10.2).
	wantIDs := []string{
		"https://docs.example.com/",
		"https://docs.example.com/docs/guide.md",
		"https://docs.example.com/docs",
	}
	gotIDs := make([]string, len(feed.Entries))
	for i, e := range feed.Entries {
		gotIDs[i] = e.ID
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("entry IDs = %v, want %v (newest first)", gotIDs, wantIDs)
	}

	root := feed.Entries[0]
	if root.Summary != "Welcome home" {
		t.Errorf("root summary = %q, want %q", root.Summary, "Welcome home")
	}
	if root.Updated != "2025-07-10T14:00:00Z" {
		t.Errorf("root updated = %q, want %q", root.Updated, "2025-07-10T14:00:00Z")
	}
	if root.Link.Href != root.ID {
		t.Errorf("root alternate link = %q, want %q", root.Link.Href, root.ID)
	}

	// Wire-format check the unmarshal above cannot make: without an explicit
	// xml:"link" tag encoding/xml falls back to the field name and emits <Link>,
	// which no Atom reader recognises — and a round-trip through the production
	// struct reads it straight back.
	if strings.Contains(string(data), "<Link") {
		t.Error("entries emit <Link>; Atom requires the lowercase <link> element")
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

	var feed atomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatalf("output is not valid XML: %v", err)
	}
	// Exact, not <=: a feed that emitted zero entries would also satisfy "at most
	// feedMaxEntries", which is the bug this test exists to catch.
	if len(feed.Entries) != feedMaxEntries {
		t.Errorf("entries = %d, want %d", len(feed.Entries), feedMaxEntries)
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

	var feed atomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatalf("output is not valid XML: %v", err)
	}
	if feed.Title != "Empty Site" {
		t.Errorf("feed title = %q, want %q", feed.Title, "Empty Site")
	}
	if len(feed.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(feed.Entries))
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

	assertRevalidates(t, revalidationCase{
		Handler:   handler,
		Path:      "/feed.xml",
		WantType:  mimeAtom,
		WantCache: cacheDynamic,
	})

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
	resolver := resolve.Build(feedTestFS, resolve.BuildOptions{StripExtensions: []string{".md"}, HasRenderer: hasRenderer})

	data, err := GenerateFeed(context.Background(), idx, "https://docs.example.com", "README.md", prov, "Test Site", resolver, "")
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}

	body := string(data)

	// Delimited: the extensionless URL is a prefix of the .md one, so an undelimited
	// Contains for the former is satisfied by the latter.
	if !strings.Contains(body, "<id>https://docs.example.com/docs/guide</id>") {
		t.Errorf("should contain extensionless URL docs/guide\n%s", body)
	}
	if strings.Contains(body, "https://docs.example.com/docs/guide.md") {
		t.Error("should not contain .md URL when resolver provides clean path")
	}

	// Default index files fold to their directory URL, not the extensionless name.
	if strings.Contains(body, "https://docs.example.com/README") {
		t.Error("root README.md should map to / not /README")
	}
	if strings.Contains(body, "https://docs.example.com/docs/README") {
		t.Error("docs/README.md should map to /docs not /docs/README")
	}
}
