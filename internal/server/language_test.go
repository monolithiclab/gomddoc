package server

import (
	"net/http/httptest"
	"testing"
)

func TestResolveAPILanguage(t *testing.T) {
	t.Parallel()

	known := map[string]bool{"en-US": true, "fr-FR": true}

	tests := []struct {
		name       string
		query      string
		acceptLang string
		defaultL   string
		expected   string
	}{
		{
			name:     "query param wins",
			query:    "lang=fr-FR",
			defaultL: "en-US",
			expected: "fr-FR",
		},
		{
			name:       "query param takes precedence over Accept-Language",
			query:      "lang=en-US",
			acceptLang: "fr-FR",
			defaultL:   "en-US",
			expected:   "en-US",
		},
		{
			name:       "Accept-Language header fallback",
			acceptLang: "fr-FR",
			defaultL:   "en-US",
			expected:   "fr-FR",
		},
		{
			name:       "Accept-Language with quality values",
			acceptLang: "fr-FR;q=0.9, en-US;q=0.8",
			defaultL:   "en-US",
			expected:   "fr-FR",
		},
		{
			name:     "unknown lang falls back to default",
			query:    "lang=ja-JP",
			defaultL: "en-US",
			expected: "en-US",
		},
		{
			name:       "unknown Accept-Language falls back to default",
			acceptLang: "ja-JP",
			defaultL:   "en-US",
			expected:   "en-US",
		},
		{
			name:     "no lang specified uses default",
			defaultL: "en-US",
			expected: "en-US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest("GET", "/api/search?q=test&"+tt.query, nil)
			if tt.acceptLang != "" {
				r.Header.Set("Accept-Language", tt.acceptLang)
			}

			got := ResolveAPILanguage(r, known, tt.defaultL)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func Test_parseAcceptLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{name: "simple tag", header: "fr-FR", expected: "fr-FR"},
		{name: "with quality", header: "fr-FR;q=0.9", expected: "fr-FR"},
		{name: "multiple tags", header: "fr-FR, en-US;q=0.8", expected: "fr-FR"},
		{name: "wildcard", header: "*", expected: "*"},
		{name: "empty", header: "", expected: ""},
		{name: "spaces", header: " en-US ; q=1.0 , fr-FR", expected: "en-US"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseAcceptLanguage(tt.header)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
