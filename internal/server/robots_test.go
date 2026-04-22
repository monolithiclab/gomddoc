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
			handler := NewRobotsHandler(tt.domain)
			req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}

			ct := w.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "text/plain") {
				t.Errorf("Content-Type = %q, want text/plain", ct)
			}

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
		name   string
		domain string
		want   string
	}{
		{
			name:   "with domain",
			domain: "https://docs.example.com",
			want:   "Sitemap: https://docs.example.com/sitemap.xml",
		},
		{
			name:   "without domain",
			domain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GenerateRobotsTxt(tt.domain)
			if !strings.Contains(got, "User-agent: *") {
				t.Error("should contain User-agent directive")
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("should contain %q", tt.want)
			}
		})
	}
}
