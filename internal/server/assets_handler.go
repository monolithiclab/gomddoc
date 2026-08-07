package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
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
//
// The 404s here stay http.NotFound rather than going through ErrorPage, which
// every content route shares: an asset request is a sub-resource fetch, so
// answering it with a themed HTML page costs a template render and hands the
// browser HTML where it asked for CSS, a font, or an image. Nothing about which
// assets exist is worth hiding — they are all served unauthenticated.
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

	serveWithETag(w, r, content, negotiate.DetectMIME(filePath), cacheImmutable)
}
