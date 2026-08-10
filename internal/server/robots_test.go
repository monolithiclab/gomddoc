package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRobotsHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		domain         string
		wantSitemap    bool
		wantSitemapURL string
	}{
		{
			name:           "with domain includes sitemap",
			domain:         "https://docs.example.com",
			wantSitemap:    true,
			wantSitemapURL: "https://docs.example.com/sitemap.xml",
		},
		{
			name:        "without domain no sitemap",
			domain:      "",
			wantSitemap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NewRobotsHandler(tt.domain, "/sitemap.xml")
			req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}

			assertRevalidates(t, revalidationCase{
				Handler:   handler,
				Path:      "/robots.txt",
				WantType:  mimePlain,
				WantCache: cacheDynamic,
			})

			body := w.Body.String()
			if !strings.Contains(body, "User-agent: *") {
				t.Error("should contain User-agent directive")
			}
			if !strings.Contains(body, "Disallow: /_assets/") {
				t.Error("should disallow /_assets/")
			}
			if !strings.Contains(body, "Disallow: /api/") {
				t.Error("should disallow /api/")
			}

			hasSitemap := strings.Contains(body, "Sitemap:")
			if hasSitemap != tt.wantSitemap {
				t.Errorf("Sitemap directive present = %v, want %v", hasSitemap, tt.wantSitemap)
			}
			if tt.wantSitemap && !strings.Contains(body, tt.wantSitemapURL) {
				t.Errorf("body should contain %q", tt.wantSitemapURL)
			}
		})
	}
}

func TestGenerateRobotsTxt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		domain      string
		sitemapPath string
		want        string
	}{
		{
			name:        "with domain",
			domain:      "https://docs.example.com",
			sitemapPath: "/sitemap.xml",
			want:        "Sitemap: https://docs.example.com/sitemap.xml",
		},
		{
			// A multi-language build publishes sitemap-index.xml precisely to
			// be the single entry point; robots.txt used to send crawlers to
			// the default-language sitemap instead, which by design lists no
			// translated page.
			name:        "index path is emitted verbatim",
			domain:      "https://docs.example.com",
			sitemapPath: "/sitemap-index.xml",
			want:        "Sitemap: https://docs.example.com/sitemap-index.xml",
		},
		{
			// Serve without a metadata index registers no /sitemap.xml route,
			// so advertising one would be a 404 on the only URL robots.txt
			// names.
			name:        "nothing published, no directive",
			domain:      "https://docs.example.com",
			sitemapPath: "",
		},
		{
			name:        "without domain",
			domain:      "",
			sitemapPath: "/sitemap.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GenerateRobotsTxt(tt.domain, tt.sitemapPath)
			if !strings.Contains(got, "User-agent: *") {
				t.Error("should contain User-agent directive")
			}
			if tt.want == "" {
				if strings.Contains(got, "Sitemap:") {
					t.Errorf("nothing to advertise, so there should be no Sitemap directive; got:\n%s", got)
				}
				return
			}
			// Delimited on both sides: a bare Contains on the index case also
			// passes when the line reads .../sitemap-index.xml.gz, and
			// /sitemap.xml is a suffix of /sitemap-index.xml.
			if !strings.Contains(got, "\n"+tt.want+"\n") {
				t.Errorf("should contain %q on its own line; got:\n%s", tt.want, got)
			}
		})
	}
}
