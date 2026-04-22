package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
	"github.com/monolithiclab/gomddoc/internal/text"
)

func TestMarkdownPassthroughRenderer_MimeTypes(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
			t.Parallel()

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

func TestMarkdownPassthroughRenderer_RenderWithEnrichment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		enrichment *enricher.EnrichmentData
		wantFM     bool     // expect frontmatter delimiters
		wantStrs   []string // strings that should appear in output
		wantNoStrs []string // strings that should NOT appear in output
	}{
		{
			name:       "nil enrichment",
			input:      "# Hello",
			enrichment: nil,
			wantFM:     false,
			wantStrs:   []string{"# Hello"},
		},
		{
			name:  "metadata preserved",
			input: "---\ntitle: Original\n---\n# Hello",
			enrichment: &enricher.EnrichmentData{
				Metadata: map[string]any{"title": "Original", "description": "A page"},
			},
			wantFM:   true,
			wantStrs: []string{"metadata:", "title: Original", "description: A page", "# Hello"},
		},
		{
			name:  "related docs",
			input: "# Hello",
			enrichment: &enricher.EnrichmentData{
				RelatedDocs: []enricher.RelatedDoc{
					{Path: "/guide.md", Title: "Guide"},
					{Path: "/faq.md", Title: "FAQ"},
				},
			},
			wantFM:   true,
			wantStrs: []string{"related_docs:", "/guide.md", "Guide", "/faq.md", "FAQ"},
		},
		{
			name:  "prev and next pages",
			input: "# Middle",
			enrichment: &enricher.EnrichmentData{
				PrevPage: &enricher.PageLink{Path: "/intro.md", Title: "Intro"},
				NextPage: &enricher.PageLink{Path: "/advanced.md", Title: "Advanced"},
			},
			wantFM:   true,
			wantStrs: []string{"prev_page:", "Intro", "next_page:", "Advanced"},
		},
		{
			name:       "empty enrichment no frontmatter",
			input:      "# Hello",
			enrichment: &enricher.EnrichmentData{},
			wantFM:     false,
			wantStrs:   []string{"# Hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewMarkdownPassthroughRenderer()
			result, err := r.Render(context.Background(), []byte(tt.input), tt.enrichment)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			output := string(result.Content)
			if tt.wantFM && !strings.Contains(output, "---\n") {
				t.Errorf("expected frontmatter delimiters, got: %s", output)
			}
			if !tt.wantFM && strings.Contains(output, "---\n") {
				t.Errorf("unexpected frontmatter delimiters, got: %s", output)
			}
			for _, s := range tt.wantStrs {
				if !strings.Contains(output, s) {
					t.Errorf("output missing %q, got: %s", s, output)
				}
			}
			for _, s := range tt.wantNoStrs {
				if strings.Contains(output, s) {
					t.Errorf("output should not contain %q, got: %s", s, output)
				}
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
	t.Parallel()

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
			t.Parallel()

			got := string(text.StripFrontmatter([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("text.StripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}
