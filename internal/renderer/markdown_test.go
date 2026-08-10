package renderer

import (
	"context"
	"regexp"
	"slices"
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

// flattenTOCIDs returns the TOC node IDs in document order.
func flattenTOCIDs(node *enricher.TOCNode) []string {
	if node == nil {
		return nil
	}
	var ids []string
	if node.Level > 0 {
		ids = append(ids, node.ID)
	}
	for _, child := range node.Children {
		ids = append(ids, flattenTOCIDs(child)...)
	}
	return ids
}

var headingIDRe = regexp.MustCompile(`<h[1-6] id="([^"]*)"`)

// headingIDs returns the id attribute of every <hN> element in html, in
// document order. Deliberately a regexp over the output rather than a parse:
// the anchor a reader's browser resolves is the literal attribute text.
func headingIDs(html string) []string {
	matches := headingIDRe.FindAllStringSubmatch(html, -1)
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m[1])
	}
	return ids
}

// TestHeadingSlugs pins goldmark's WithAutoHeadingID output. Anchor stability is
// an inbound-link contract — #installation quietly becoming #installing breaks
// every external link to it — and the algorithm is goldmark's choice, not ours,
// so a dependency bump can change it with nothing in the tree going red.
//
// It also pins the two parsers against each other. The renderer writes the id
// attribute and the enricher supplies the TOC's href, from separate goldmark
// instances configured in separate packages (renderer/markdown.go,
// enricher/markdown.go). Enabling WithAutoHeadingID in one and not the other
// leaves every TOC link pointing at nothing, and neither package's own tests
// can see it.
func TestHeadingSlugs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "lowercased, spaces become hyphens",
			input: "# Getting Started",
			want:  []string{"getting-started"},
		},
		{
			name:  "punctuation is dropped, not replaced",
			input: "# What's New?",
			want:  []string{"whats-new"},
		},
		{
			// The dot goes the way of the apostrophe — only ASCII letters and
			// digits are kept. The em dash is dropped but its two surrounding
			// spaces each still emit a hyphen, which is where the doubled one
			// comes from: there is no run collapsing.
			name:  "dots are dropped, hyphens survive, spaces do not collapse",
			input: "# Go 1.25 — release-notes",
			want:  []string{"go-125--release-notes"},
		},
		{
			// The other half of that rule, stated on its own so a regression in
			// either direction has a row: `_` folds to `-` rather than being
			// kept, and a doubled space yields a doubled hyphen.
			name:  "underscores fold to hyphens, runs are not collapsed",
			input: "# snake_case  and--more",
			want:  []string{"snake-case--and--more"},
		},
		{
			name:  "duplicates get a numeric suffix, first occurrence bare",
			input: "# Setup\n\n# Setup\n\n# Setup\n",
			want:  []string{"setup", "setup-1", "setup-2"},
		},
		{
			// Not transliterated and not preserved — dropped. goldmark's
			// slugifier keeps ASCII alphanumerics only, so an accent takes its
			// letter with it. Documented in
			// docs/guide/12-advanced/02-markdown-extensions.md and filed in
			// REVIEW.md; this row exists to make the loss visible rather than
			// to bless it.
			name:  "non-ASCII letters are dropped",
			input: "# Café Français",
			want:  []string{"caf-franais"},
		},
		{
			name:  "inline markup contributes its text only",
			input: "# The `Handler` **type**",
			want:  []string{"the-handler-type"},
		},
		{
			// Nothing is left to slugify, so goldmark falls back to a positional
			// id rather than emitting an empty one.
			name:  "a heading with no slugifiable text still gets an id",
			input: "# !!!",
			want:  []string{"heading"},
		},
		{
			// The same rule taken to its conclusion: with no ASCII left there
			// is nothing to slugify, so a non-Latin page's anchors are heading,
			// heading-1, heading-2… — deduplicated like any other collision,
			// which is what makes the loss above lossy rather than merely ugly.
			name:  "fully non-ASCII headings collapse to deduplicated fallbacks",
			input: "# 日本語\n\n# 한국어\n",
			want:  []string{"heading", "heading-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			enrich := enricher.NewMarkdownEnricher(enricher.MarkdownEnricherOptions{})
			data, err := enrich.Enrich(context.Background(), []byte(tt.input), "/test.md")
			if err != nil {
				t.Fatalf("Enrich() error = %v", err)
			}
			if got := flattenTOCIDs(data.TOC); !slices.Equal(got, tt.want) {
				t.Errorf("TOC anchor IDs = %v, want %v", got, tt.want)
			}

			result, err := NewMarkdownRenderer(MarkdownOptions{}).Render(context.Background(), []byte(tt.input), data)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if got := headingIDs(string(result.Content)); !slices.Equal(got, tt.want) {
				t.Errorf("rendered heading IDs = %v, want %v", got, tt.want)
			}
		})
	}
}
