package server

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// maxTagLength bounds the {tag} route segment. Real tags are short; rejecting
// over-long values avoids needlessly lowercasing huge inputs (REVIEW §9.9).
const maxTagLength = 128

// TagPageHandler serves the HTML page for /tags/{tag} (or /{lang}/tags/{tag}).
type TagPageHandler struct {
	index    *metadata.Index
	renderer template.Renderer
	tFunc    func(string) string
	lang     string // "" for default language
}

// NewTagPageHandler binds an index, renderer, and translator to a language scope.
func NewTagPageHandler(index *metadata.Index, renderer template.Renderer, tFunc func(string) string, lang string) *TagPageHandler {
	return &TagPageHandler{index: index, renderer: renderer, tFunc: tFunc, lang: lang}
}

func (h *TagPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" || len(tag) > maxTagLength {
		http.NotFound(w, r)
		return
	}

	pages := h.index.ByTag(tag)
	if len(pages) == 0 {
		http.NotFound(w, r)
		return
	}

	slices.SortFunc(pages, metadata.CompareTitles)

	body, err := h.renderer.RenderTagPage(r.Context(), h.lang, h.tFunc, tag, pages)
	if err != nil {
		slog.Error("render tag page", text.Safe("tag", tag), slog.Any("error", err)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	serveWithETag(w, r, body, mimeHTML, cacheDynamic)
}

// TagsIndexHandler serves the HTML page for /tags/ (or /{lang}/tags/).
type TagsIndexHandler struct {
	index    *metadata.Index
	renderer template.Renderer
	tFunc    func(string) string
	lang     string
}

// NewTagsIndexHandler binds an index, renderer, and translator to a language scope.
func NewTagsIndexHandler(index *metadata.Index, renderer template.Renderer, tFunc func(string) string, lang string) *TagsIndexHandler {
	return &TagsIndexHandler{index: index, renderer: renderer, tFunc: tFunc, lang: lang}
}

func (h *TagsIndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tags := h.index.AllTags() // already alphabetical
	entries := make([]template.TagCount, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, template.TagCount{Tag: tag, Count: h.index.CountByTag(tag)})
	}
	body, err := h.renderer.RenderTagsIndex(r.Context(), h.lang, h.tFunc, entries)
	if err != nil {
		slog.Error("render tags index", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	serveWithETag(w, r, body, mimeHTML, cacheDynamic)
}
