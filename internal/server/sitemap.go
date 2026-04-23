package server

import (
	"context"
	"encoding/xml"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/resolve"
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
	resolver     *resolve.PathResolver
	pathPrefix   string // e.g. "/fr-FR" for per-language sitemaps

	mu     sync.Mutex
	cached []byte
}

// NewSitemapHandler creates a new SitemapHandler.
// pathPrefix is prepended to all page paths (empty for default language).
func NewSitemapHandler(index *metadata.Index, domain, defaultIndex string, prov provider.Provider, resolver *resolve.PathResolver, pathPrefix string) *SitemapHandler {
	return &SitemapHandler{
		index:        index,
		domain:       domain,
		defaultIndex: defaultIndex,
		provider:     prov,
		resolver:     resolver,
		pathPrefix:   pathPrefix,
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
	data, err := h.getOrGenerate()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", mimeXML)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// getOrGenerate returns cached sitemap XML, generating it on first successful
// call. Unlike sync.Once, subsequent requests retry if generation failed.
func (h *SitemapHandler) getOrGenerate() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cached != nil {
		return h.cached, nil
	}

	out, err := GenerateSitemap(context.Background(), h.index, h.domain, h.defaultIndex, h.provider, h.resolver, h.pathPrefix)
	if err != nil {
		slog.Error("Failed to generate sitemap", slog.Any("error", err))
		return nil, err
	}
	h.cached = out
	return h.cached, nil
}

// GenerateSitemap produces the sitemap XML bytes.
// When prov is non-nil, each entry includes a <lastmod> from the file's modification time.
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

		pagePath := resolvedPagePath(page.Path, defaultIndex, resolver)

		loc := seo.PageURL(domain, pathPrefix+pagePath, defaultIndex)
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

// resolvedPagePath returns the URL path for a page, using the resolver for
// extensionless paths when available. Default index files are excluded from
// resolver lookup — their URL is the directory path, handled by seo.PageURL.
func resolvedPagePath(pagePath, defaultIndex string, resolver *resolve.PathResolver) string {
	if resolver == nil {
		return pagePath
	}
	lookupPath := strings.TrimPrefix(pagePath, "/")
	if IsDefaultIndex(lookupPath, defaultIndex) {
		return pagePath
	}
	if clean, found := resolver.CleanPath(lookupPath); found {
		return "/" + clean
	}
	return pagePath
}

// IsDefaultIndex reports whether filePath's basename matches defaultIndex.
func IsDefaultIndex(filePath, defaultIndex string) bool {
	return defaultIndex != "" && strings.EqualFold(path.Base(filePath), defaultIndex)
}
