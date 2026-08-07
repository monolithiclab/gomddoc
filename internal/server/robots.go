package server

import (
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/seo"
)

// RobotsHandler serves a robots.txt response.
type RobotsHandler struct {
	cached []byte
}

// NewRobotsHandler creates a new RobotsHandler.
func NewRobotsHandler(domain string) *RobotsHandler {
	return &RobotsHandler{cached: []byte(GenerateRobotsTxt(domain))}
}

// ServeHTTP writes the robots.txt response.
func (h *RobotsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serveWithETag(w, r, h.cached, mimePlain, cacheDynamic)
}

// GenerateRobotsTxt produces the robots.txt content string.
// If domain is non-empty, includes a Sitemap directive.
func GenerateRobotsTxt(domain string) string {
	txt := "User-agent: *\nAllow: /\nDisallow: /_assets/\nDisallow: /api/\nDisallow: /debug/\n"
	if domain != "" {
		sitemapURL := seo.PageURL(domain, "/sitemap.xml", "")
		if sitemapURL != "" {
			txt += "\nSitemap: " + sitemapURL + "\n"
		}
	}
	return txt
}
