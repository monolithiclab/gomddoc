package server

import (
	"context"
	"encoding/xml"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/seo"
)

// SitemapHandler serves an XML sitemap from the metadata index.
// The sitemap is generated once on first request and cached, since
// the metadata index is immutable after construction.
type SitemapHandler struct {
	index        *metadata.Index
	domain       string
	defaultIndex string
	provider     provider.Provider

	once   sync.Once
	cached []byte
}

// NewSitemapHandler creates a new SitemapHandler.
func NewSitemapHandler(index *metadata.Index, domain, defaultIndex string, prov provider.Provider) *SitemapHandler {
	return &SitemapHandler{
		index:        index,
		domain:       domain,
		defaultIndex: defaultIndex,
		provider:     prov,
	}
}

// urlSet is the root element of a sitemap XML document.
type urlSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// sitemapURL represents a single URL entry in the sitemap.
type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// ServeHTTP writes the sitemap XML response.
func (h *SitemapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.once.Do(func() {
		out, err := GenerateSitemap(r.Context(), h.index, h.domain, h.defaultIndex, h.provider)
		if err == nil {
			h.cached = out
		}
	})

	if h.cached == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.cached)
}

// GenerateSitemap produces the sitemap XML bytes.
// When prov is non-nil, each entry includes a <lastmod> from the file's modification time.
func GenerateSitemap(ctx context.Context, index *metadata.Index, domain, defaultIndex string, prov provider.Provider) ([]byte, error) {
	var contentRoot fs.FS
	if prov != nil {
		if root, err := prov.RootFS(ctx); err == nil {
			contentRoot = root
		}
	}

	pages := index.AllPages()

	urls := make([]sitemapURL, 0, len(pages))
	for _, page := range pages {
		if robots, ok := page.Meta["robots"].(string); ok && strings.Contains(robots, "noindex") {
			continue
		}
		loc := seo.PageURL(domain, "/"+page.Path, defaultIndex)
		if loc != "" {
			entry := sitemapURL{Loc: loc}
			if contentRoot != nil {
				statPath := strings.TrimPrefix(page.Path, "/")
				if info, err := fs.Stat(contentRoot, statPath); err == nil {
					entry.LastMod = info.ModTime().UTC().Format("2006-01-02")
				}
			}
			urls = append(urls, entry)
		}
	}

	sitemap := urlSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	out := []byte(xml.Header)
	body, err := xml.MarshalIndent(sitemap, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append(out, body...)
	out = append(out, '\n')
	return out, nil
}
