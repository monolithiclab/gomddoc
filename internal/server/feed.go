package server

import (
	"context"
	"encoding/xml"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/seo"
)

const feedMaxEntries = 20

// FeedHandler serves an Atom feed from the metadata index.
// The feed is generated once on first request and cached, since
// the metadata index is immutable after construction.
type FeedHandler struct {
	index        *metadata.Index
	domain       string
	defaultIndex string
	provider     provider.Provider
	siteTitle    string
	resolver     *resolve.PathResolver

	mu     sync.Mutex
	cached []byte
}

// NewFeedHandler creates a new FeedHandler.
func NewFeedHandler(index *metadata.Index, domain, defaultIndex string, prov provider.Provider, siteTitle string, resolver *resolve.PathResolver) *FeedHandler {
	return &FeedHandler{
		index:        index,
		domain:       domain,
		defaultIndex: defaultIndex,
		provider:     prov,
		siteTitle:    siteTitle,
		resolver:     resolver,
	}
}

// ServeHTTP writes the Atom feed response.
func (h *FeedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := h.getOrGenerate()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// getOrGenerate returns cached feed XML, generating it on first successful
// call. Unlike sync.Once, subsequent requests retry if generation failed.
func (h *FeedHandler) getOrGenerate() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cached != nil {
		return h.cached, nil
	}

	out, err := GenerateFeed(context.Background(), h.index, h.domain, h.defaultIndex, h.provider, h.siteTitle, h.resolver)
	if err != nil {
		slog.Error("Failed to generate feed", slog.Any("error", err))
		return nil, err
	}
	h.cached = out
	return h.cached, nil
}

// atomFeed is the root element of an Atom feed.
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

// atomLink represents an Atom link element.
type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr,omitempty"`
}

// atomEntry represents a single entry in the Atom feed.
type atomEntry struct {
	Title   string `xml:"title"`
	ID      string `xml:"id"`
	Updated string `xml:"updated"`
	Link    atomLink
	Summary string `xml:"summary,omitempty"`
}

// GenerateFeed produces the Atom feed XML bytes.
// When prov is non-nil, each entry includes an updated time from the file's modification time.
// When resolver is non-nil, URLs use extensionless paths.
func GenerateFeed(ctx context.Context, index *metadata.Index, domain, defaultIndex string, prov provider.Provider, siteTitle string, resolver *resolve.PathResolver) ([]byte, error) {
	var contentRoot fs.FS
	if prov != nil {
		if root, err := prov.RootFS(ctx); err == nil {
			contentRoot = root
		}
	}

	pages := index.AllPages()

	// Collect entries with modification times for sorting.
	type pageWithTime struct {
		page    metadata.PageInfo
		modTime time.Time
	}

	var candidates []pageWithTime
	for _, page := range pages {
		if robots, ok := page.Meta["robots"].(string); ok && strings.Contains(robots, "noindex") {
			continue
		}

		var modTime time.Time
		if contentRoot != nil {
			statPath := strings.TrimPrefix(page.Path, "/")
			if info, err := fs.Stat(contentRoot, statPath); err == nil {
				modTime = info.ModTime().UTC()
			}
		}
		if modTime.IsZero() && !page.Date.IsZero() {
			modTime = page.Date
		}

		candidates = append(candidates, pageWithTime{page: page, modTime: modTime})
	}

	// Sort by modification time descending (newest first).
	slices.SortFunc(candidates, func(a, b pageWithTime) int {
		return -a.modTime.Compare(b.modTime)
	})

	// Limit to feedMaxEntries.
	if len(candidates) > feedMaxEntries {
		candidates = candidates[:feedMaxEntries]
	}

	// Build Atom entries.
	entries := make([]atomEntry, 0, len(candidates))
	for _, c := range candidates {
		pagePath := resolvedPagePath(c.page.Path, defaultIndex, resolver)

		loc := seo.PageURL(domain, pagePath, defaultIndex)
		if loc == "" {
			continue
		}

		entry := atomEntry{
			Title: c.page.Title,
			ID:    loc,
			Link:  atomLink{Rel: "alternate", Href: loc},
		}

		if !c.modTime.IsZero() {
			entry.Updated = c.modTime.Format(time.RFC3339)
		}
		if c.page.Description != "" {
			entry.Summary = c.page.Description
		}

		entries = append(entries, entry)
	}

	// Determine feed-level updated time.
	feedUpdated := time.Now().UTC().Format(time.RFC3339)
	if len(candidates) > 0 && !candidates[0].modTime.IsZero() {
		feedUpdated = candidates[0].modTime.Format(time.RFC3339)
	}

	selfURL := seo.PageURL(domain, "/feed.xml", "")
	altURL := seo.PageURL(domain, "/", "")

	feed := atomFeed{
		XMLNS:   "http://www.w3.org/2005/Atom",
		Title:   siteTitle,
		ID:      altURL,
		Updated: feedUpdated,
		Links: []atomLink{
			{Rel: "self", Href: selfURL, Type: "application/atom+xml"},
			{Rel: "alternate", Href: altURL},
		},
		Entries: entries,
	}

	out := []byte(xml.Header)
	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append(out, body...)
	out = append(out, '\n')
	return out, nil
}
