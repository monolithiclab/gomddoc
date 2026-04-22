package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestMarkdownRenderer_MimeTypes(t *testing.T) {
	t.Parallel()

	renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})

	inputTypes := renderer.InputMimeTypes()
	if len(inputTypes) != 1 || inputTypes[0] != "text/markdown" {
		t.Errorf("InputMimeTypes() = %v, want [text/markdown]", inputTypes)
	}

	outputTypes := renderer.OutputMimeTypes()
	if len(outputTypes) != 1 || outputTypes[0] != "text/html" {
		t.Errorf("OutputMimeTypes() = %v, want [text/html]", outputTypes)
	}
}

func TestMarkdownRenderer_Render(t *testing.T) {
	t.Parallel()

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
			wantContains: []string{"<h1", `id="hello"`, "gmd-heading-anchor", `href="#hello"`, "</h1>"},
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
			t.Parallel()

			renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})
			result, err := renderer.Render(context.Background(), []byte(tt.input), &enricher.EnrichmentData{})

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
	t.Parallel()

	renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})
	input := `---
title: Hello World
tags: [a, b]
---
# Content`

	result, err := renderer.Render(context.Background(), []byte(input), &enricher.EnrichmentData{
		Metadata: map[string]any{"title": "Hello World"},
	})
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
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

	renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})
	testContextCancellation(t, func(ctx context.Context) error {
		_, err := renderer.Render(ctx, []byte("# Test"), &enricher.EnrichmentData{})
		return err
	})
}

func TestMarkdownRenderer_ConcurrentRenders(t *testing.T) {
	t.Parallel()

	renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})
	ctx := context.Background()

	// Test that renderer can be used concurrently
	const concurrency = 100
	done := make(chan bool, concurrency)

	for i := range concurrency {
		go func(n int) {
			input := []byte("# Heading " + string(rune('A'+(n%26))))
			_, err := renderer.Render(ctx, input, &enricher.EnrichmentData{})
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
	t.Parallel()

	renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})

	// Generate large markdown content
	var builder strings.Builder
	for i := range 1000 {
		builder.WriteString("## Section ")
		builder.WriteString(string(rune('0' + (i % 10))))
		builder.WriteString("\n\nThis is paragraph ")
		builder.WriteString(string(rune('0' + (i % 10))))
		builder.WriteString(".\n\n")
	}

	result, err := renderer.Render(context.Background(), []byte(builder.String()), &enricher.EnrichmentData{})

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
