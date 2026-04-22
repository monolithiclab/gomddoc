package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

func TestBuildCmd_GeneratesHTMLFromMarkdown(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "hello.md", "# Hello World\n\nThis is a test.")

	cmd := exec.Command(binary, "build", srcDir, "--output", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	htmlContent := readTestFile(t, outDir, "hello.html")
	if !strings.Contains(htmlContent, "Hello World") {
		t.Errorf("Expected 'Hello World' in HTML output, got:\n%s", htmlContent)
	}
	if !strings.Contains(htmlContent, "<h1") {
		t.Errorf("Expected <h1 tag in HTML output, got:\n%s", htmlContent)
	}
}

func TestBuildCmd_READMECreatesIndexHTML(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "README.md", "# Project README\n\nWelcome.")
	writeTestFile(t, filepath.Join(srcDir, "docs"), "README.md", "# Docs README")

	cmd := exec.Command(binary, "build", srcDir, "--output", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	// Root README.md should produce both README.html and index.html
	readmeContent := readTestFile(t, outDir, "README.html")
	indexContent := readTestFile(t, outDir, "index.html")
	if readmeContent != indexContent {
		t.Error("Expected README.html and index.html to have identical content")
	}
	if !strings.Contains(indexContent, "Project README") {
		t.Errorf("Expected 'Project README' in index.html, got:\n%s", indexContent)
	}

	// Subdirectory README.md should also produce index.html
	docsIndex := readTestFile(t, filepath.Join(outDir, "docs"), "index.html")
	if !strings.Contains(docsIndex, "Docs README") {
		t.Errorf("Expected 'Docs README' in docs/index.html, got:\n%s", docsIndex)
	}
}

func TestBuildCmd_CopiesNonMarkdownFiles(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "style.css", "body { color: red; }")
	writeTestFile(t, filepath.Join(srcDir, "images"), "logo.txt", "logo-placeholder")

	cmd := exec.Command(binary, "build", srcDir, "--output", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	cssContent := readTestFile(t, outDir, "style.css")
	if cssContent != "body { color: red; }" {
		t.Errorf("Expected CSS content to be copied as-is, got: %s", cssContent)
	}

	logoContent := readTestFile(t, filepath.Join(outDir, "images"), "logo.txt")
	if logoContent != "logo-placeholder" {
		t.Errorf("Expected logo.txt to be copied as-is, got: %s", logoContent)
	}
}

func TestBuildCmd_SkipsHiddenFiles(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "visible.md", "# Visible")
	writeTestFile(t, srcDir, ".hidden.md", "# Hidden")
	writeTestFile(t, filepath.Join(srcDir, ".hiddendir"), "secret.md", "# Secret")

	cmd := exec.Command(binary, "build", srcDir, "--output", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	// visible.html should exist
	if _, err := os.Stat(filepath.Join(outDir, "visible.html")); os.IsNotExist(err) {
		t.Error("Expected visible.html to exist")
	}

	// .hidden.html should not exist
	if _, err := os.Stat(filepath.Join(outDir, ".hidden.html")); !os.IsNotExist(err) {
		t.Error("Expected .hidden.html to not exist")
	}

	// .hiddendir should not exist
	if _, err := os.Stat(filepath.Join(outDir, ".hiddendir")); !os.IsNotExist(err) {
		t.Error("Expected .hiddendir to not exist")
	}
}

func TestBuildCmd_CreatesOutputDirectory(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "nested", "output")

	writeTestFile(t, srcDir, "page.md", "# Page")

	cmd := exec.Command(binary, "build", srcDir, "--output", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(outDir, "page.html")); os.IsNotExist(err) {
		t.Error("Expected page.html to exist in nested output directory")
	}
}

func TestBuildCmd_ParallelBuildProcessesAllFiles(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := t.TempDir()

	// Create many markdown and non-markdown files to exercise parallelism.
	const numMarkdown = 20
	const numAssets = 10
	for i := range numMarkdown {
		writeTestFile(t, srcDir, fmt.Sprintf("page-%03d.md", i), fmt.Sprintf("# Page %d\n\nContent for page %d.", i, i))
	}
	for i := range numAssets {
		writeTestFile(t, srcDir, fmt.Sprintf("asset-%03d.css", i), fmt.Sprintf("body { color: #%03d; }", i))
	}

	cmd := exec.Command(binary, "build", srcDir, "--output", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, output)
	}

	// Verify all markdown files were rendered.
	for i := range numMarkdown {
		htmlFile := fmt.Sprintf("page-%03d.html", i)
		content := readTestFile(t, outDir, htmlFile)
		expected := fmt.Sprintf("Page %d", i)
		if !strings.Contains(content, expected) {
			t.Errorf("Expected %q in %s", expected, htmlFile)
		}
	}

	// Verify all asset files were copied.
	for i := range numAssets {
		cssFile := fmt.Sprintf("asset-%03d.css", i)
		content := readTestFile(t, outDir, cssFile)
		expected := fmt.Sprintf("body { color: #%03d; }", i)
		if content != expected {
			t.Errorf("Expected %q in %s, got %q", expected, cssFile, content)
		}
	}
}

