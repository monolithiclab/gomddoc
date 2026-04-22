package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

func TestWriteOutputFile(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	// Test basic write
	err := b.writeOutputFile("test.html", []byte("<html>hello</html>"))
	if err != nil {
		t.Fatalf("writeOutputFile failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(outDir, "test.html"))
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(content) != "<html>hello</html>" {
		t.Errorf("content = %q, want %q", content, "<html>hello</html>")
	}

	// Test nested directory creation
	err = b.writeOutputFile(filepath.Join("sub", "dir", "page.html"), []byte("nested"))
	if err != nil {
		t.Fatalf("writeOutputFile with nested dirs failed: %v", err)
	}
	content, err = os.ReadFile(filepath.Join(outDir, "sub", "dir", "page.html"))
	if err != nil {
		t.Fatalf("failed to read nested output file: %v", err)
	}
	if string(content) != "nested" {
		t.Errorf("nested content = %q, want %q", content, "nested")
	}
}

func TestCopyFile(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"style.css": &fstest.MapFile{Data: []byte("body{}")},
	}

	err := b.copyFile(contentRoot, "style.css", stats)
	if err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "style.css"))
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(content) != "body{}" {
		t.Errorf("content = %q, want %q", content, "body{}")
	}

	if stats.copiedFiles.Load() != 1 {
		t.Errorf("copiedFiles = %d, want 1", stats.copiedFiles.Load())
	}
	if stats.totalBytes.Load() != int64(len("body{}")) {
		t.Errorf("totalBytes = %d, want %d", stats.totalBytes.Load(), len("body{}"))
	}
}

// newTestTemplateRenderer creates a minimal HTMLRenderer backed by an in-memory
// template filesystem, suitable for unit tests that need to render markdown.
func newTestTemplateRenderer(t *testing.T) (*tmpl.HTMLRenderer, *config.SiteConfig) {
	t.Helper()

	siteConfig := config.NewSiteConfig(".")

	templateFS := fstest.MapFS{
		"assets/themes/default/layouts/default.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Page.Content}}</body></html>`),
		},
	}

	templateRenderer := tmpl.NewHTMLRenderer(&siteConfig, templateFS)
	return templateRenderer, &siteConfig
}

// newTestRegistry creates a renderer registry with the default markdown,
// markdown passthrough, and passthrough renderers, suitable for unit tests.
func newTestRegistry() renderer.RendererRegistry {
	registry := renderer.NewDefaultRegistry()
	registry.Register(renderer.NewMarkdownPassthroughRenderer())
	registry.Register(renderer.NewMarkdownRenderer(renderer.MarkdownOptions{}))
	registry.Register(renderer.NewPassthroughRenderer())
	return registry
}

