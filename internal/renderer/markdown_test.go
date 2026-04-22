package renderer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMarkdownRenderer_SupportedMimeTypes(t *testing.T) {
	renderer := NewMarkdownRenderer()
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
			wantContains: []string{"<h1", "id=\"hello\"", ">Hello</h1>"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "paragraph",
			input:        "This is a paragraph.",
			wantContains: []string{"<p>This is a paragraph.</p>"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "multiple headings",
			input:        "# Title\n\n## Subtitle",
			wantContains: []string{"<h1", "id=\"title\"", "<h2", "id=\"subtitle\""},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "code block",
			input:        "```go\nfunc main() {}\n```",
			wantContains: []string{"<pre>", "<code", "func main()"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "list",
			input:        "- Item 1\n- Item 2",
			wantContains: []string{"<ul>", "<li>Item 1</li>", "<li>Item 2</li>"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "emphasis",
			input:        "**bold** and *italic*",
			wantContains: []string{"<strong>bold</strong>", "<em>italic</em>"},
			wantMimeType: "text/html; charset=utf-8",
		},
		{
			name:         "link",
			input:        "[link](https://example.com)",
			wantContains: []string{"<a href=\"https://example.com\">link</a>"},
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
			renderer := NewMarkdownRenderer()
			output, mimeType, err := renderer.Render(context.Background(), []byte(tt.input))

			if err != nil {
				t.Fatalf("Render() error = %v, want nil", err)
			}

			if mimeType != tt.wantMimeType {
				t.Errorf("Render() mimeType = %q, want %q", mimeType, tt.wantMimeType)
			}

			outputStr := string(output)
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

func TestMarkdownRenderer_ContextCancellation(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		wantError bool
	}{
		{
			name: "already cancelled context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx
			},
			wantError: true,
		},
		{
			name: "timeout context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(2 * time.Millisecond) // Ensure timeout
				return ctx
			},
			wantError: true,
		},
		{
			name:      "valid context",
			setupCtx:  context.Background,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewMarkdownRenderer()
			ctx := tt.setupCtx()

			_, _, err := renderer.Render(ctx, []byte("# Test"))

			if tt.wantError && err == nil {
				t.Error("Render() error = nil, want error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Render() error = %v, want nil", err)
			}

			if tt.wantError && err != nil {
				// Verify it's a context error
				if err != context.Canceled && err != context.DeadlineExceeded {
					t.Errorf("Render() error = %v, want context.Canceled or context.DeadlineExceeded", err)
				}
			}
		})
	}
}

func TestMarkdownRenderer_ConcurrentRenders(t *testing.T) {
	renderer := NewMarkdownRenderer()
	ctx := context.Background()

	// Test that renderer can be used concurrently
	const concurrency = 100
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(n int) {
			input := []byte("# Heading " + string(rune('A'+(n%26))))
			_, _, err := renderer.Render(ctx, input)
			if err != nil {
				t.Errorf("Concurrent render failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

func TestMarkdownRenderer_LargeContent(t *testing.T) {
	renderer := NewMarkdownRenderer()

	// Generate large markdown content
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString("## Section ")
		builder.WriteString(string(rune('0' + (i % 10))))
		builder.WriteString("\n\nThis is paragraph ")
		builder.WriteString(string(rune('0' + (i % 10))))
		builder.WriteString(".\n\n")
	}

	output, mimeType, err := renderer.Render(context.Background(), []byte(builder.String()))

	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}

	if mimeType != "text/html; charset=utf-8" {
		t.Errorf("Render() mimeType = %q, want %q", mimeType, "text/html; charset=utf-8")
	}

	if len(output) == 0 {
		t.Error("Render() output is empty for large content")
	}

	// Verify structure is maintained
	outputStr := string(output)
	if !strings.Contains(outputStr, "<h2") {
		t.Error("Render() output missing headings for large content")
	}
}
