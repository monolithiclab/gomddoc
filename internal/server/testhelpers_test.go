package server

import (
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/testutil/provider"
)

// Re-export MemoryProvider from testutil for backward compatibility in server tests
type memoryProvider = provider.MemoryProvider

func newMemoryProvider(files fstest.MapFS, defaultIndex string, dirIndex bool) *memoryProvider {
	return provider.NewMemoryProvider(files, defaultIndex, dirIndex)
}

func setupTestRenderer() *tmpl.HTMLRenderer {
	templateContent := `<!DOCTYPE html>
<html>
<head><title>{{.Site.Meta.Title}}</title></head>
<body>{{.Page.Content}}</body>
</html>`

	testFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": {
			Data: []byte(templateContent),
		},
	}

	siteConfig := config.NewSiteConfig(".")
	siteConfig.Meta.Title = "Test Site"

	return tmpl.NewHTMLRenderer(&siteConfig, testFS)
}

func setupTestRegistry() renderer.RendererRegistry {
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownPassthroughRenderer())
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{ColorChips: true}))
	registry.Register(renderer.NewPassthroughRenderer())
	return registry
}

func setupTestEnricherRegistry() enricher.EnricherRegistry {
	reg := enricher.NewDefaultEnricherRegistry()
	reg.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{}))
	return reg
}