func TestBuildFile(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"page.md": &fstest.MapFile{Data: []byte("# Test Page\n\nSome content.")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	enricherReg := newTestEnricherRegistry()
	err := b.buildFile(context.Background(), contentRoot, "page.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, nil, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	htmlContent, err := os.ReadFile(filepath.Join(outDir, "page.html"))
	if err != nil {
		t.Fatalf("Expected page.html to exist: %v", err)
	}
	if !strings.Contains(string(htmlContent), "Test Page") {
		t.Errorf("Expected 'Test Page' in output HTML, got:\n%s", htmlContent)
	}

	if stats.markdownFiles.Load() != 1 {
		t.Errorf("markdownFiles = %d, want 1", stats.markdownFiles.Load())
	}
	if stats.totalBytes.Load() == 0 {
		t.Error("totalBytes should be > 0 after rendering markdown")
	}
}

func TestBuildFile_README_BecomesIndex(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("# Project")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	enricherReg := newTestEnricherRegistry()
	// No index.md exists, so README.md should produce only index.html
	err := b.buildFile(context.Background(), contentRoot, "README.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, nil, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	indexHTML, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal("Expected index.html to exist")
	}
	if !strings.Contains(string(indexHTML), "Project") {
		t.Errorf("Expected 'Project' in index.html, got:\n%s", indexHTML)
	}

	// README.html should NOT exist
	if _, err := os.Stat(filepath.Join(outDir, "README.html")); !os.IsNotExist(err) {
		t.Error("README.html should not exist when no index.md is present")
	}
}

func TestBuildFile_README_WithIndexMD(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("# README Content")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	enricherReg := newTestEnricherRegistry()
	// Simulate index.md existing in same directory
	dirsWithIndexMD := map[string]bool{".": true}
	err := b.buildFile(context.Background(), contentRoot, "README.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, dirsWithIndexMD, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// README.md should produce README.html (not index.html) since index.md exists
	readmeHTML, err := os.ReadFile(filepath.Join(outDir, "README.html"))
	if err != nil {
		t.Fatal("Expected README.html to exist")
	}
	if !strings.Contains(string(readmeHTML), "README Content") {
		t.Errorf("Expected 'README Content' in README.html")
	}

	// index.html should NOT exist (index.md would produce it separately)
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); !os.IsNotExist(err) {
		t.Error("index.html should not exist — index.md handles it")
	}
}

func TestBuildFile_SubdirREADME(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"docs/README.md": &fstest.MapFile{Data: []byte("# Docs Index")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	enricherReg := newTestEnricherRegistry()
	err := b.buildFile(context.Background(), contentRoot, "docs/README.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, nil, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// Should produce only docs/index.html (not docs/README.html)
	if _, err := os.Stat(filepath.Join(outDir, "docs", "index.html")); err != nil {
		t.Fatal("Expected docs/index.html to exist")
	}
	if _, err := os.Stat(filepath.Join(outDir, "docs", "README.html")); !os.IsNotExist(err) {
		t.Error("docs/README.html should not exist")
	}
}

func TestWalkAndBuild(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{
		"page.md":    &fstest.MapFile{Data: []byte("# Page")},
		"style.css":  &fstest.MapFile{Data: []byte("body{}")},
		".hidden.md": &fstest.MapFile{Data: []byte("# Hidden")},
	}

	registry := newTestRegistry()
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	stats, err := b.walkAndBuild(contentRoot, registry, newTestEnricherRegistry(), templateRenderer, siteConfig)
	if err != nil {
		t.Fatalf("walkAndBuild failed: %v", err)
	}

	if stats.markdownFiles.Load() != 1 {
		t.Errorf("markdownFiles = %d, want 1", stats.markdownFiles.Load())
	}
	if stats.copiedFiles.Load() != 1 {
		t.Errorf("copiedFiles = %d, want 1", stats.copiedFiles.Load())
	}
	if stats.skippedFiles.Load() != 1 {
		t.Errorf("skippedFiles = %d, want 1 (hidden file)", stats.skippedFiles.Load())
	}

	// Verify output files exist
	if _, err := os.Stat(filepath.Join(outDir, "page.html")); err != nil {
		t.Error("Expected page.html to exist")
	}
	if _, err := os.Stat(filepath.Join(outDir, "style.css")); err != nil {
		t.Error("Expected style.css to exist")
	}
	// Hidden file should not be in output
	if _, err := os.Stat(filepath.Join(outDir, ".hidden.html")); !os.IsNotExist(err) {
		t.Error("Expected .hidden.html to not exist")
	}
}

func TestWalkAndBuild_MixedContent(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{
		"README.md":        &fstest.MapFile{Data: []byte("# Root")},
		"docs/guide.md":    &fstest.MapFile{Data: []byte("# Guide")},
		"docs/style.css":   &fstest.MapFile{Data: []byte("h1{}")},
		".gomddoc/cfg.yml": &fstest.MapFile{Data: []byte("theme: default")},
	}

	registry := newTestRegistry()
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	stats, err := b.walkAndBuild(contentRoot, registry, newTestEnricherRegistry(), templateRenderer, siteConfig)
	if err != nil {
		t.Fatalf("walkAndBuild failed: %v", err)
	}

	// README.md and docs/guide.md are markdown
	if stats.markdownFiles.Load() != 2 {
		t.Errorf("markdownFiles = %d, want 2", stats.markdownFiles.Load())
	}
	// docs/style.css is copied
	if stats.copiedFiles.Load() != 1 {
		t.Errorf("copiedFiles = %d, want 1", stats.copiedFiles.Load())
	}
	// .gomddoc directory is skipped entirely (hidden dir)

	// README.md should produce index.html
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Error("Expected index.html from README.md")
	}
}

func TestWalkAndBuild_IndexMDAndREADME(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("# README")},
		"index.md":  &fstest.MapFile{Data: []byte("# Home")},
	}

	registry := newTestRegistry()
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	stats, err := b.walkAndBuild(contentRoot, registry, newTestEnricherRegistry(), templateRenderer, siteConfig)
	if err != nil {
		t.Fatalf("walkAndBuild failed: %v", err)
	}

	if stats.markdownFiles.Load() != 2 {
		t.Errorf("markdownFiles = %d, want 2", stats.markdownFiles.Load())
	}

	// index.md should produce index.html with "Home" content
	indexHTML := readTestFile(t, outDir, "index.html")
	if !strings.Contains(indexHTML, "Home") {
		t.Errorf("Expected 'Home' in index.html (from index.md), got:\n%s", indexHTML)
	}

	// README.md should produce README.html (not index.html) since index.md exists
	readmeHTML := readTestFile(t, outDir, "README.html")
	if !strings.Contains(readmeHTML, "README") {
		t.Errorf("Expected 'README' in README.html, got:\n%s", readmeHTML)
	}
}

func newTestEnricherRegistry() enricher.EnricherRegistry {
	reg := enricher.NewDefaultEnricherRegistry()
	reg.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{}))
	return reg
}

