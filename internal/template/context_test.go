package template

import (
	"html/template"
	"net/http"
	"testing"
	"testing/fstest"

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

	t.Run("does not write into the caller's EnrichmentData", func(t *testing.T) {
		t.Parallel()

		// No "title" key, so the default-title write fires — the one write
		// BuildPageContext makes into the map. Aliasing put it in the caller's
		// EnrichmentData, which is a data race the moment enrichment is cached.
		enr := &enricher.EnrichmentData{Metadata: map[string]any{"category": "docs"}}

		ctx := BuildPageContext(PageContextInput{
			Site:       site,
			Path:       "/docs/my-page.md",
			Enrichment: enr,
		})

		if ctx.Page.Meta["title"] != "My Page" {
			t.Fatalf("derived title = %v, want My Page", ctx.Page.Meta["title"])
		}
		if _, ok := enr.Metadata["title"]; ok {
			t.Error("BuildPageContext wrote the derived title into the caller's Metadata map")
		}
		// The clone still carries what the caller supplied.
		if ctx.Page.Meta["category"] != "docs" {
			t.Errorf("category = %v, want docs", ctx.Page.Meta["category"])
		}
	})
}

func TestBuildPageContext_SourcePath(t *testing.T) {
	t.Parallel()

	site := &config.SiteConfig{Language: "en-US"}
	tests := []struct {
		name   string
		prefix string
		file   string
		want   string
	}{
		{name: "content dir inside the work tree", prefix: "docs", file: "specs/x.md", want: "docs/specs/x.md"},
		{name: "language pipeline", prefix: "docs/fr-FR", file: "index.md", want: "docs/fr-FR/index.md"},
		{name: "content dir is the work tree root", prefix: "", file: "guide.md", want: "guide.md"},
		{name: "generated listing has no source", prefix: "docs", file: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := NewHTMLRenderer(site, fstest.MapFS{}, WithSourcePrefix(tt.prefix))
			ctx := BuildPageContext(PageContextInput{
				Site:       site,
				Path:       "/x",
				Enrichment: &enricher.EnrichmentData{},
				SourceFile: tt.file,
				Renderer:   r,
			})
			if ctx.Page.SourcePath != tt.want {
				t.Errorf("SourcePath = %q, want %q", ctx.Page.SourcePath, tt.want)
			}
		})
	}

	t.Run("nil renderer keeps the file as given", func(t *testing.T) {
		t.Parallel()
		ctx := BuildPageContext(PageContextInput{
			Site:       site,
			Path:       "/x",
			Enrichment: &enricher.EnrichmentData{},
			SourceFile: "a/b.md",
		})
		if ctx.Page.SourcePath != "a/b.md" {
			t.Errorf("SourcePath = %q, want a/b.md", ctx.Page.SourcePath)
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
