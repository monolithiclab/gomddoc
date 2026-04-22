package template

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
)

// buildSmallPageContext returns a minimal PageContext with short content,
// no TOC, and no navigation.
func buildSmallPageContext(siteConfig *config.SiteConfig) *TemplateContext {
	return &TemplateContext{
		Site: siteConfig,
		Page: PageContext{
			Content: template.HTML("<h1>Hello</h1><p>Short page.</p>"),
			Path:    "/hello",
		},
	}
}

// buildLargePageContext returns a full PageContext with long HTML content,
// a deep TOC, a navigation tree, and metadata.
func buildLargePageContext(siteConfig *config.SiteConfig) *TemplateContext {
	var sb strings.Builder
	for i := range 20 {
		fmt.Fprintf(&sb, "<h2 id=\"section-%d\">Section %d</h2>\n", i, i)
		sb.WriteString("<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. ")
		sb.WriteString("Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.</p>\n")
	}

	// Build a deep TOC tree
	tocRoot := &enricher.TOCNode{Level: 0}
	for i := range 10 {
		h2 := &enricher.TOCNode{
			Level: 2,
			Text:  fmt.Sprintf("Section %d", i),
			ID:    fmt.Sprintf("section-%d", i),
		}
		for j := range 3 {
			h2.Children = append(h2.Children, &enricher.TOCNode{
				Level: 3,
				Text:  fmt.Sprintf("Subsection %d.%d", i, j),
				ID:    fmt.Sprintf("subsection-%d-%d", i, j),
			})
		}
		tocRoot.Children = append(tocRoot.Children, h2)
	}

	// Build a navigation tree with nested directories
	navItems := make([]enricher.NavItem, 0, 4)
	for i := range 4 {
		dir := enricher.NavItem{
			Title: fmt.Sprintf("Category %d", i),
			Path:  fmt.Sprintf("/cat-%d/", i),
			IsDir: true,
			Open:  i == 0,
		}
		for j := range 5 {
			dir.Children = append(dir.Children, enricher.NavItem{
				Title:  fmt.Sprintf("Page %d-%d", i, j),
				Path:   fmt.Sprintf("/cat-%d/page-%d", i, j),
				Active: i == 0 && j == 0,
			})
		}
		navItems = append(navItems, dir)
	}

	return &TemplateContext{
		Site: siteConfig,
		Page: PageContext{
			Content: template.HTML(sb.String()),
			Path:    "/cat-0/page-0",
			Meta: map[string]any{
				"title":       "Large Page Title",
				"description": "A detailed description for the large page.",
				"tags":        []string{"go", "benchmark", "performance"},
			},
			TOC:        tocRoot,
			Navigation: &enricher.NavTree{Items: navItems},
		},
	}
}

// benchFS returns a test filesystem with a realistic template layout and partials.
func benchFS() fstest.MapFS {
	layout := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.Site.Meta.Title}}</title>
</head>
<body>
{{ template "header" . }}
<main>
  {{ toc .Page.TOC 2 3 }}
  {{ .Page.Content }}
</main>
{{ template "footer" . }}
{{ navigation .Page.Navigation }}
</body>
</html>`

	headerPartial := `{{ define "header" }}<header><nav>{{ .Site.Meta.Title }}</nav></header>{{ end }}`
	footerPartial := `{{ define "footer" }}<footer>&copy; {{ .Site.Meta.Title }}</footer>{{ end }}`

	return fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {Data: []byte(layout)},
		"assets/themes/default/partials/header.html.tmpl": {Data: []byte(headerPartial)},
		"assets/themes/default/partials/footer.html.tmpl": {Data: []byte(footerPartial)},
	}
}

func BenchmarkHTMLRendererRender(b *testing.B) {
	fs := benchFS()
	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Benchmark Site"
	siteConfig.Meta.Description = "Benchmarking template rendering"

	gen := breadcrumb.NewGenerator(func(p string) bool {
		return strings.HasSuffix(p, "/")
	})

	smallCtx := buildSmallPageContext(&siteConfig)
	largeCtx := buildLargePageContext(&siteConfig)

	benchmarks := []struct {
		name   string
		cached bool
		data   *TemplateContext
	}{
		{"Small/NoCache", false, smallCtx},
		{"Small/Cached", true, smallCtx},
		{"Large/NoCache", false, largeCtx},
		{"Large/Cached", true, largeCtx},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()

			var opts []RendererOption
			opts = append(opts, WithBreadcrumbGenerator(gen))
			if bm.cached {
				opts = append(opts, WithCache(&CachedTemplateStore{}))
			}
			renderer := NewHTMLRenderer(&siteConfig, fs, opts...)

			// Warm up cached renderer so the benchmark measures execution, not parsing
			if bm.cached {
				_, _ = renderer.Render(context.Background(), "default.html.tmpl", bm.data)
			}

			ctx := context.Background()
			for b.Loop() {
				result, err := renderer.Render(ctx, "default.html.tmpl", bm.data)
				if err != nil {
					b.Fatalf("Render failed: %v", err)
				}
				if len(result) == 0 {
					b.Fatal("Render returned empty result")
				}
			}
		})
	}
}