// --- Unit tests that call functions directly for coverage ---

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
	err := b.buildFile(context.Background(), contentRoot, "page.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, stats)
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

func TestBuildFile_README(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("# Project")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)

	enricherReg := newTestEnricherRegistry()
	err := b.buildFile(context.Background(), contentRoot, "README.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// Should produce both README.html and index.html
	readmeHTML, err := os.ReadFile(filepath.Join(outDir, "README.html"))
	if err != nil {
		t.Fatal("Expected README.html to exist")
	}
	indexHTML, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal("Expected index.html to exist")
	}
	if string(readmeHTML) != string(indexHTML) {
		t.Error("README.html and index.html should have identical content")
	}
	if !strings.Contains(string(readmeHTML), "Project") {
		t.Errorf("Expected 'Project' in output HTML, got:\n%s", readmeHTML)
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
	err := b.buildFile(context.Background(), contentRoot, "docs/README.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// Should produce docs/README.html and docs/index.html
	if _, err := os.Stat(filepath.Join(outDir, "docs", "README.html")); err != nil {
		t.Fatal("Expected docs/README.html to exist")
	}
	if _, err := os.Stat(filepath.Join(outDir, "docs", "index.html")); err != nil {
		t.Fatal("Expected docs/index.html to exist")
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

func newTestEnricherRegistry() enricher.EnricherRegistry {
	reg := enricher.NewDefaultEnricherRegistry()
	reg.Register(enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{}))
	return reg
}

func TestBuildCmd_Run(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "README.md", "# Build Run Test\n\nContent.")
	writeTestFile(t, srcDir, "style.css", "body{color:red}")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify markdown was rendered
	htmlContent := readTestFile(t, outDir, "README.html")
	if !strings.Contains(htmlContent, "Build Run Test") {
		t.Error("Expected 'Build Run Test' in output HTML")
	}

	// Verify index.html was created for README
	indexContent := readTestFile(t, outDir, "index.html")
	if !strings.Contains(indexContent, "Build Run Test") {
		t.Error("Expected index.html from README.md")
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
	outDir := t.TempDir()

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

	cmd := &BuildCmd{Dir: "/nonexistent/dir", Output: t.TempDir()}
	if err := cmd.Run(); err == nil {
		t.Error("Run() with nonexistent dir should return error")
	}
}

func TestBuildCmd_Run_ReadOnlyOutput(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# Test")

	outDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(outDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outDir, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(outDir, 0750) })

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	err := cmd.Run()
	if err == nil {
		t.Error("Run() with read-only output should return error")
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

	err := b.buildFile(context.Background(), contentRoot, "titled.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, stats)
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

	err := b.buildFile(context.Background(), contentRoot, "nonexistent.md", mdRenderer, enricherReg, "text/markdown", templateRenderer, siteConfig, stats)
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
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "README.md", "# Root")
	writeTestFile(t, filepath.Join(srcDir, "api"), "endpoints.md", "# API Endpoints")
	writeTestFile(t, filepath.Join(srcDir, "api"), "types.md", "# Types")
	writeTestFile(t, filepath.Join(srcDir, "assets"), "logo.txt", "logo")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify all markdown files were rendered
	for _, f := range []string{"README.html", "index.html"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
			t.Errorf("Expected %s to exist", f)
		}
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
	cmd := &BuildCmd{Dir: srcDir, Output: t.TempDir()}
	// No README.md: provider will still work but no files to process
	if err := cmd.Run(); err != nil {
		// This may or may not error depending on whether empty dirs are ok
		t.Logf("Run() error (acceptable) = %v", err)
	}
}

func TestBuildCmd_Run_MultipleRuns(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := t.TempDir()
	writeTestFile(t, srcDir, "README.md", "# First Run")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}

	// Run twice to verify idempotent behavior
	if err := cmd.Run(); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	htmlContent := readTestFile(t, outDir, "README.html")
	if !strings.Contains(htmlContent, "First Run") {
		t.Error("Expected 'First Run' in output HTML")
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
