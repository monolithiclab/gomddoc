package server

import (
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/seo"
)

// RobotsHandler serves a robots.txt response.
type RobotsHandler struct {
	domain string
}

// NewRobotsHandler creates a new RobotsHandler.
func NewRobotsHandler(domain string) *RobotsHandler {
	return &RobotsHandler{domain: domain}
}

// ServeHTTP writes the robots.txt response.
func (h *RobotsHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(GenerateRobotsTxt(h.domain)))
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
