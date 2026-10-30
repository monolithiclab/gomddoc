package server

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/monolithiclab/gomddoc/internal/resolve"
	"github.com/monolithiclab/gomddoc/internal/search"
)

// SearchHandler serves the full-text search JSON API.
type SearchHandler struct {
	sources     map[string]SearchSource // lang → source ("" for single-language mode)
	defaultLang string
	knownLangs  map[string]bool
}

// SearchSource is one tree's search index and what turns its result paths,
// the Markdown files it indexed, into the page URLs the theme's search box
// links: the tree's resolver and its language prefix ("" for the default
// tree). A result linked by its file path would reach the page's Markdown URL,
// which serves text/markdown (D54).
type SearchSource struct {
	Index        *search.Index
	Resolver     *resolve.PathResolver
	DefaultIndex string
	PathPrefix   string
}

// NewSearchHandler creates a new SearchHandler backed by a single search index.
func NewSearchHandler(source SearchSource) *SearchHandler {
	return &SearchHandler{
		sources: map[string]SearchSource{"": source},
	}
}

// NewMultiLangSearchHandler creates a SearchHandler with per-language sources.
// The defaultLang key must exist in the sources map.
func NewMultiLangSearchHandler(sources map[string]SearchSource, defaultLang string) *SearchHandler {
	known := make(map[string]bool, len(sources))
	for lang := range sources {
		known[lang] = true
	}
	return &SearchHandler{
		sources:     sources,
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

	source := h.resolveSource(r)
	if source.Index == nil {
		writeJSON(w, http.StatusOK, []search.SearchResult{})
		return
	}

	// When language is resolved from Accept-Language, the response varies by it.
	if h.knownLangs != nil {
		w.Header().Add("Vary", "Accept-Language")
	}

	results := source.Index.Search(q, limit)
	for i := range results {
		results[i].Path = source.PathPrefix + source.Resolver.PageURLPath(results[i].Path, source.DefaultIndex)
	}
	writeJSON(w, http.StatusOK, results)
}

// resolveSource selects the search source for the request.
// In single-language mode (knownLangs is nil), uses the "" key.
// In multi-language mode, resolves via ResolveAPILanguage.
func (h *SearchHandler) resolveSource(r *http.Request) SearchSource {
	if h.knownLangs == nil {
		return h.sources[""]
	}
	return h.sources[ResolveAPILanguage(r, h.knownLangs, h.defaultLang)]
}
