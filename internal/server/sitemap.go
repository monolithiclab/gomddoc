package server

import (
	"context"
	"encoding/xml"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/seo"
)

// SitemapHandler serves an XML sitemap from the metadata index.
// The sitemap is generated once on first request and cached, since
// the metadata index is immutable after construction.
type SitemapHandler struct {
	body *lazyBytes
}

// NewSitemapHandler creates a new SitemapHandler.
// pathPrefix is prepended to all page paths (empty for default language).
func NewSitemapHandler(index *metadata.Index, domain, defaultIndex string, prov provider.Provider, resolver *resolve.PathResolver, pathPrefix string) *SitemapHandler {
	return &SitemapHandler{
		body: newLazyBytes("sitemap", func() ([]byte, error) {
			return GenerateSitemap(context.Background(), index, domain, defaultIndex, prov, resolver, pathPrefix)
		}),
	}
}

// sitemapNS is the sitemap protocol 0.9 namespace URI.
const sitemapNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

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

// sitemapIndex is the root element of a sitemap index document, referencing
// per-language sitemaps.
type sitemapIndex struct {
	XMLName  xml.Name          `xml:"sitemapindex"`
	XMLNS    string            `xml:"xmlns,attr"`
	Sitemaps []sitemapIndexLoc `xml:"sitemap"`
}

// sitemapIndexLoc is a single <sitemap> entry in a sitemap index.
type sitemapIndexLoc struct {
	Loc string `xml:"loc"`
}

// GenerateSitemapIndex builds a sitemap-index.xml referencing the default-
// language sitemap plus one per additional language. URLs are built via
// seo.PageURL for consistent scheme handling, and the document is produced with
// xml.Marshal (never string concatenation) so special characters are escaped.
func GenerateSitemapIndex(domain string, langs []string) ([]byte, error) {
	entries := make([]sitemapIndexLoc, 0, len(langs)+1)
	entries = append(entries, sitemapIndexLoc{Loc: seo.PageURL(domain, "/sitemap.xml", "")})
	for _, lang := range langs {
		entries = append(entries, sitemapIndexLoc{Loc: seo.PageURL(domain, "/"+lang+"/sitemap.xml", "")})
	}

	doc := sitemapIndex{
		XMLNS:    sitemapNS,
		Sitemaps: entries,
	}

	out := []byte(xml.Header)
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append(out, body...)
	out = append(out, '\n')
	return out, nil
}

// ServeHTTP writes the sitemap XML response.
func (h *SitemapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.body.serve(w, r, mimeXML)
}

// GenerateSitemap produces the sitemap XML bytes.
// When prov is non-nil, each entry includes a <lastmod> from seo.LastModified —
// the file's modification time, or the frontmatter date when it is unavailable.
// When resolver is non-nil, URLs use extensionless paths.
// pathPrefix is prepended to all page paths (e.g. "/fr-FR" for per-language sitemaps).
func GenerateSitemap(ctx context.Context, index *metadata.Index, domain, defaultIndex string, prov provider.Provider, resolver *resolve.PathResolver, pathPrefix string) ([]byte, error) {
	var contentRoot fs.FS
	if prov != nil {
		if root, err := prov.RootFS(ctx); err == nil {
			contentRoot = root
		}
	}

	pages := index.AllPages()
	tags := index.AllTags()

	urls := make([]sitemapURL, 0, len(pages)+1+len(tags))
	for _, page := range pages {
		if robots, ok := page.Meta["robots"].(string); ok && strings.Contains(robots, "noindex") {
			continue
		}

		pagePath := resolver.PageURLPath(page.Path, defaultIndex)

		loc := seo.PageURL(domain, pathPrefix+pagePath, "")
		if loc != "" {
			entry := sitemapURL{Loc: loc}
			if lastMod := seo.LastModified(seo.StatModTime(contentRoot, page.Path), page.Date); !lastMod.IsZero() {
				entry.LastMod = lastMod.Format(time.DateOnly)
			}
			urls = append(urls, entry)
		}
	}

	// Tag index page.
	indexURL := seo.PageURL(domain, pathPrefix+"/tags/", "")
	if indexURL != "" {
		urls = append(urls, sitemapURL{Loc: indexURL})
	}

	// One URL per unique tag.
	for _, tag := range tags {
		tagURL := seo.PageURL(domain, pathPrefix+"/tags/"+url.PathEscape(tag), "")
		if tagURL != "" {
			urls = append(urls, sitemapURL{Loc: tagURL})
		}
	}

	sitemap := urlSet{
		XMLNS: sitemapNS,
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
