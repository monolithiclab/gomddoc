package template

import (
	"html/template"
	"maps"
	"net/http"
	"time"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
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

	ModTime time.Time // Source file mtime; zero when unknown (see seo.LastModified)

	// Renderer supplies the page data derived from Path — currently the
	// breadcrumb trail. Passing the renderer rather than the trail keeps the two
	// in agreement by construction. May be nil, in which case Breadcrumbs is nil.
	Renderer Renderer

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
	// Cloned because Meta is the one enrichment field this function *writes*
	// (the default title below); the rest are copied through and read-only.
	// Aliasing put that write in the caller's EnrichmentData.
	// maps.Clone(nil) is nil, so the empty-map fallback still fires.
	meta := maps.Clone(in.Enrichment.Metadata)
	if meta == nil {
		meta = make(map[string]any)
	}
	if _, ok := meta["title"]; !ok {
		meta["title"] = text.DeriveTitle(in.Path)
	}

	// Generated here, once, because both the layout's breadcrumb bar and the
	// JSON-LD partial read PageContext.Breadcrumbs.
	var breadcrumbs []breadcrumb.Breadcrumb
	if in.Renderer != nil {
		breadcrumbs = in.Renderer.Breadcrumbs(in.Path)
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
			Breadcrumbs: breadcrumbs,
			ModTime:     in.ModTime,
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
// identical metadata. Breadcrumbs are deliberately left nil: error pages carry
// no breadcrumb bar, and a noindex page has no use for a JSON-LD BreadcrumbList.
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
