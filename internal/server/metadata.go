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

// TagPagesHandler returns all pages matching the given tag.
// GET /api/tags/{tag}
func (h *MetadataHandler) TagPagesHandler(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tag parameter required"})
		return
	}
	if len(tag) > maxTagLength {
		// Over-long tag can't match any indexed tag; return an empty list
		// rather than lowercasing a huge input.
		writeJSON(w, http.StatusOK, []metadata.PageInfo{})
		return
	}

	pages := h.index.ByTag(tag)
	if pages == nil {
		pages = []metadata.PageInfo{}
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
