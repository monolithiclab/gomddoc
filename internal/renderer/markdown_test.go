package renderer

import (
	"context"
	"strings"
	"testing"
)

func TestMarkdownRenderer_SupportedMimeTypes(t *testing.T) {
	renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
	mimeTypes := renderer.SupportedMimeTypes()

	if len(mimeTypes) != 1 {
		t.Errorf("SupportedMimeTypes() returned %d types, want 1", len(mimeTypes))
	}

	if mimeTypes[0] != "text/markdown" {
		t.Errorf("SupportedMimeTypes() = %v, want [text/markdown]", mimeTypes)
	}
}

func TestMarkdownRenderer_Render(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantMimeType   string
		wantNotContain []string
	}{
		{
			name:         "heading with auto ID",
			input:        "# Hello",
			wantContains: []string{"<h1", "id=\"hello\"", "heading-anchor", `href="#hello"`, "</h1>"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "paragraph",
			input:        "This is a paragraph.",
			wantContains: []string{"<p>This is a paragraph.</p>"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "empty content",
			input:        "",
			wantContains: []string{},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "table",
			input:        "| Header |\n|--------|\n| Cell   |",
			wantContains: []string{"<table>", "<thead>", "<tbody>", "Header", "Cell"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:           "no template wrapping",
			input:          "# Test",
			wantNotContain: []string{"<!DOCTYPE", "<html>", "<body>"},
			wantMimeType:   "text/html; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
			result, err := renderer.Render(context.Background(), []byte(tt.input))

			if err != nil {
				t.Fatalf("Render() error = %v, want nil", err)
			}

			if result.MimeType != tt.wantMimeType {
				t.Errorf("Render() mimeType = %q, want %q", result.MimeType, tt.wantMimeType)
			}

			outputStr := string(result.Content)
			for _, want := range tt.wantContains {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Render() output missing %q\nGot: %s", want, outputStr)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(outputStr, notWant) {
					t.Errorf("Render() output should not contain %q\nGot: %s", notWant, outputStr)
				}
			}
		})
	}
}

func TestMarkdownRenderer_FrontMatter(t *testing.T) {
	renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
	input := `---
title: Hello World
tags: [a, b]
---
# Content`

	result, err := renderer.Render(context.Background(), []byte(input))
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}

	if result.Metadata == nil {
		t.Fatal("Render() metadata is nil")
	}

	if title, ok := result.Metadata["title"].(string); !ok || title != "Hello World" {
		t.Errorf("Metadata['title'] = %v, want 'Hello World'", result.Metadata["title"])
	}

	outputStr := string(result.Content)
	if strings.Contains(outputStr, "title: Hello World") {
		t.Error("Render() output should not contain front matter")
	}
	if !strings.Contains(outputStr, "<h1") {
		t.Error("Render() output should contain markdown content")
	}
}

func TestMarkdownRenderer_ContextCancellation(t *testing.T) {
	t.Parallel()

	renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
	testContextCancellation(t, func(ctx context.Context) error {
		_, err := renderer.Render(ctx, []byte("# Test"))
		return err
	})
}

func TestMarkdownRenderer_ConcurrentRenders(t *testing.T) {
	renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
	ctx := context.Background()

	// Test that renderer can be used concurrently
	const concurrency = 100
	done := make(chan bool, concurrency)

	for i := range concurrency {
		go func(n int) {
			input := []byte("# Heading " + string(rune('A'+(n%26))))
			_, err := renderer.Render(ctx, input)
			if err != nil {
				t.Errorf("Concurrent render failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range concurrency {
		<-done
	}
}

func TestMarkdownRenderer_LargeContent(t *testing.T) {
	renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})

	// Generate large markdown content
	var builder strings.Builder
	for i := range 1000 {
		builder.WriteString("## Section ")
		builder.WriteString(string(rune('0' + (i % 10))))
		builder.WriteString("\n\nThis is paragraph ")
		builder.WriteString(string(rune('0' + (i % 10))))
		builder.WriteString(".\n\n")
	}

	result, err := renderer.Render(context.Background(), []byte(builder.String()))

	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}

	if result.MimeType != "text/html; charset=utf-8" {
		t.Errorf("Render() mimeType = %q, want %q", result.MimeType, "text/html; charset=utf-8")
	}

	if len(result.Content) == 0 {
		t.Error("Render() output is empty for large content")
	}

	// Verify structure is maintained
	outputStr := string(result.Content)
	if !strings.Contains(outputStr, "<h2") {
		t.Error("Render() output missing headings for large content")
	}
}
