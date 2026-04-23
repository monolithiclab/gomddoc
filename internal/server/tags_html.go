package server

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// TagPageHandler serves the HTML page for /tags/{tag} (or /{lang}/tags/{tag}).
type TagPageHandler struct {
	index    *metadata.Index
	renderer *template.HTMLRenderer
	tFunc    func(string) string
	lang     string // "" for default language
}

// NewTagPageHandler binds an index, renderer, and translator to a language scope.
func NewTagPageHandler(index *metadata.Index, renderer *template.HTMLRenderer, tFunc func(string) string, lang string) *TagPageHandler {
	return &TagPageHandler{index: index, renderer: renderer, tFunc: tFunc, lang: lang}
}

func (h *TagPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" {
		http.NotFound(w, r)
		return
	}

	pages := h.index.ByTag(tag)
	if len(pages) == 0 {
		http.NotFound(w, r)
		return
	}

	slices.SortFunc(pages, func(a, b metadata.PageInfo) int {
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	body, err := h.renderer.RenderTagPage(r.Context(), h.lang, h.tFunc, tag, pages)
	if err != nil {
		slog.Error("render tag page", text.Safe("tag", tag), slog.Any("error", err)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	serveWithETag(w, r, body, "text/html; charset=utf-8", "public, max-age=300")
}
