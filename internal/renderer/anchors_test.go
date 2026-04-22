package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestHeadingAnchors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "h1 with auto id",
			input: "# Title",
			wantContains: []string{
				`<h1 id="title">`,
				`Title`,
				`<a href="#title" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>`,
				`</h1>`,
			},
		},
		{
			name:  "h2 with auto id",
			input: "## Section",
			wantContains: []string{
				`<h2 id="section">`,
				`Section`,
				`<a href="#section" class="heading-anchor"`,
			},
		},
		{
			name:  "h3 with auto id",
			input: "### Sub",
			wantContains: []string{
				`<h3 id="sub">`,
				`<a href="#sub"`,
			},
		},
		{
			name:  "heading with inline markup",
			input: "## Code `example`",
			wantContains: []string{
				`<h2 id="code-example">`,
				`Code `,
				`<code>example</code>`,
				`<a href="#code-example" class="heading-anchor"`,
			},
		},
		{
			name:  "multiple headings in same content",
			input: "# First\n\nSome text.\n\n## Second",
			wantContains: []string{
				`<a href="#first"`,
				`<a href="#second"`,
			},
		},
		{
			name:         "paragraph only, no headings",
			input:        "Just a paragraph.",
			wantContains: []string{"<p>Just a paragraph.</p>"},
			wantNotContain: []string{
				"heading-anchor",
			},
		},
		{
			name:           "empty content",
			input:          "",
			wantNotContain: []string{"heading-anchor"},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := r.Render(ctx, []byte(tt.input), &enricher.EnrichmentData{})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			output := string(result.Content)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, output)
				}
			}
		})
	}
}

func TestHeadingAnchors_Disabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("globally disabled", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"heading_anchors": false}})
		result, err := r.Render(ctx, []byte("## Section"), &enricher.EnrichmentData{})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		output := string(result.Content)
		if strings.Contains(output, "heading-anchor") {
			t.Errorf("heading anchors should not appear when globally disabled\nGot: %s", output)
		}
		if !strings.Contains(output, `<h2 id="section">`) {
			t.Errorf("heading should still have id attribute\nGot: %s", output)
		}
	})

	t.Run("globally enabled but page disables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(ctx, []byte("## Section"), &enricher.EnrichmentData{
			Features: map[string]bool{"heading_anchors": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "heading-anchor") {
			t.Error("heading anchors should not appear when page disables them")
		}
	})

	t.Run("globally disabled but page enables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"heading_anchors": false}})
		result, err := r.Render(ctx, []byte("## Section"), &enricher.EnrichmentData{
			Features: map[string]bool{"heading_anchors": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "heading-anchor") {
			t.Error("heading anchors should appear when page enables them")
		}
	})
}
