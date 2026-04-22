package main

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/common"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// BuildCmd holds all flags for the build subcommand.
type BuildCmd struct {
	Dir    string `arg:"" optional:"" default:"." help:"Markdown source directory or Git URL."`
	Output string `name:"output" short:"o" default:"build/site" help:"Output directory for generated static site."`
}

// buildStats tracks statistics for the build process.
// All fields use atomic types for safe concurrent updates during parallel builds.
type buildStats struct {
	markdownFiles atomic.Int64
	copiedFiles   atomic.Int64
	skippedFiles  atomic.Int64
	totalBytes    atomic.Int64
}

// Run executes the build command.
func (b *BuildCmd) Run() error {
	start := time.Now()

	cfg, err := config.NewFromServeArgs(b.Dir, ":8080", false, "")
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}

	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex)
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}
	defer func() {
		if closeErr := prov.Close(); closeErr != nil {
			slog.Error("Failed to close provider", slog.Any("error", closeErr))
		}
	}()

	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownPassthroughRenderer())
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{
		HighlightTheme: cfg.Site.Highlighting.Theme,
		ColorChips:     cfg.Site.ColorChips,
	}))
	registry.Register(renderer.NewPassthroughRenderer())

	breadcrumbGen := breadcrumb.NewGenerator(func(path string) bool {
		info, err := prov.Stat(context.Background(), path)
		return err == nil && info.IsDir()
	})

	contentRoot, err := prov.RootFS(context.Background())
	if err != nil {
		return fmt.Errorf("build root fs: %w", err)
	}
	assetsFS := assets.BuildFS(contentRoot, embeddedAssets)

	templateRenderer := tmpl.NewHTMLRenderer(&cfg.Site, assetsFS, tmpl.WithBreadcrumbGenerator(breadcrumbGen))
	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		return fmt.Errorf("build validate theme: %w", err)
	}

	enricherRegistry := enricher.NewDefaultEnricherRegistry()
	enricherRegistry.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{}))

	slog.Info("Building static site", slog.String("source", b.Dir), slog.String("output", b.Output))

	stats, err := b.walkAndBuild(contentRoot, registry, enricherRegistry, templateRenderer, &cfg.Site)
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	slog.Info("Build complete",
		slog.Int64("markdown_files", stats.markdownFiles.Load()),
		slog.Int64("copied_files", stats.copiedFiles.Load()),
		slog.Int64("skipped_files", stats.skippedFiles.Load()),
		slog.Int64("total_bytes", stats.totalBytes.Load()),
		slog.Duration("elapsed", elapsed),
	)

	return nil
}

// walkAndBuild walks the content root and generates the static site.
// Files are collected first via fs.WalkDir, then processed in parallel
// using an errgroup worker pool bounded by runtime.NumCPU().
func (b *BuildCmd) walkAndBuild(
	contentRoot fs.FS,
	registry renderer.RendererRegistry,
	enricherRegistry enricher.EnricherRegistry,
	templateRenderer *tmpl.HTMLRenderer,
	siteConfig *config.SiteConfig,
) (*buildStats, error) {
	stats := &buildStats{}

	// Collect all file paths first, skipping hidden files/directories.
	var filePaths []string
	err := fs.WalkDir(contentRoot, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", filePath, err)
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			stats.skippedFiles.Add(1)
			return nil
		}

		if !d.IsDir() {
			filePaths = append(filePaths, filePath)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Process files in parallel with a bounded worker pool.
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(runtime.NumCPU())

	// Build accepts only HTML output — this selects the markdown→HTML renderer
	// for markdown files and falls through to copy for everything else.
	htmlAccept := []negotiate.MediaType{{Type: "text", Subtype: "html", Q: 1.0}}

	for _, fp := range filePaths {
		g.Go(func() error {
			mimeType := common.DetectMIME(fp)
			normalized := common.NormalizeMimeType(mimeType)

			contentRenderer, _, err := registry.Get(normalized, htmlAccept)
			if err != nil {
				// No HTML renderer for this type → copy as-is
				return b.copyFile(contentRoot, fp, stats)
			}
			return b.buildFile(ctx, contentRoot, fp, contentRenderer, enricherRegistry, normalized, templateRenderer, siteConfig, stats)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return stats, nil
}

// buildFile renders a file to HTML and writes it to the output directory.
func (b *BuildCmd) buildFile(
	ctx context.Context,
	contentRoot fs.FS,
	filePath string,
	contentRenderer renderer.ContentRenderer,
	enricherRegistry enricher.EnricherRegistry,
	mimeType string,
	templateRenderer *tmpl.HTMLRenderer,
	siteConfig *config.SiteConfig,
	stats *buildStats,
) error {
	content, err := fs.ReadFile(contentRoot, filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	enrichment, err := enricherRegistry.Get(mimeType).Enrich(ctx, content, "/"+filePath)
	if err != nil {
		return fmt.Errorf("enrich %s: %w", filePath, err)
	}

	renderResult, err := contentRenderer.Render(ctx, content, enrichment)
	if err != nil {
		return fmt.Errorf("render %s: %w", filePath, err)
	}

	// Build metadata with title fallback
	metadata := enrichment.Metadata
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if _, ok := metadata["title"]; !ok {
		metadata["title"] = text.DeriveTitle("/" + filePath)
	}

	templateCtx := &tmpl.TemplateContext{
		Site: siteConfig,
		Page: tmpl.PageContext{
			Content: template.HTML(renderResult.Content), // #nosec G203
			Path:    "/" + filePath,
			Meta:    metadata,
			TOC:     enrichment.TOC,
		},
	}

	rendered, err := templateRenderer.Render(ctx, "default.html.tmpl", templateCtx)
	if err != nil {
		return fmt.Errorf("template render %s: %w", filePath, err)
	}

	// Write the .html file
	htmlPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".html"
	if err := b.writeOutputFile(htmlPath, rendered); err != nil {
		return err
	}
	stats.totalBytes.Add(int64(len(rendered)))

	// For README.md files, also write index.html in the same directory
	baseName := strings.ToLower(filepath.Base(filePath))
	if baseName == "readme.md" {
		indexPath := filepath.Join(filepath.Dir(filePath), "index.html")
		if err := b.writeOutputFile(indexPath, rendered); err != nil {
			return err
		}
		stats.totalBytes.Add(int64(len(rendered)))
	}

	stats.markdownFiles.Add(1)
	slog.Debug("Built", slog.String("file", filePath), slog.String("output", htmlPath))

	return nil
}

// copyFile copies a non-markdown file to the output directory as-is.
func (b *BuildCmd) copyFile(contentRoot fs.FS, filePath string, stats *buildStats) error {
	content, err := fs.ReadFile(contentRoot, filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	if err := b.writeOutputFile(filePath, content); err != nil {
		return err
	}

	stats.copiedFiles.Add(1)
	stats.totalBytes.Add(int64(len(content)))
	slog.Debug("Copied", slog.String("file", filePath))

	return nil
}

// writeOutputFile writes content to a file in the output directory, creating parent directories as needed.
func (b *BuildCmd) writeOutputFile(relPath string, content []byte) error {
	outPath := filepath.Join(b.Output, relPath)
	dir := filepath.Dir(outPath)

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outPath, content, 0644); err != nil { // #nosec G306
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}
