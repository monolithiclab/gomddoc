package server

import (
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/seo"
)

// RobotsHandler serves a robots.txt response.
type RobotsHandler struct {
	cached []byte
}

// NewRobotsHandler creates a new RobotsHandler. sitemapPath is the sitemap the
// caller publishes, or "" if it publishes none; see GenerateRobotsTxt.
func NewRobotsHandler(domain, sitemapPath string) *RobotsHandler {
	return &RobotsHandler{cached: []byte(GenerateRobotsTxt(domain, sitemapPath))}
}

// ServeHTTP writes the robots.txt response.
func (h *RobotsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serveWithETag(w, r, h.cached, mimePlain, cacheDynamic)
}

// GenerateRobotsTxt produces the robots.txt content string, including a Sitemap
// directive naming sitemapPath when both it and domain are non-empty.
//
// sitemapPath is the sitemap the caller actually published — the path the route
// it registered answers on, or the file it just wrote — never a literal chosen
// here. A directive naming a URL nobody serves is a 404 on the one URL
// robots.txt hands a crawler, and naming /sitemap.xml when a build published a
// sitemap-index.xml sends crawlers to the default-language sitemap, which by
// design lists no translated page. Empty means the caller publishes no sitemap
// and the directive is omitted.
func GenerateRobotsTxt(domain, sitemapPath string) string {
	txt := "User-agent: *\nAllow: /\nDisallow: /_assets/\nDisallow: /api/\nDisallow: /debug/\n"
	if sitemapPath != "" {
		if sitemapURL := seo.PageURL(domain, sitemapPath, ""); sitemapURL != "" {
			txt += "\nSitemap: " + sitemapURL + "\n"
		}
	}
	return txt
}
