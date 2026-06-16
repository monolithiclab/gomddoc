package template

import (
	"html/template"
	"net/http"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestBuildPageContext(t *testing.T) {
	t.Parallel()

	site := &config.SiteConfig{
		Language: "en-US",
		Theme:    config.ThemeConfig{Features: map[string]bool{"toc": true}},
	}
	langs := []LanguageInfo{{Code: "en-US", Active: true, Default: true}}

	t.Run("copies enrichment fields and merges features", func(t *testing.T) {
		t.Parallel()

		enr := &enricher.EnrichmentData{
			Metadata:    map[string]any{"title": "Custom"},
			Features:    map[string]bool{"toc": false},
			TOC:         &enricher.TOCNode{},
			Navigation:  &enricher.NavTree{},
			PrevPage:    &enricher.PageLink{Path: "/prev"},
			NextPage:    &enricher.PageLink{Path: "/next"},
			RelatedDocs: []enricher.RelatedDoc{{Path: "/r.md"}},
		}

		ctx := BuildPageContext(PageContextInput{
			Site:       site,
			Path:       "/guide/intro",
			Content:    template.HTML("<p>hi</p>"),
			Enrichment: enr,
			Lang:       "en-US",
			Languages:  langs,
		})

		if ctx.Page.Meta["title"] != "Custom" {
			t.Errorf("title = %v, want Custom (existing meta preserved)", ctx.Page.Meta["title"])
		}
		if ctx.Page.Content != "<p>hi</p>" {
			t.Errorf("content = %q", ctx.Page.Content)
		}
		// Page override (toc:false) wins over site default (toc:true).
		if ctx.Feature("toc") {
			t.Error("toc feature should be disabled by page override")
		}
		if ctx.Page.TOC != enr.TOC || ctx.Page.Navigation != enr.Navigation {
			t.Error("TOC/Navigation not copied through")
		}
		if ctx.Page.PrevPage != enr.PrevPage || ctx.Page.NextPage != enr.NextPage {
			t.Error("PrevPage/NextPage not copied through")
		}
		if len(ctx.Page.RelatedDocs) != 1 {
			t.Errorf("RelatedDocs len = %d, want 1", len(ctx.Page.RelatedDocs))
		}
		if got := ctx.Languages(); len(got) != 1 || !got[0].Active {
			t.Errorf("languages not wired: %+v", got)
		}
	})

	t.Run("derives title when absent", func(t *testing.T) {
		t.Parallel()

		ctx := BuildPageContext(PageContextInput{
			Site:       site,
			Path:       "/docs/my-page.md",
			Enrichment: &enricher.EnrichmentData{},
		})
		if ctx.Page.Meta["title"] != "My Page" {
			t.Errorf("derived title = %v, want My Page", ctx.Page.Meta["title"])
		}
	})

	t.Run("nil metadata map is allocated", func(t *testing.T) {
		t.Parallel()

		ctx := BuildPageContext(PageContextInput{
			Site:       site,
			Path:       "/x",
			Enrichment: &enricher.EnrichmentData{Metadata: nil},
		})
		if ctx.Page.Meta == nil {
			t.Fatal("Meta should be allocated, got nil")
		}
	})
}

func TestBuildErrorContext(t *testing.T) {
	t.Parallel()

	site := &config.SiteConfig{Language: "en-US"}
	ctx := BuildErrorContext(ErrorContextInput{
		Site:       site,
		Path:       "/404",
		StatusCode: http.StatusNotFound,
		Message:    "Page not found",
		Lang:       "en-US",
		Languages:  []LanguageInfo{{Code: "en-US", Active: true}},
	})

	want := map[string]any{
		"title":         "Not Found",
		"robots":        "noindex",
		"error_code":    http.StatusNotFound,
		"error_title":   "Not Found",
		"error_message": "Page not found",
	}
	for k, v := range want {
		if ctx.Page.Meta[k] != v {
			t.Errorf("Meta[%q] = %v, want %v", k, ctx.Page.Meta[k], v)
		}
	}
	if ctx.Page.Path != "/404" {
		t.Errorf("Path = %q, want /404", ctx.Page.Path)
	}
	if got := ctx.Languages(); len(got) != 1 {
		t.Errorf("languages not wired through: %+v", got)
	}
}
