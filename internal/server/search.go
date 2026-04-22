package server

import (
	"net/http"
	"strconv"

	"github.com/monolithiclab/gomddoc/internal/search"
)

// SearchHandler serves the full-text search JSON API.
type SearchHandler struct {
	index *search.Index
}

// NewSearchHandler creates a new SearchHandler backed by the given search index.
func NewSearchHandler(index *search.Index) *SearchHandler {
	return &SearchHandler{index: index}
}

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

// SearchEndpoint handles GET /api/search?q=<query>&limit=<n>.
// Returns a JSON array of search results ranked by relevance.
func (h *SearchHandler) SearchEndpoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []search.SearchResult{})
		return
	}

	limit := defaultSearchLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = min(parsed, maxSearchLimit)
		}
	}

	results := h.index.Search(q, limit)
	writeJSON(w, http.StatusOK, results)
}
