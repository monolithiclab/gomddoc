package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/metadata"
)

// MetadataHandler serves JSON API endpoints for metadata queries.
type MetadataHandler struct {
	index *metadata.Index
}

// NewMetadataHandler creates a new MetadataHandler backed by the given index.
func NewMetadataHandler(index *metadata.Index) *MetadataHandler {
	return &MetadataHandler{index: index}
}

// TagsHandler returns all unique tags as a JSON array.
// GET /api/tags
func (h *MetadataHandler) TagsHandler(w http.ResponseWriter, _ *http.Request) {
	tags := h.index.AllTags()
	writeJSON(w, http.StatusOK, tags)
}

// apiError is the JSON error body for the /api routes. A struct rather than a
// map so the encoder uses its cached path, and a package-level value so the
// 404 — now the endpoint's most common answer — allocates nothing.
type apiError struct {
	Error string `json:"error"`
}

var errUnknownTag = apiError{Error: "unknown tag"}

// TagPagesHandler returns all pages matching the given tag, sorted by title.
// GET /api/tags/{tag}
//
// Index.LookupTag decides whether the tag is known, so this route, /tags/{tag}
// and the MCP tag resource cannot answer that differently.
func (h *MetadataHandler) TagPagesHandler(w http.ResponseWriter, r *http.Request) {
	pages, ok := h.index.LookupTag(r.PathValue("tag"))
	if !ok {
		writeJSON(w, http.StatusNotFound, errUnknownTag)
		return
	}
	writeJSON(w, http.StatusOK, pages)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", mimeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