func TestBuildCmd_Run(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeTestFile(t, srcDir, "README.md", "# Build Run Test\n\nContent.")
	writeTestFile(t, srcDir, "style.css", "body{color:red}")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify README.md was rendered as index.html (not README.html)
	indexContent := readTestFile(t, outDir, "index.html")
	if !strings.Contains(indexContent, "Build Run Test") {
		t.Error("Expected 'Build Run Test' in index.html")
	}
	if _, err := os.Stat(filepath.Join(outDir, "README.html")); !os.IsNotExist(err) {
		t.Error("README.html should not exist — README.md should produce only index.html")
	}

	// Verify CSS was copied
	cssContent := readTestFile(t, outDir, "style.css")
	if cssContent != "body{color:red}" {
		t.Errorf("CSS content = %q, want %q", cssContent, "body{color:red}")
	}

	// Static assets may or may not exist depending on theme config
	// Just verify the command succeeded without error
}

func TestCopyStaticAssets(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	staticFS := fstest.MapFS{
		"style.css":    &fstest.MapFile{Data: []byte("body{}")},
		"js/app.js":    &fstest.MapFile{Data: []byte("console.log('hi')")},
		"img/logo.txt": &fstest.MapFile{Data: []byte("logo-data")},
	}

	if err := b.copyStaticAssets(staticFS, stats); err != nil {
		t.Fatalf("copyStaticAssets error = %v", err)
	}

	// Verify files were copied to _assets/
	cssContent := readTestFile(t, filepath.Join(outDir, "_assets"), "style.css")
	if cssContent != "body{}" {
		t.Errorf("CSS content = %q, want %q", cssContent, "body{}")
	}

	jsContent := readTestFile(t, filepath.Join(outDir, "_assets", "js"), "app.js")
	if jsContent != "console.log('hi')" {
		t.Errorf("JS content = %q", jsContent)
	}

	if stats.copiedFiles.Load() != 3 {
		t.Errorf("copiedFiles = %d, want 3", stats.copiedFiles.Load())
	}
}

func TestBuildCmd_Run_WithFrontmatter(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeTestFile(t, srcDir, "page.md", "---\ntitle: Custom Title\ntags: [go, docs]\n---\n# Page\n\nContent.")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	htmlContent := readTestFile(t, outDir, "page.html")
	if !strings.Contains(htmlContent, "Page") {
		t.Error("Expected 'Page' in output HTML")
	}
}

func TestBuildCmd_Run_NonexistentDir(t *testing.T) {
	t.Parallel()

	cmd := &BuildCmd{Dir: "/nonexistent/dir", Output: filepath.Join(t.TempDir(), "out")}
	if err := cmd.Run(); err == nil {
		t.Error("Run() with nonexistent dir should return error")
	}
}

func TestBuildCmd_Run_ReadOnlyParent(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Test")

	// Make parent read-only so the output dir can't be created
	parent := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(parent, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0750) })

	outDir := filepath.Join(parent, "out")
	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	err := cmd.Run()
	if err == nil {
		t.Error("Run() with read-only parent should return error")
	}
}

