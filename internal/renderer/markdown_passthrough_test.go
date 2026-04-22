package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestMarkdownPassthroughRenderer_MimeTypes(t *testing.T) {
	r := NewMarkdownPassthroughRenderer()

	inputTypes := r.InputMimeTypes()
	if len(inputTypes) != 1 || inputTypes[0] != "text/markdown" {
		t.Errorf("InputMimeTypes() = %v, want [text/markdown]", inputTypes)
	}

	outputTypes := r.OutputMimeTypes()
	if len(outputTypes) != 1 || outputTypes[0] != "text/markdown" {
		t.Errorf("OutputMimeTypes() = %v, want [text/markdown]", outputTypes)
	}
}

func TestMarkdownPassthroughRenderer_Render(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string
		wantNoFM     bool
	}{
		{
			name:         "plain markdown",
			input:        "# Hello\n\nWorld",
			wantContains: "# Hello",
		},
		{
			name:         "frontmatter stripped",
			input:        "---\ntitle: Test\n---\n# Hello\n\nWorld",
			wantContains: "# Hello",
			wantNoFM:     true,
		},
		{
			name:         "empty content",
			input:        "",
			wantContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewMarkdownPassthroughRenderer()
			result, err := r.Render(context.Background(), []byte(tt.input), &enricher.EnrichmentData{})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if result.MimeType != "text/markdown; charset=utf-8" {
				t.Errorf("MimeType = %q, want text/markdown; charset=utf-8", result.MimeType)
			}

			output := string(result.Content)
			if tt.wantContains != "" && !strings.Contains(output, tt.wantContains) {
				t.Errorf("output missing %q, got: %s", tt.wantContains, output)
			}

			if tt.wantNoFM && strings.Contains(output, "---") {
				t.Errorf("output should not contain frontmatter delimiters, got: %s", output)
			}
		})
	}
}

func TestMarkdownPassthroughRenderer_ContextCancellation(t *testing.T) {
	t.Parallel()

	r := NewMarkdownPassthroughRenderer()
	testContextCancellation(t, func(ctx context.Context) error {
		_, err := r.Render(ctx, []byte("# Test"), &enricher.EnrichmentData{})
		return err
	})
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no frontmatter",
			input: "# Hello",
			want:  "# Hello",
		},
		{
			name:  "with frontmatter",
			input: "---\ntitle: Test\n---\n# Hello",
			want:  "# Hello",
		},
		{
			name:  "only frontmatter open",
			input: "---\ntitle: Test\n# Hello",
			want:  "---\ntitle: Test\n# Hello",
		},
		{
			name:  "empty body after frontmatter",
			input: "---\ntitle: Test\n---\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripFrontmatter([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("stripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}
