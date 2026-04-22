package main

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/metadata"
	"github.com/monolithiclab/gomddoc/internal/negotiate"
	"github.com/monolithiclab/gomddoc/internal/provider"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	"github.com/monolithiclab/gomddoc/internal/server"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	"github.com/monolithiclab/gomddoc/internal/text"
)

// BuildCmd holds all flags for the build subcommand.
type BuildCmd struct {
	Dir    string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown source directory or Git URL."`
	Output string `name:"output" short:"o" default:"build/site" env:"GOMDDOC_BUILD_OUTPUT" help:"Output directory for generated static site."`
	Domain string `name:"domain" short:"d" default:"" env:"GOMDDOC_DOMAIN" help:"Override site domain for canonical URLs, sitemap, and SEO tags."`
	Force  bool   `name:"force" short:"f" default:"false" env:"GOMDDOC_BUILD_FORCE" help:"Overwrite output directory if it already exists."`
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

	if err := b.guardOutputDir(); err != nil {
		return err
	}

	cfg, err := config.NewFromDir(b.Dir)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}

	if b.Domain != "" {
		cfg.Site.Meta.Domain = b.Domain
		if err := cfg.Site.Validate(); err != nil {
			return fmt.Errorf("domain flag: %w", err)
		}
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

	pipeline, err := setupPipeline(cfg, prov, PipelineOptions{
		EnableCache:      true,
		EnableNavigation: false, // build doesn't serve navigation
		EnableMetadata:   false, // build doesn't serve tag API
	})
	if err != nil {
		return fmt.Errorf("build pipeline: %w", err)
	}

	contentRoot, err := prov.RootFS(context.Background())
	if err != nil {
		return fmt.Errorf("build root fs: %w", err)
	}

	slog.Info("Building static site", slog.String("source", b.Dir), slog.String("output", b.Output))

	stats, err := b.walkAndBuild(contentRoot, pipeline.Registry, pipeline.EnricherRegistry, pipeline.TemplateRenderer, &cfg.Site)
	if err != nil {
		return err
	}

	// Copy static assets to _assets/ directory in the output
	if pipeline.StaticFS != nil {
		if err := b.copyStaticAssets(pipeline.StaticFS, stats); err != nil {
			return fmt.Errorf("copy static assets: %w", err)
		}
	}

	// Generate SEO files (robots.txt and sitemap.xml)
	if err := b.generateSEOFiles(prov, &cfg.Site); err != nil {
		return fmt.Errorf("generate SEO files: %w", err)
	}

	// Generate 404.html for static host compatibility (Netlify, GitHub Pages, Cloudflare Pages)
	errorContent, err := b.renderErrorPage(http.StatusNotFound, pipeline.TemplateRenderer, &cfg.Site)
	if err != nil {
		return fmt.Errorf("render 404 page: %w", err)
	}
	if err := b.writeOutputFile("404.html", errorContent); err != nil {
		return fmt.Errorf("write 404.html: %w", err)
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

// guardOutputDir checks whether the output directory already exists.
// If it does and --force is not set, an error is returned.
// If --force is set, the existing directory is removed for a clean build.
func (b *BuildCmd) guardOutputDir() error {
	info, err := os.Stat(b.Output)
	if err != nil {
		return nil // does not exist — nothing to guard
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %q exists but is not a directory", b.Output)
	}
	if !b.Force {
		return fmt.Errorf("output directory %q already exists; use --force to overwrite", b.Output)
	}
	if err := os.RemoveAll(b.Output); err != nil {
		return fmt.Errorf("remove existing output directory: %w", err)
	}
	slog.Info("Removed existing output directory", slog.String("path", b.Output))
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
	// Also track which directories contain an index.md so we know whether
	// DefaultIndex (e.g. README.md) should become index.html or keep its name.
	var filePaths []string
	dirsWithIndexMD := make(map[string]bool)
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
			if strings.EqualFold(name, "index.md") {
				dirsWithIndexMD[path.Dir(filePath)] = true
			}
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
			mimeType := negotiate.DetectMIME(fp)
			normalized := negotiate.NormalizeMimeType(mimeType)

			contentRenderer, _, err := registry.Get(normalized, htmlAccept)
			if err != nil {
				// No HTML renderer for this type → copy as-is
				return b.copyFile(contentRoot, fp, stats)
			}
			return b.buildFile(ctx, contentRoot, fp, contentRenderer, enricherRegistry, normalized, templateRenderer, siteConfig, dirsWithIndexMD, stats)
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
	dirsWithIndexMD map[string]bool,
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

	templateName := tmpl.ResolveLayout(templateRenderer, metadata)
	rendered, err := templateRenderer.Render(ctx, templateName, templateCtx)
	if err != nil {
		return fmt.Errorf("template render %s: %w", filePath, err)
	}

	// Determine the output path. The DefaultIndex file (e.g. README.md)
	// becomes index.html unless an index.md exists in the same directory.
	htmlPath := strings.TrimSuffix(filePath, path.Ext(filePath)) + ".html"
	if b.isDefaultIndex(filePath, siteConfig.DefaultIndex) && !dirsWithIndexMD[path.Dir(filePath)] {
		htmlPath = path.Join(path.Dir(filePath), "index.html")
	}
	if err := b.writeOutputFile(htmlPath, rendered); err != nil {
		return err
	}
	stats.totalBytes.Add(int64(len(rendered)))

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

// copyStaticAssets walks the static filesystem and copies all files to the _assets/ output directory.
func (b *BuildCmd) copyStaticAssets(staticFS fs.FS, stats *buildStats) error {
	return fs.WalkDir(staticFS, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk static %s: %w", filePath, err)
		}
		if d.IsDir() {
			return nil
		}

		content, readErr := fs.ReadFile(staticFS, filePath)
		if readErr != nil {
			return fmt.Errorf("read static %s: %w", filePath, readErr)
		}

		outPath := path.Join("_assets", filePath)
		if writeErr := b.writeOutputFile(outPath, content); writeErr != nil {
			return writeErr
		}

		stats.copiedFiles.Add(1)
		stats.totalBytes.Add(int64(len(content)))
		slog.Debug("Copied static asset", slog.String("file", outPath))
		return nil
	})
}

// generateSEOFiles generates robots.txt and optionally sitemap.xml in the output directory.
func (b *BuildCmd) generateSEOFiles(prov provider.Provider, siteConfig *config.SiteConfig) error {
	// Always generate robots.txt
	robotsTxt := server.GenerateRobotsTxt(siteConfig.Meta.Domain)
	if err := b.writeOutputFile("robots.txt", []byte(robotsTxt)); err != nil {
		return fmt.Errorf("write robots.txt: %w", err)
	}
	slog.Debug("Generated", slog.String("file", "robots.txt"))

	// Generate sitemap.xml only if domain is configured
	if siteConfig.Meta.Domain != "" {
		contentRoot, err := prov.RootFS(context.Background())
		if err != nil {
			return fmt.Errorf("get content root for sitemap: %w", err)
		}

		idx, err := metadata.BuildIndex(context.Background(), contentRoot)
		if err != nil {
			return fmt.Errorf("build metadata index for sitemap: %w", err)
		}

		sitemapData, err := server.GenerateSitemap(context.Background(), idx, siteConfig.Meta.Domain, siteConfig.DefaultIndex, prov)
		if err != nil {
			return fmt.Errorf("generate sitemap: %w", err)
		}

		if err := b.writeOutputFile("sitemap.xml", sitemapData); err != nil {
			return fmt.Errorf("write sitemap.xml: %w", err)
		}
		slog.Debug("Generated", slog.String("file", "sitemap.xml"))

		feedData, err := server.GenerateFeed(context.Background(), idx, siteConfig.Meta.Domain, siteConfig.DefaultIndex, prov, siteConfig.Meta.Title)
		if err != nil {
			return fmt.Errorf("generate feed: %w", err)
		}

		if err := b.writeOutputFile("feed.xml", feedData); err != nil {
			return fmt.Errorf("write feed.xml: %w", err)
		}
		slog.Debug("Generated", slog.String("file", "feed.xml"))
	}

	return nil
}

// isDefaultIndex reports whether filePath's basename matches the configured
// DefaultIndex filename (case-insensitive). For example, "docs/README.md"
// matches DefaultIndex "README.md".
func (b *BuildCmd) isDefaultIndex(filePath, defaultIndex string) bool {
	return strings.EqualFold(path.Base(filePath), defaultIndex)
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

// renderErrorPage renders a 404 page through the template engine.
func (b *BuildCmd) renderErrorPage(statusCode int, templateRenderer tmpl.Renderer, siteConfig *config.SiteConfig) ([]byte, error) {
	statusTitle := http.StatusText(statusCode)

	ctx := &tmpl.TemplateContext{
		Site: siteConfig,
		Page: tmpl.PageContext{
			Path: "/" + strconv.Itoa(statusCode),
			Meta: map[string]any{
				"title":         statusTitle,
				"robots":        "noindex",
				"error_code":    statusCode,
				"error_title":   statusTitle,
				"error_message": server.StatusMessage(statusCode),
			},
		},
	}

	return templateRenderer.Render(context.Background(), "error.html.tmpl", ctx)
}
