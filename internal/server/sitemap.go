package server

import (
	"encoding/xml"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/seo"
)

// SitemapHandler serves an XML sitemap from the metadata index.
type SitemapHandler struct {
	index        *metadata.Index
	domain       string
	defaultIndex string
}

// NewSitemapHandler creates a new SitemapHandler.
func NewSitemapHandler(index *metadata.Index, domain, defaultIndex string) *SitemapHandler {
	return &SitemapHandler{
		index:        index,
		domain:       domain,
		defaultIndex: defaultIndex,
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
	Loc string `xml:"loc"`
}

// ServeHTTP writes the sitemap XML response.
func (h *SitemapHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	out, err := GenerateSitemap(h.index, h.domain, h.defaultIndex)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// GenerateSitemap produces the sitemap XML bytes for use in build mode.
func GenerateSitemap(index *metadata.Index, domain, defaultIndex string) ([]byte, error) {
	pages := index.AllPages()

	urls := make([]sitemapURL, 0, len(pages))
	for _, page := range pages {
		loc := seo.PageURL(domain, "/"+page.Path, defaultIndex)
		if loc != "" {
			urls = append(urls, sitemapURL{Loc: loc})
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
