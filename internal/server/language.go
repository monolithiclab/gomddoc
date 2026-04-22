package server

import (
	"net/http"
	"strings"
)

// ResolveAPILanguage determines the language for an API request.
// Priority: ?lang= query param > Accept-Language header > default.
// Unknown languages fall back to the default.
func ResolveAPILanguage(r *http.Request, known map[string]bool, defaultLang string) string {
	// 1. Query param.
	if lang := r.URL.Query().Get("lang"); lang != "" {
		if known[lang] {
			return lang
		}
		return defaultLang
	}

	// 2. Accept-Language header (take first matching tag).
	if accept := r.Header.Get("Accept-Language"); accept != "" {
		lang := parseAcceptLanguage(accept)
		if lang != "" && known[lang] {
			return lang
		}
	}

	return defaultLang
}

// parseAcceptLanguage extracts the first BCP 47 tag from an Accept-Language header.
func parseAcceptLanguage(header string) string {
	first, _, _ := strings.Cut(header, ",")
	first = strings.TrimSpace(first)
	tag, _, _ := strings.Cut(first, ";")
	return strings.TrimSpace(tag)
}
