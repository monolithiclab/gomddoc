package server

import (
	"log/slog"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// TagHandlerConfig holds what both tag handlers need. ErrorPage must be the one
// the language's content handler uses, so /tags/nope and /nope answer alike —
// they used to differ, net/http's plain default against the theme's 404.
type TagHandlerConfig struct {
	Index     *metadata.Index
	Renderer  template.Renderer
	TFunc     func(string) string
	Lang      string // "" for default language
	ErrorPage *ErrorPage
}

// TagPageHandler serves the HTML page for /tags/{tag} (or /{lang}/tags/{tag}).
type TagPageHandler struct {
	TagHandlerConfig
}

// NewTagPageHandler binds an index, renderer, and translator to a language scope.
func NewTagPageHandler(cfg TagHandlerConfig) *TagPageHandler {
	return &TagPageHandler{cfg}
}

func (h *TagPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	pages, ok := h.Index.LookupTag(tag)
	if !ok {
		h.ErrorPage.NotFound(w, r)
		return
	}

	body, err := h.Renderer.RenderTagPage(r.Context(), h.Lang, h.TFunc, tag, pages)
	if err != nil {
		slog.Error("render tag page", text.Safe("tag", tag), slog.Any("error", err)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
		h.ErrorPage.Write(w, r, http.StatusInternalServerError, r.URL.Path)
		return
	}
	serveWithETag(w, r, body, mimeHTML, cacheDynamic)
}

// TagsIndexHandler serves the HTML page for /tags/ (or /{lang}/tags/).
type TagsIndexHandler struct {
	TagHandlerConfig
}

// NewTagsIndexHandler binds an index, renderer, and translator to a language scope.
func NewTagsIndexHandler(cfg TagHandlerConfig) *TagsIndexHandler {
	return &TagsIndexHandler{cfg}
}

func (h *TagsIndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tags := h.Index.AllTags() // already alphabetical
	entries := make([]template.TagCount, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, template.TagCount{Tag: tag, Count: h.Index.CountByTag(tag)})
	}
	body, err := h.Renderer.RenderTagsIndex(r.Context(), h.Lang, h.TFunc, entries)
	if err != nil {
		slog.Error("render tags index", slog.Any("error", err))
		h.ErrorPage.Write(w, r, http.StatusInternalServerError, r.URL.Path)
		return
	}
	serveWithETag(w, r, body, mimeHTML, cacheDynamic)
}
