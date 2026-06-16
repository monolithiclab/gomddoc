package template

import (
	"html/template"
	"net/http"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// PageContextInput holds the inputs for assembling a content-page
// TemplateContext. Both the live server and the static build feed it so the two
// code paths cannot silently diverge — every page field is set in one place.
type PageContextInput struct {
	Site       *config.SiteConfig
	Path       string                   // Request path (serve) or "/"+filePath (build)
	Content    template.HTML            // Rendered HTML body
	Enrichment *enricher.EnrichmentData // Metadata, TOC, navigation, related docs

	// i18n
	Lang      string
	TFunc     func(string) string
	Languages []LanguageInfo
}

// BuildPageContext assembles the TemplateContext for a rendered content page.
// It applies the default-title fallback, merges site + page features, copies the
// enrichment fields, and wires i18n. Callers pass already-rendered content and
// pre-sorted related docs; this function adds no ordering of its own.
func BuildPageContext(in PageContextInput) *TemplateContext {
	meta := in.Enrichment.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	if _, ok := meta["title"]; !ok {
		meta["title"] = text.DeriveTitle(in.Path)
	}

	ctx := &TemplateContext{
		Site: in.Site,
		Page: PageContext{
			Content:     in.Content,
			Path:        in.Path,
			Meta:        meta,
			Features:    config.MergeFeatures(in.Site.Theme.Features, in.Enrichment.Features),
			TOC:         in.Enrichment.TOC,
			Navigation:  in.Enrichment.Navigation,
			PrevPage:    in.Enrichment.PrevPage,
			NextPage:    in.Enrichment.NextPage,
			RelatedDocs: in.Enrichment.RelatedDocs,
		},
	}
	ctx.WithI18n(in.Lang, in.TFunc, in.Languages)
	return ctx
}

// ErrorContextInput holds the inputs for assembling an error-page
// TemplateContext, shared by the server's runtime error responses and the
// build's static 404 generation.
type ErrorContextInput struct {
	Site       *config.SiteConfig
	Path       string // Page path for the error (e.g. the requested URL, or "/404")
	StatusCode int
	Message    string // Human-readable message (e.g. server.StatusMessage(code))

	// i18n
	Lang      string
	TFunc     func(string) string
	Languages []LanguageInfo
}

// BuildErrorContext assembles the TemplateContext for an error page. The status
// title is derived from the code via http.StatusText so both callers produce
// identical metadata.
func BuildErrorContext(in ErrorContextInput) *TemplateContext {
	statusTitle := http.StatusText(in.StatusCode)

	ctx := &TemplateContext{
		Site: in.Site,
		Page: PageContext{
			Path: in.Path,
			Meta: map[string]any{
				"title":         statusTitle,
				"robots":        "noindex",
				"error_code":    in.StatusCode,
				"error_title":   statusTitle,
				"error_message": in.Message,
			},
			Features: config.MergeFeatures(in.Site.Theme.Features),
		},
	}
	ctx.WithI18n(in.Lang, in.TFunc, in.Languages)
	return ctx
}
