package seo

import "testing"

func TestPageURL(t *testing.T) {
	tests := []struct {
		name         string
		domain       string
		pagePath     string
		defaultIndex string
		want         string
	}{
		{
			name:         "empty domain returns empty",
			domain:       "",
			pagePath:     "/docs/guide.md",
			defaultIndex: "README.md",
			want:         "",
		},
		{
			name:         "domain without scheme",
			domain:       "docs.example.com",
			pagePath:     "/docs/guide.md",
			defaultIndex: "README.md",
			want:         "https://docs.example.com/docs/guide.md",
		},
		{
			name:         "domain with https scheme",
			domain:       "https://docs.example.com",
			pagePath:     "/docs/guide.md",
			defaultIndex: "README.md",
			want:         "https://docs.example.com/docs/guide.md",
		},
		{
			name:         "domain with http scheme preserved",
			domain:       "http://localhost:8080",
			pagePath:     "/docs/guide.md",
			defaultIndex: "README.md",
			want:         "http://localhost:8080/docs/guide.md",
		},
		{
			name:         "strips trailing default index",
			domain:       "https://docs.example.com",
			pagePath:     "/docs/README.md",
			defaultIndex: "README.md",
			want:         "https://docs.example.com/docs/",
		},
		{
			name:         "root README.md becomes /",
			domain:       "https://docs.example.com",
			pagePath:     "/README.md",
			defaultIndex: "README.md",
			want:         "https://docs.example.com/",
		},
		{
			name:         "root path",
			domain:       "https://docs.example.com",
			pagePath:     "/",
			defaultIndex: "README.md",
			want:         "https://docs.example.com/",
		},
		{
			name:         "domain with trailing slash",
			domain:       "https://docs.example.com/",
			pagePath:     "/guide.md",
			defaultIndex: "README.md",
			want:         "https://docs.example.com/guide.md",
		},
		{
			name:         "custom default index",
			domain:       "https://docs.example.com",
			pagePath:     "/docs/index.md",
			defaultIndex: "index.md",
			want:         "https://docs.example.com/docs/",
		},
		{
			name:         "empty default index no stripping",
			domain:       "https://docs.example.com",
			pagePath:     "/docs/README.md",
			defaultIndex: "",
			want:         "https://docs.example.com/docs/README.md",
		},
		{
			name:         "domain with subpath",
			domain:       "https://example.com/docs",
			pagePath:     "/guide.md",
			defaultIndex: "README.md",
			want:         "https://example.com/docs/guide.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PageURL(tt.domain, tt.pagePath, tt.defaultIndex)
			if got != tt.want {
				t.Errorf("PageURL(%q, %q, %q) = %q, want %q",
					tt.domain, tt.pagePath, tt.defaultIndex, got, tt.want)
			}
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   string
	}{
		{"docs.example.com", "https://docs.example.com"},
		{"https://docs.example.com", "https://docs.example.com"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://docs.example.com/", "https://docs.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := normalizeDomain(tt.domain)
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}

func TestNormalizePagePath(t *testing.T) {
	tests := []struct {
		name         string
		pagePath     string
		defaultIndex string
		want         string
	}{
		{"strips README.md", "/docs/README.md", "README.md", "/docs/"},
		{"root README.md", "/README.md", "README.md", "/"},
		{"no match", "/docs/guide.md", "README.md", "/docs/guide.md"},
		{"empty path", "", "README.md", "/"},
		{"dot path", ".", "README.md", "/"},
		{"empty default index", "/README.md", "", "/README.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePagePath(tt.pagePath, tt.defaultIndex)
			if got != tt.want {
				t.Errorf("normalizePagePath(%q, %q) = %q, want %q",
					tt.pagePath, tt.defaultIndex, got, tt.want)
			}
		})
	}
}
