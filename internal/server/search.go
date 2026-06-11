package server

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/monolithiclab/gomddoc/internal/search"
)

// SearchHandler serves the full-text search JSON API.
type SearchHandler struct {
	indexes     map[string]*search.Index // lang → index ("" for single-language mode)
	defaultLang string
	knownLangs  map[string]bool
}

// NewSearchHandler creates a new SearchHandler backed by a single search index.
func NewSearchHandler(index *search.Index) *SearchHandler {
	return &SearchHandler{
		indexes: map[string]*search.Index{"": index},
	}
}

// NewMultiLangSearchHandler creates a SearchHandler with per-language indexes.
// The defaultLang key must exist in the indexes map.
func NewMultiLangSearchHandler(indexes map[string]*search.Index, defaultLang string) *SearchHandler {
	known := make(map[string]bool, len(indexes))
	for lang := range indexes {
		known[lang] = true
	}
	return &SearchHandler{
		indexes:     indexes,
		defaultLang: defaultLang,
		knownLangs:  known,
	}
}

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
	maxQueryLength     = 500
)

// truncateQuery caps the query at maxQueryLength bytes. Cutting on a byte
// boundary can split a multi-byte rune, so any trailing partial rune left by
// the cut is trimmed to keep the query valid UTF-8.
func truncateQuery(q string) string {
	if len(q) <= maxQueryLength {
		return q
	}
	q = q[:maxQueryLength]
	for len(q) > 0 {
		if r, size := utf8.DecodeLastRuneInString(q); r != utf8.RuneError || size > 1 {
			break
		}
		q = q[:len(q)-1]
	}
	return q
}

// SearchEndpoint handles GET /api/search?q=<query>&limit=<n>&lang=<code>.
// Returns a JSON array of search results ranked by relevance.
func (h *SearchHandler) SearchEndpoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []search.SearchResult{})
		return
	}
	q = truncateQuery(q)

	limit := defaultSearchLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = min(parsed, maxSearchLimit)
		}
	}

	index := h.resolveIndex(r)
	if index == nil {
		writeJSON(w, http.StatusOK, []search.SearchResult{})
		return
	}

	// When language is resolved from Accept-Language, the response varies by it.
	if h.knownLangs != nil {
		w.Header().Add("Vary", "Accept-Language")
	}

	results := index.Search(q, limit)
	writeJSON(w, http.StatusOK, results)
}

// resolveIndex selects the search index for the request.
// In single-language mode (knownLangs is nil), uses the "" key.
// In multi-language mode, resolves via ResolveAPILanguage.
func (h *SearchHandler) resolveIndex(r *http.Request) *search.Index {
	if h.knownLangs == nil {
		return h.indexes[""]
	}
	lang := ResolveAPILanguage(r, h.knownLangs, h.defaultLang)
	return h.indexes[lang]
}
