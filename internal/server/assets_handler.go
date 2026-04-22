package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/common"
)

// AssetsHandler serves static theme and shared assets from a layered filesystem.
type AssetsHandler struct {
	staticFS fs.FS
}

// NewAssetsHandler creates a handler that serves files from the given static filesystem.
func NewAssetsHandler(staticFS fs.FS) *AssetsHandler {
	return &AssetsHandler{staticFS: staticFS}
}

// ServeHTTP serves a static file from the overlay filesystem.
// Blocks directory listings and dotfile access, sets cache headers,
// and supports ETag-based conditional requests.
func (h *AssetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/")

	if filePath == "" || filePath == "." {
		http.NotFound(w, r)
		return
	}

	// Block dotfiles
	for segment := range strings.SplitSeq(filePath, "/") {
		if segment != "" && strings.HasPrefix(segment, ".") {
			http.NotFound(w, r)
			return
		}
	}

	// Stat first to reject directories without reading content
	info, err := fs.Stat(h.staticFS, filePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	content, err := fs.ReadFile(h.staticFS, filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", common.DetectMIME(filePath))

	etag := generateETag(content)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	if checkETag(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	_, _ = w.Write(content)
}