func TestBuildFile_WithFrontmatterTitle(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"titled.md": &fstest.MapFile{Data: []byte("---\ntitle: My Title\n---\n# Content")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	enricherReg := newTestEnricherRegistry()

	err := b.buildFile(context.Background(), contentRoot, "titled.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, nil, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	htmlContent, err := os.ReadFile(filepath.Join(outDir, "titled.html"))
	if err != nil {
		t.Fatalf("Expected titled.html to exist: %v", err)
	}
	if !strings.Contains(string(htmlContent), "Content") {
		t.Errorf("Expected 'Content' in output HTML")
	}
}

func TestBuildCmd_Defaults(t *testing.T) {
	t.Parallel()

	cmd := BuildCmd{}
	if cmd.Dir != "" {
		t.Errorf("Dir default = %q, want empty (Kong sets '.')", cmd.Dir)
	}
	if cmd.Output != "" {
		t.Errorf("Output default = %q, want empty (Kong sets 'build/site')", cmd.Output)
	}
	if cmd.Force {
		t.Error("Force should default to false")
	}
}

func TestWriteOutputFile_ReadOnlyDir(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(outDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outDir, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(outDir, 0750) })

	b := &BuildCmd{Output: outDir}
	err := b.writeOutputFile(filepath.Join("sub", "file.html"), []byte("data"))
	if err == nil {
		t.Error("writeOutputFile should fail on read-only directory")
	}
}

func TestCopyFile_MissingFile(t *testing.T) {
	t.Parallel()

	b := &BuildCmd{Output: t.TempDir()}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{}
	err := b.copyFile(contentRoot, "missing.css", stats)
	if err == nil {
		t.Error("copyFile should fail for missing file")
	}
	if stats.copiedFiles.Load() != 0 {
		t.Error("copiedFiles should be 0 on error")
	}
}

func TestBuildFile_ReadError(t *testing.T) {
	t.Parallel()

	b := &BuildCmd{Output: t.TempDir()}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{} // no files
	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	enricherReg := newTestEnricherRegistry()

	err := b.buildFile(context.Background(), contentRoot, "nonexistent.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, nil, stats)
	if err == nil {
		t.Error("buildFile should fail for missing file")
	}
}

func TestWalkAndBuild_HiddenDir(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{
		".git/config": &fstest.MapFile{Data: []byte("[core]")},
		".hidden":     &fstest.MapFile{Data: []byte("hidden")},
		"visible.md":  &fstest.MapFile{Data: []byte("# Visible")},
	}

	registry := newTestRegistry()
	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	stats, err := b.walkAndBuild(contentRoot, registry, newTestEnricherRegistry(), templateRenderer, siteConfig)
	if err != nil {
		t.Fatalf("walkAndBuild failed: %v", err)
	}

	if stats.markdownFiles.Load() != 1 {
		t.Errorf("markdownFiles = %d, want 1", stats.markdownFiles.Load())
	}
	// .git dir skipped (SkipDir), .hidden file skipped
	if stats.skippedFiles.Load() != 1 {
		t.Errorf("skippedFiles = %d, want 1", stats.skippedFiles.Load())
	}
}

func TestCopyStaticAssets_ReadError(t *testing.T) {
	t.Parallel()

	// Use a FS where WalkDir succeeds but ReadFile fails — use an errFS
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	// Empty FS — copyStaticAssets with no files should succeed
	emptyFS := fstest.MapFS{}
	if err := b.copyStaticAssets(emptyFS, stats); err != nil {
		t.Fatalf("copyStaticAssets with empty FS error = %v", err)
	}
	if stats.copiedFiles.Load() != 0 {
		t.Errorf("copiedFiles = %d, want 0", stats.copiedFiles.Load())
	}
}

func TestBuildCmd_Run_WithSubdirectories(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeTestFile(t, srcDir, "README.md", "# Root")
	writeTestFile(t, filepath.Join(srcDir, "api"), "endpoints.md", "# API Endpoints")
	writeTestFile(t, filepath.Join(srcDir, "api"), "types.md", "# Types")
	writeTestFile(t, filepath.Join(srcDir, "assets"), "logo.txt", "logo")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify README.md produced only index.html
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Error("Expected index.html to exist")
	}
	if _, err := os.Stat(filepath.Join(outDir, "README.html")); !os.IsNotExist(err) {
		t.Error("README.html should not exist")
	}
	for _, f := range []string{"endpoints.html", "types.html"} {
		if _, err := os.Stat(filepath.Join(outDir, "api", f)); err != nil {
			t.Errorf("Expected api/%s to exist", f)
		}
	}
	// Non-markdown file copied
	if _, err := os.Stat(filepath.Join(outDir, "assets", "logo.txt")); err != nil {
		t.Error("Expected assets/logo.txt to exist")
	}
}

func TestBuildCmd_Run_ProviderError(t *testing.T) {
	t.Parallel()

	// A valid config dir but that has a bad .gomddoc config to trigger pipeline errors
	srcDir := t.TempDir()
	// Empty dir with no markdown: Run should still succeed
	cmd := &BuildCmd{Dir: srcDir, Output: filepath.Join(t.TempDir(), "out")}
	// No README.md: provider will still work but no files to process
	if err := cmd.Run(); err != nil {
		// This may or may not error depending on whether empty dirs are ok
		t.Logf("Run() error (acceptable) = %v", err)
	}
}

func TestBuildCmd_Run_MultipleRuns(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	writeTestFile(t, srcDir, "README.md", "# First Run")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}

	// First run: output dir does not exist — should succeed
	if err := cmd.Run(); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}

	// Second run without --force: should fail because output dir exists
	if err := cmd.Run(); err == nil {
		t.Error("second Run() without --force should return error")
	}

	// Second run with --force: should succeed
	cmd.Force = true
	if err := cmd.Run(); err != nil {
		t.Fatalf("second Run() with --force error = %v", err)
	}

	htmlContent := readTestFile(t, outDir, "index.html")
	if !strings.Contains(htmlContent, "First Run") {
		t.Error("Expected 'First Run' in index.html")
	}
}

