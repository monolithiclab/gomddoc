package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
