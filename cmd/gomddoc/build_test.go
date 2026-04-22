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
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
)

func TestBuildCmd_GeneratesHTMLFromMarkdown(t *testing.T) {
	binary := buildTestBinary(t)
	srcDir := t.TempDir()
	outDir := t.TempDir()

	writeTestFile(t, srcDir, "hello.md", "# Hello World\n\nThis is a test.")

	cmd := exec.Command(binary, "build", "--dir", srcDir, "--output", outDir)
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

	cmd := exec.Command(binary, "build", "--dir", srcDir, "--output", outDir)
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

	cmd := exec.Command(binary, "build", "--dir", srcDir, "--output", outDir)
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

	cmd := exec.Command(binary, "build", "--dir", srcDir, "--output", outDir)
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

	cmd := exec.Command(binary, "build", "--dir", srcDir, "--output", outDir)
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

	cmd := exec.Command(binary, "build", "--dir", srcDir, "--output", outDir)
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

	err := b.buildFile(context.Background(), contentRoot, "page.md", mdRenderer, templateRenderer, siteConfig, stats)
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

	err := b.buildFile(context.Background(), contentRoot, "README.md", mdRenderer, templateRenderer, siteConfig, stats)
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

	err := b.buildFile(context.Background(), contentRoot, "docs/README.md", mdRenderer, templateRenderer, siteConfig, stats)
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

	stats, err := b.walkAndBuild(contentRoot, registry, templateRenderer, siteConfig)
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

	stats, err := b.walkAndBuild(contentRoot, registry, templateRenderer, siteConfig)
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
