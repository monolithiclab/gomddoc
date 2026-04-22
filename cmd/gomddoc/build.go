package main

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/common"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/template/breadcrumb"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// BuildCmd holds all flags for the build subcommand.
type BuildCmd struct {
	Dir    string `name:"dir" short:"d" default:"." help:"Markdown source directory or Git URL."`
	Output string `name:"output" short:"o" default:"build/site" help:"Output directory for generated static site."`
}

// buildStats tracks statistics for the build process.
type buildStats struct {
	markdownFiles int
	copiedFiles   int
	skippedFiles  int
	totalBytes    int64
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
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{
		HighlightTheme: cfg.Site.Highlighting.Theme,
		ColorChips:     cfg.Site.ColorChips,
	}))
	registry.Register(renderer.NewPassthroughRenderer())

	breadcrumbGen := breadcrumb.NewGenerator(&infoProviderAdapter{p: prov})

	contentRoot, err := prov.RootFS(context.Background())
	if err != nil {
		return fmt.Errorf("build root fs: %w", err)
	}
	assetsFS := assets.BuildFS(contentRoot, embeddedAssets)

	templateRenderer := tmpl.NewHTMLRenderer(&cfg.Site, assetsFS, tmpl.WithBreadcrumbGenerator(breadcrumbGen))
	if err := templateRenderer.ValidateDefaultTheme(); err != nil {
		return fmt.Errorf("build validate theme: %w", err)
	}

	slog.Info("Building static site", slog.String("source", b.Dir), slog.String("output", b.Output))

	stats, err := b.walkAndBuild(contentRoot, registry, templateRenderer, &cfg.Site)
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	slog.Info("Build complete",
		slog.Int("markdown_files", stats.markdownFiles),
		slog.Int("copied_files", stats.copiedFiles),
		slog.Int("skipped_files", stats.skippedFiles),
		slog.Int64("total_bytes", stats.totalBytes),
		slog.Duration("elapsed", elapsed),
	)

	return nil
}

// walkAndBuild walks the content root and generates the static site.
func (b *BuildCmd) walkAndBuild(
	contentRoot fs.FS,
	registry renderer.RendererRegistry,
	templateRenderer *tmpl.HTMLRenderer,
	siteConfig *config.SiteConfig,
) (*buildStats, error) {
	stats := &buildStats{}
	ctx := context.Background()

	return stats, fs.WalkDir(contentRoot, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", filePath, err)
		}

		// Skip hidden files and directories
		name := d.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			stats.skippedFiles++
			return nil
		}

		if d.IsDir() {
			return nil
		}

		mimeType := common.DetectMIME(filePath)
		normalized := renderer.NormalizeMimeType(mimeType)

		if normalized == "text/markdown" {
			if err := b.buildMarkdownFile(ctx, contentRoot, filePath, registry, templateRenderer, siteConfig, stats); err != nil {
				return err
			}
		} else {
			if err := b.copyFile(contentRoot, filePath, stats); err != nil {
				return err
			}
		}

		return nil
	})
}

// buildMarkdownFile renders a markdown file to HTML and writes it to the output directory.
func (b *BuildCmd) buildMarkdownFile(
	ctx context.Context,
	contentRoot fs.FS,
	filePath string,
	registry renderer.RendererRegistry,
	templateRenderer *tmpl.HTMLRenderer,
	siteConfig *config.SiteConfig,
	stats *buildStats,
) error {
	content, err := fs.ReadFile(contentRoot, filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	mimeType := common.DetectMIME(filePath)
	normalized := renderer.NormalizeMimeType(mimeType)

	contentRenderer, err := registry.Get(normalized)
	if err != nil {
		return fmt.Errorf("get renderer for %s: %w", filePath, err)
	}

	renderResult, err := contentRenderer.Render(ctx, content)
	if err != nil {
		return fmt.Errorf("render %s: %w", filePath, err)
	}

	// Build metadata with title fallback
	metadata := renderResult.Metadata
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
			TOC:     renderResult.TOC,
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
	stats.totalBytes += int64(len(rendered))

	// For README.md files, also write index.html in the same directory
	baseName := strings.ToLower(filepath.Base(filePath))
	if baseName == "readme.md" {
		indexPath := filepath.Join(filepath.Dir(filePath), "index.html")
		if err := b.writeOutputFile(indexPath, rendered); err != nil {
			return err
		}
		stats.totalBytes += int64(len(rendered))
	}

	stats.markdownFiles++
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

	stats.copiedFiles++
	stats.totalBytes += int64(len(content))
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