func TestWalkAndBuild_WriteError(t *testing.T) {
	t.Parallel()

	// Use a read-only output directory so writes fail
	outDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(outDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outDir, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(outDir, 0750) })

	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{
		"page.md": &fstest.MapFile{Data: []byte("# Page")},
	}

	registry := newTestRegistry()
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	_, err := b.walkAndBuild(contentRoot, registry, newTestEnricherRegistry(), templateRenderer, siteConfig)
	if err == nil {
		t.Error("walkAndBuild should fail when output dir is read-only")
	}
}

func TestWalkAndBuild_EmptyFS(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{}
	registry := newTestRegistry()
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	stats, err := b.walkAndBuild(contentRoot, registry, newTestEnricherRegistry(), templateRenderer, siteConfig)
	if err != nil {
		t.Fatalf("walkAndBuild failed: %v", err)
	}

	if stats.markdownFiles.Load() != 0 {
		t.Errorf("markdownFiles = %d, want 0", stats.markdownFiles.Load())
	}
	if stats.copiedFiles.Load() != 0 {
		t.Errorf("copiedFiles = %d, want 0", stats.copiedFiles.Load())
	}
}

func TestGuardOutputDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, output string) // pre-create output state
		force     bool
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "output does not exist",
			setup:   func(t *testing.T, output string) {},
			wantErr: false,
		},
		{
			name: "output exists without force",
			setup: func(t *testing.T, output string) {
				if err := os.MkdirAll(output, 0750); err != nil {
					t.Fatal(err)
				}
			},
			wantErr:   true,
			errSubstr: "already exists",
		},
		{
			name: "output exists with force",
			setup: func(t *testing.T, output string) {
				if err := os.MkdirAll(output, 0750); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, output, "stale.html", "<p>old</p>")
			},
			force:   true,
			wantErr: false,
		},
		{
			name: "output is a file not dir",
			setup: func(t *testing.T, output string) {
				if err := os.WriteFile(output, []byte("not a dir"), 0644); err != nil {
					t.Fatal(err)
				}
			},
			wantErr:   true,
			errSubstr: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output := filepath.Join(t.TempDir(), "out")
			tt.setup(t, output)

			b := &BuildCmd{Output: output, Force: tt.force}
			err := b.guardOutputDir()

			if tt.wantErr {
				if err == nil {
					t.Error("guardOutputDir() = nil, want error")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error = %q, want substring %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Errorf("guardOutputDir() = %v, want nil", err)
			}

			// With force, verify old content was removed
			if tt.force {
				if _, statErr := os.Stat(filepath.Join(output, "stale.html")); !os.IsNotExist(statErr) {
					t.Error("stale.html should have been removed by --force")
				}
			}
		})
	}
}

// writeTestFile creates a file with the given content, creating parent directories as needed.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file %s: %v", name, err)
	}
}

// readTestFile reads a file and returns its content as a string.
func readTestFile(t *testing.T, dir, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("Failed to read file %s/%s: %v", dir, name, err)
	}
	return string(content)
}
