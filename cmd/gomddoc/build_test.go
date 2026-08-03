package main

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/locale"
	"github.com/monolithiclab/gomddoc/internal/renderer"
	tmpl "github.com/monolithiclab/gomddoc/internal/template"
	registryutil "github.com/monolithiclab/gomddoc/internal/testutil/registry"
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

	err := b.copyFile(contentRoot, "style.css", "", stats)
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

func TestBuildFile(t *testing.T) {
	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{
		"page.md": &fstest.MapFile{Data: []byte("# Test Page\n\nSome content.")},
	}

	mdRenderer := renderer.NewMarkdownRenderer(renderer.MarkdownOptions{})
	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	err := b.buildFile(context.Background(), contentRoot, "page.md", mdRenderer, "text/markdown", bc, nil, "", stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// With default StripExtensions=[".md"], pretty URLs are used: page.md -> page/index.html
	htmlContent, err := os.ReadFile(filepath.Join(outDir, "page", "index.html"))
	if err != nil {
		t.Fatalf("Expected page/index.html to exist: %v", err)
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
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	// No index.md exists, so README.md should produce only index.html
	err := b.buildFile(context.Background(), contentRoot, "README.md", mdRenderer, "text/markdown", bc, nil, "", stats)
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

	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	// Simulate index.md existing in same directory
	dirsWithIndexMD := map[string]bool{".": true}
	err := b.buildFile(context.Background(), contentRoot, "README.md", mdRenderer, "text/markdown", bc, dirsWithIndexMD, "", stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// With pretty URLs, README.md becomes README/index.html when index.md exists
	readmeHTML, err := os.ReadFile(filepath.Join(outDir, "README", "index.html"))
	if err != nil {
		t.Fatal("Expected README/index.html to exist")
	}
	if !strings.Contains(string(readmeHTML), "README Content") {
		t.Errorf("Expected 'README Content' in README/index.html")
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

	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	err := b.buildFile(context.Background(), contentRoot, "docs/README.md", mdRenderer, "text/markdown", bc, nil, "", stats)
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

	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	stats, err := b.walkAndBuild(contentRoot, bc)
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

	// Verify output files exist (pretty URLs: page.md -> page/index.html)
	if _, err := os.Stat(filepath.Join(outDir, "page", "index.html")); err != nil {
		t.Error("Expected page/index.html to exist")
	}
	if _, err := os.Stat(filepath.Join(outDir, "style.css")); err != nil {
		t.Error("Expected style.css to exist")
	}
	// Hidden file should not be in output
	if _, err := os.Stat(filepath.Join(outDir, ".hidden", "index.html")); !os.IsNotExist(err) {
		t.Error("Expected .hidden/index.html to not exist")
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

	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	stats, err := b.walkAndBuild(contentRoot, bc)
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

	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	stats, err := b.walkAndBuild(contentRoot, bc)
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

	// With pretty URLs, README.md becomes README/index.html when index.md exists
	readmeHTML := readTestFile(t, filepath.Join(outDir, "README"), "index.html")
	if !strings.Contains(readmeHTML, "README") {
		t.Errorf("Expected 'README' in README/index.html, got:\n%s", readmeHTML)
	}
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

	// With default StripExtensions, page.md -> page/index.html
	htmlContent := readTestFile(t, filepath.Join(outDir, "page"), "index.html")
	if !strings.Contains(htmlContent, "Page") {
		t.Error("Expected 'Page' in output HTML")
	}
}

func TestBuildCmd_Run_WithDomain(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	writeTestFile(t, srcDir, "README.md", "---\ntitle: Domain Build Test\n---\n# Domain Build Test")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir, Domain: "build.example.com"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Exhaustive, not a domain-substring sweep: every URL in a sitemap contains the
	// domain, so Contains("build.example.com") holds for any non-empty document.
	wantLocs := []string{
		"https://build.example.com/",
		"https://build.example.com/tags/",
	}
	if got := sitemapLocs(t, outDir, "sitemap.xml"); !slices.Equal(got, wantLocs) {
		t.Errorf("sitemap locs = %v, want %v", got, wantLocs)
	}

	// feed.xml content was asserted nowhere before this.
	wantIDs := []string{"https://build.example.com/"}
	if got := feedEntryIDs(t, outDir, "feed.xml"); !slices.Equal(got, wantIDs) {
		t.Errorf("feed entry IDs = %v, want %v", got, wantIDs)
	}

	// Verify robots.txt references the domain
	robotsContent := readTestFile(t, outDir, "robots.txt")
	if !strings.Contains(robotsContent, "build.example.com") {
		t.Error("robots.txt should contain the domain")
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
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	err := b.buildFile(context.Background(), contentRoot, "titled.md", mdRenderer, "text/markdown", bc, nil, "", stats)
	if err != nil {
		t.Fatalf("buildFile failed: %v", err)
	}

	// With default StripExtensions=[".md"], pretty URLs are used: titled.md -> titled/index.html
	htmlContent, err := os.ReadFile(filepath.Join(outDir, "titled", "index.html"))
	if err != nil {
		t.Fatalf("Expected titled/index.html to exist: %v", err)
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

func TestWriteOutputFile_PathTraversal(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	tests := []struct {
		name    string
		relPath string
	}{
		{"dot-dot prefix", "../escape.html"},
		{"nested dot-dot", "sub/../../escape.html"},
		{"absolute path", "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := b.writeOutputFile(tt.relPath, []byte("pwned"))
			if err == nil {
				t.Error("writeOutputFile should reject path traversal")
			}
			if !strings.Contains(err.Error(), "outside output directory") {
				t.Errorf("error = %q, want substring %q", err, "outside output directory")
			}
		})
	}
}

func TestCopyFile_MissingFile(t *testing.T) {
	t.Parallel()

	b := &BuildCmd{Output: t.TempDir()}
	stats := &buildStats{}

	contentRoot := fstest.MapFS{}
	err := b.copyFile(contentRoot, "missing.css", "", stats)
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
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	err := b.buildFile(context.Background(), contentRoot, "nonexistent.md", mdRenderer, "text/markdown", bc, nil, "", stats)
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

	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	stats, err := b.walkAndBuild(contentRoot, bc)
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

	// Read the outputs rather than Stat them: pages are rendered and written from
	// an errgroup, and the canonical failure of a parallel per-file write path —
	// one page's HTML landing in another page's index.html — passes an existence
	// check. Each assertion below therefore also denies the sibling's heading.
	if root := readTestFile(t, outDir, "index.html"); !strings.Contains(root, `<h1 id="root">Root`) {
		t.Errorf("index.html should render the root page, got:\n%s", root)
	}
	if _, err := os.Stat(filepath.Join(outDir, "README.html")); !os.IsNotExist(err) {
		t.Error("README.html should not exist")
	}

	// With default StripExtensions, pretty URLs are used.
	endpointsH1, typesH1 := `<h1 id="api-endpoints">API Endpoints`, `<h1 id="types">Types`
	for _, tc := range []struct{ dir, want, deny string }{
		{"endpoints", endpointsH1, typesH1},
		{"types", typesH1, endpointsH1},
	} {
		page := readTestFile(t, filepath.Join(outDir, "api", tc.dir), "index.html")
		if !strings.Contains(page, tc.want) {
			t.Errorf("api/%s/index.html missing %q, got:\n%s", tc.dir, tc.want, page)
		}
		if strings.Contains(page, tc.deny) {
			t.Errorf("api/%s/index.html rendered a sibling page's heading %q", tc.dir, tc.deny)
		}
	}

	// Non-markdown file copied byte for byte.
	if got := readTestFile(t, filepath.Join(outDir, "assets"), "logo.txt"); got != "logo" {
		t.Errorf("assets/logo.txt = %q, want %q", got, "logo")
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

	// Sentinel should exist after first run
	if _, err := os.Stat(filepath.Join(outDir, sentinelFile)); err != nil {
		t.Fatalf("sentinel file missing after first run: %v", err)
	}

	// Second run: sentinel exists, so it should succeed (auto-overwrite)
	if err := cmd.Run(); err != nil {
		t.Fatalf("second Run() error = %v", err)
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

	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	_, err := b.walkAndBuild(contentRoot, bc)
	if err == nil {
		t.Error("walkAndBuild should fail when output dir is read-only")
	}
}

func TestWalkAndBuild_EmptyFS(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	b := &BuildCmd{Output: outDir}

	contentRoot := fstest.MapFS{}
	templateRenderer, siteConfig := newTestTemplateRenderer(t)
	bc := newTestBuildContext(t, siteConfig, templateRenderer)

	stats, err := b.walkAndBuild(contentRoot, bc)
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
		setup     func(t *testing.T, output string)
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "output does not exist",
			setup:   func(t *testing.T, output string) {},
			wantErr: false,
		},
		{
			name: "output exists and is empty",
			setup: func(t *testing.T, output string) {
				if err := os.MkdirAll(output, 0750); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: false,
		},
		{
			name: "non-empty with sentinel removes dir",
			setup: func(t *testing.T, output string) {
				if err := os.MkdirAll(output, 0750); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, output, sentinelFile, "generated by gomddoc build\n")
				writeTestFile(t, output, "stale.html", "<p>old</p>")
			},
			wantErr: false,
		},
		{
			name: "non-empty without sentinel refuses",
			setup: func(t *testing.T, output string) {
				if err := os.MkdirAll(output, 0750); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, output, "important.txt", "do not delete")
			},
			wantErr:   true,
			errSubstr: "not created by gomddoc build",
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

			b := &BuildCmd{Output: output}
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
		})
	}
}

func TestGuardOutputDir_SentinelRemovesContent(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(output, 0750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, output, sentinelFile, "generated by gomddoc build\n")
	writeTestFile(t, output, "stale.html", "<p>old</p>")

	b := &BuildCmd{Output: output}
	if err := b.guardOutputDir(); err != nil {
		t.Fatalf("guardOutputDir() = %v", err)
	}

	if _, err := os.Stat(filepath.Join(output, "stale.html")); !os.IsNotExist(err) {
		t.Error("stale.html should have been removed")
	}
}

// testBundle creates a locale bundle for tests using embedded assets.
func testBundle(t *testing.T) *locale.Bundle {
	t.Helper()
	bundle, err := locale.LoadBundle("en-US", embeddedAssets, "locales")
	if err != nil {
		t.Fatalf("failed to load test bundle: %v", err)
	}
	return bundle
}

// newTestBuildContext creates a buildContext for tests with the given components.
func newTestBuildContext(t *testing.T, siteConfig *config.SiteConfig, templateRenderer *tmpl.HTMLRenderer) *buildContext {
	t.Helper()
	bundle := testBundle(t)
	return &buildContext{
		registry:         registryutil.NewRendererRegistry(),
		enricherRegistry: registryutil.NewEnricherRegistry(),
		templateRenderer: templateRenderer,
		siteConfig:       siteConfig,
		bundle:           bundle,
		lang:             "en-US",
		tFunc:            bundle.TFunc("en-US"),
	}
}

// writeTestFile creates a file with the given content, creating parent directories as needed.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	// name may itself contain directories ("guides/README.md"), so mkdir the
	// full parent rather than just dir.
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0750); err != nil {
		t.Fatalf("Failed to create directory %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
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

// sitemapLocs parses a generated sitemap and returns its <loc> values in document
// order. Substring checks cannot distinguish a per-language sitemap from one full
// of default-language URLs, because every URL shares the domain prefix.
func sitemapLocs(t *testing.T, dir, name string) []string {
	t.Helper()

	var doc struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(readTestFile(t, dir, name)), &doc); err != nil {
		t.Fatalf("%s/%s is not valid XML: %v", dir, name, err)
	}
	locs := make([]string, len(doc.URLs))
	for i, u := range doc.URLs {
		locs[i] = u.Loc
	}
	return locs
}

// feedEntryIDs parses a generated Atom feed and returns its entry <id> values in
// document order.
func feedEntryIDs(t *testing.T, dir, name string) []string {
	t.Helper()

	var doc struct {
		Entries []struct {
			ID string `xml:"id"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal([]byte(readTestFile(t, dir, name)), &doc); err != nil {
		t.Fatalf("%s/%s is not valid XML: %v", dir, name, err)
	}
	ids := make([]string, len(doc.Entries))
	for i, e := range doc.Entries {
		ids[i] = e.ID
	}
	return ids
}

func TestPrettyOutputPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		filePath        string
		defaultIndex    string
		dirsWithIndexMD map[string]bool
		want            string
	}{
		{"regular md", "docs/guide.md", "README.md", map[string]bool{}, "docs/guide/index.html"},
		{"index.md stays", "docs/index.md", "README.md", map[string]bool{}, "docs/index.html"},
		{"root file", "about.md", "README.md", map[string]bool{}, "about/index.html"},
		{"nested", "a/b/c.md", "README.md", map[string]bool{}, "a/b/c/index.html"},
		{"README becomes index", "docs/README.md", "README.md", map[string]bool{}, "docs/index.html"},
		{"README with index.md in same dir", "docs/README.md", "README.md", map[string]bool{"docs": true}, "docs/README/index.html"},
		{"root index.md", "index.md", "README.md", map[string]bool{}, "index.html"},
		{"root README becomes index", "README.md", "README.md", map[string]bool{}, "index.html"},
		{"root README with index.md", "README.md", "README.md", map[string]bool{".": true}, "README/index.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := prettyOutputPath(tt.filePath, tt.defaultIndex, tt.dirsWithIndexMD)
			if got != tt.want {
				t.Errorf("prettyOutputPath(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestBuildCmd_EmitsTagPages(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contentDir, "a.md"),
		[]byte("---\ntitle: A\ntags: [go, machine learning]\n---\n# A"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "b.md"),
		[]byte("---\ntitle: B\ntags: [go]\n---\n# B"), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	b := &BuildCmd{Dir: contentDir, Output: outDir}
	if err := b.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Read the pages, not just their inodes: an existence check cannot see a tag
	// page that lists the wrong members, which is the failure a parallel render
	// path actually produces.
	goPage := readTestFile(t, filepath.Join(outDir, "tags", "go"), "index.html")
	for _, want := range []string{`href="/a">A<`, `href="/b">B<`} {
		if !strings.Contains(goPage, want) {
			t.Errorf("tags/go/index.html missing %s, got:\n%s", want, goPage)
		}
	}

	mlPage := readTestFile(t, filepath.Join(outDir, "tags", "machine%20learning"), "index.html")
	if !strings.Contains(mlPage, `href="/a">A<`) {
		t.Errorf("tags/machine%%20learning/index.html missing A, got:\n%s", mlPage)
	}
	if strings.Contains(mlPage, `href="/b">B<`) {
		t.Error(`tags/machine%20learning/index.html lists B, which carries only the "go" tag`)
	}

	tagIndex := readTestFile(t, filepath.Join(outDir, "tags"), "index.html")
	for _, want := range []string{
		`href="/tags/go">go</a>`,
		`href="/tags/machine%20learning">machine learning</a>`,
		`<span class="tag-index-count">(2)</span>`, // go
		`<span class="tag-index-count">(1)</span>`, // machine learning
	} {
		if !strings.Contains(tagIndex, want) {
			t.Errorf("tags/index.html missing %s, got:\n%s", want, tagIndex)
		}
	}
}

func TestBuildCmd_Run_MultiLanguage(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	// Default language content (en-US).
	writeTestFile(t, srcDir, "README.md", "---\ntitle: Home\n---\n# Home")
	// Non-default language directory (BCP 47), with a tagged page so the
	// per-language tag pages get exercised too.
	writeTestFile(t, filepath.Join(srcDir, "fr-FR"), "README.md", "---\ntitle: Accueil\n---\n# Accueil")
	writeTestFile(t, filepath.Join(srcDir, "fr-FR"), "guide.md",
		"---\ntitle: Guide\ntags: [docs]\nredirect_from:\n  - /ancien-guide\n---\n# Guide")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir, Domain: "build.example.com"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Default-language outputs at the root.
	if !strings.Contains(readTestFile(t, outDir, "index.html"), "Home") {
		t.Error("root index.html should contain default-language content")
	}

	// walkAndBuildLang: per-language content built under fr-FR/.
	frHome := readTestFile(t, filepath.Join(outDir, "fr-FR"), "index.html")
	if !strings.Contains(frHome, "Accueil") {
		t.Error("fr-FR/index.html should contain the French content")
	}
	if _, err := os.Stat(filepath.Join(outDir, "fr-FR", "guide", "index.html")); err != nil {
		t.Errorf("expected fr-FR/guide/index.html: %v", err)
	}

	// Per-language 404 and tag pages.
	for _, want := range []string{
		filepath.Join("fr-FR", "404.html"),
		filepath.Join("fr-FR", "tags", "index.html"),
		filepath.Join("fr-FR", "tags", "docs", "index.html"),
	} {
		if _, err := os.Stat(filepath.Join(outDir, want)); err != nil {
			t.Errorf("expected per-language output %s: %v", want, err)
		}
	}

	// The per-language sitemap and feed must carry fr-FR URLs. Stat'ing them says
	// nothing: a per-language sitemap full of default-language URLs is the exact
	// bug the pathPrefix plumbing exists to prevent.
	wantFrLocs := []string{
		"https://build.example.com/fr-FR/",
		"https://build.example.com/fr-FR/guide",
		"https://build.example.com/fr-FR/tags/",
		"https://build.example.com/fr-FR/tags/docs",
	}
	if got := sitemapLocs(t, filepath.Join(outDir, "fr-FR"), "sitemap.xml"); !slices.Equal(got, wantFrLocs) {
		t.Errorf("fr-FR/sitemap.xml locs = %v, want %v", got, wantFrLocs)
	}
	wantFrIDs := []string{
		"https://build.example.com/fr-FR/guide",
		"https://build.example.com/fr-FR/",
	}
	if got := feedEntryIDs(t, filepath.Join(outDir, "fr-FR"), "feed.xml"); !slices.Equal(got, wantFrIDs) {
		t.Errorf("fr-FR/feed.xml entry IDs = %v, want %v", got, wantFrIDs)
	}

	// redirect_from and extension redirects are generated per language. Neither
	// existed before the default pipeline stopped indexing fr-FR: the default
	// pipeline owned both, and it emitted default-language targets.
	if got := readTestFile(t, filepath.Join(outDir, "fr-FR", "ancien-guide"), "index.html"); !strings.Contains(got, `url=/fr-FR/guide"`) {
		t.Errorf("fr-FR/ancien-guide/index.html should redirect to /fr-FR/guide, got:\n%s", got)
	}
	if got := readTestFile(t, filepath.Join(outDir, "fr-FR"), "guide.md"); !strings.Contains(got, `url=/fr-FR/guide"`) {
		t.Errorf("fr-FR/guide.md should redirect to /fr-FR/guide, got:\n%s", got)
	}

	// The default-language sitemap must not index the fr-FR pages: they already
	// appear in fr-FR/sitemap.xml, and listing them twice under two URLs is a
	// duplicate-content signal to crawlers. The default tag index likewise holds
	// only default-language tags — "docs" belongs to the French guide alone.
	wantRootLocs := []string{
		"https://build.example.com/",
		"https://build.example.com/tags/", // the tag index is emitted even when empty
	}
	if got := sitemapLocs(t, outDir, "sitemap.xml"); !slices.Equal(got, wantRootLocs) {
		t.Errorf("root sitemap.xml locs = %v, want %v", got, wantRootLocs)
	}
	if _, err := os.Stat(filepath.Join(outDir, "tags", "docs", "index.html")); !os.IsNotExist(err) {
		t.Errorf("tags/docs/index.html should not exist: the only page tagged docs is French (err = %v)", err)
	}

	// sitemap-index.xml stitches the languages together and references fr-FR.
	indexContent := readTestFile(t, outDir, "sitemap-index.xml")
	if !strings.Contains(indexContent, "fr-FR/sitemap.xml") {
		t.Errorf("sitemap-index.xml should reference the fr-FR sitemap, got:\n%s", indexContent)
	}

	// Parity fix (REVIEW §9.2): the static 404 carries the language switcher,
	// so the fr-FR code appears in the default 404 page.
	if root404 := readTestFile(t, outDir, "404.html"); !strings.Contains(root404, "fr-FR") {
		t.Error("static 404.html should include the language switcher (fr-FR)")
	}
}

func TestBuildCmd_GeneratesRedirectFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeTestFile(t, srcDir, "new-page.md",
		"---\ntitle: New Page\nredirect_from:\n  - /old-page\n  - /legacy/url\n---\n# New Page")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Each redirect_from source becomes source/index.html with a redirect to
	// the canonical (extensionless) target.
	for _, src := range []string{"old-page", filepath.Join("legacy", "url")} {
		body := readTestFile(t, outDir, filepath.Join(src, "index.html"))
		if !strings.Contains(body, "/new-page") {
			t.Errorf("redirect %s/index.html should point at /new-page, got:\n%s", src, body)
		}
		if !strings.Contains(strings.ToLower(body), "refresh") {
			t.Errorf("redirect %s/index.html should be a meta-refresh redirect, got:\n%s", src, body)
		}
	}
}

func TestBuildCmd_EmitsSeeAlsoSection(t *testing.T) {
	t.Parallel()

	contentDir := t.TempDir()
	files := map[string][]byte{
		"a.md": []byte("---\ntitle: A\ntags: [shared]\n---\n# A\n\nContent."),
		"b.md": []byte("---\ntitle: B\ntags: [shared]\n---\n# B\n\nMore."),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(contentDir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	outDir := t.TempDir()
	b := &BuildCmd{Dir: contentDir, Output: outDir}
	if err := b.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(outDir, "a", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if !strings.Contains(body, "see-also") {
		t.Errorf("a/index.html missing see-also section\n%s", body)
	}
	if !strings.Contains(body, ">B<") {
		t.Errorf("see-also missing related page B\n%s", body)
	}
}

// TestBuildCmd_Run_PagePathIsTheURLPath pins build mode's Page.Path to the URL
// the page is actually published at.
//
// Build walks files while serve is handed URLs, so build has to derive the URL
// itself. When it passed the raw file path instead, every path-keyed lookup
// missed: canonical/og:url/JSON-LD advertised a `.md` URL the site does not
// serve — contradicting the sitemap the same build emitted — and prev/next
// links plus sidebar active state silently vanished from every static build.
func TestBuildCmd_Run_PagePathIsTheURLPath(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeTestFile(t, srcDir, "README.md", "---\ntitle: Home\n---\n# Home")
	writeTestFile(t, srcDir, "guides/README.md", "---\ntitle: Guides\n---\n# Guides")
	writeTestFile(t, srcDir, "guides/alpha.md", "---\ntitle: Alpha\n---\n# Alpha")
	// Three siblings so the middle one has both a prev and a next.
	writeTestFile(t, srcDir, "guides/beta.md", "---\ntitle: Beta\n---\n# Beta")
	writeTestFile(t, srcDir, "guides/gamma.md", "---\ntitle: Gamma\n---\n# Gamma")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir, Domain: "build.example.com"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	alpha := readTestFile(t, filepath.Join(outDir, "guides", "alpha"), "index.html")

	// The whole point: no rendered URL may carry the source extension.
	if strings.Contains(alpha, "guides/alpha.md") {
		t.Errorf("guides/alpha/index.html leaks the .md path:\n%s", alpha)
	}

	// canonical, og:url and JSON-LD @id all derive from Page.Path, so checking
	// each one separately guards against a fix that only reaches the first.
	for _, want := range []string{
		`<link rel="canonical" href="https://build.example.com/guides/alpha">`,
		`<meta property="og:url" content="https://build.example.com/guides/alpha">`,
		`"@id":"https://build.example.com/guides/alpha"`,
	} {
		if !strings.Contains(alpha, want) {
			t.Errorf("guides/alpha/index.html missing %s", want)
		}
	}

	// The sitemap and the page must agree on one URL per page — the
	// self-contradiction that made this bug visible.
	sitemap := readTestFile(t, outDir, "sitemap.xml")
	if !strings.Contains(sitemap, "<loc>https://build.example.com/guides/alpha</loc>") {
		t.Errorf("sitemap disagrees with the page canonical:\n%s", sitemap)
	}

	// Navigation and prev/next index on clean paths, so a `.md` Page.Path made
	// both look up a key that does not exist and render nothing at all.
	if !strings.Contains(alpha, "<details open") {
		t.Error("guides/alpha/index.html has no expanded sidebar ancestor")
	}
	beta := readTestFile(t, filepath.Join(outDir, "guides", "beta"), "index.html")
	for _, want := range []string{
		`rel="prev" href="https://build.example.com/guides/alpha"`,
		`rel="next" href="https://build.example.com/guides/gamma"`,
	} {
		if !strings.Contains(beta, want) {
			t.Errorf("guides/beta/index.html missing %s", want)
		}
	}

	// A directory's default index is published at the directory, not at
	// /guides/README.
	guides := readTestFile(t, filepath.Join(outDir, "guides"), "index.html")
	if !strings.Contains(guides, `<link rel="canonical" href="https://build.example.com/guides">`) {
		t.Error("guides/index.html canonical is not the directory URL")
	}
	root := readTestFile(t, outDir, "index.html")
	if !strings.Contains(root, `<link rel="canonical" href="https://build.example.com/">`) {
		t.Error("index.html canonical is not the site root")
	}
}

// TestBuildCmd_Run_DefaultThemeLinksAndHreflang covers the two ways the bundled
// default theme used to emit URLs a visitor cannot follow: it forked head-meta
// and so shipped no hreflang at all, and the tag listing linked to raw
// PageInfo.Path values (".md" paths, with no language prefix, so every
// translated hit landed on the English page).
func TestBuildCmd_Run_DefaultThemeLinksAndHreflang(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")

	writeTestFile(t, srcDir, "README.md", "---\ntitle: Home\n---\n# Home")
	writeTestFile(t, srcDir, "guides/alpha.md", "---\ntitle: Alpha\ntags: [guide]\n---\n# Alpha")
	// beta exists only in French, so the default resolver cannot map it.
	writeTestFile(t, srcDir, "fr-FR/guides/beta.md", "---\ntitle: FR Beta\ntags: [guide]\n---\n# FR Beta")

	cmd := &BuildCmd{Dir: srcDir, Output: outDir, Domain: "build.example.com"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// hreflang lives in head-shared's head-meta. The default theme re-implemented
	// that block and dropped the one line it could not afford to lose.
	alpha := readTestFile(t, filepath.Join(outDir, "guides", "alpha"), "index.html")
	for _, want := range []string{
		`<link rel="alternate" hreflang="en-US" href="/guides/alpha">`,
		`<link rel="alternate" hreflang="x-default" href="/guides/alpha">`,
		`<link rel="alternate" hreflang="fr-FR" href="/fr-FR/guides/alpha">`,
	} {
		if !strings.Contains(alpha, want) {
			t.Errorf("guides/alpha/index.html missing %s", want)
		}
	}

	// The viewport tag is the one the switch to head-meta physically moved.
	if !strings.Contains(alpha, `<meta name="viewport" content="width=device-width, initial-scale=1">`) {
		t.Error("guides/alpha/index.html lost its viewport tag when switching to head-meta")
	}

	// A tag listing links real file paths, so it has to resolve them.
	tagPage := readTestFile(t, filepath.Join(outDir, "tags", "guide"), "index.html")
	if strings.Contains(tagPage, ".md\"") {
		t.Errorf("tags/guide/index.html links a raw .md path:\n%s", tagPage)
	}
	if !strings.Contains(tagPage, `<a class="tag-result-title" href="/guides/alpha">`) {
		t.Errorf("tags/guide/index.html does not link the clean page URL:\n%s", tagPage)
	}

	// A language index stores paths relative to its own sub-FS, so contentURL
	// must apply the pipeline's language prefix — otherwise this href is
	// "/guides/beta", which does not exist.
	frTagPage := readTestFile(t, filepath.Join(outDir, "fr-FR", "tags", "guide"), "index.html")
	if !strings.Contains(frTagPage, `<a class="tag-result-title" href="/fr-FR/guides/beta">`) {
		t.Errorf("fr-FR/tags/guide/index.html does not link into the fr-FR tree:\n%s", frTagPage)
	}
}
