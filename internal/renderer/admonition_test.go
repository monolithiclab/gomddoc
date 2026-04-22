package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestAdmonitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "NOTE admonition",
			input: "> [!NOTE]\n> This is a note.",
			wantContains: []string{
				`class="admonition admonition-note"`,
				`class="admonition-title"`,
				"Note",
				"This is a note.",
			},
			wantNotContain: []string{"<blockquote>", "[!NOTE]"},
		},
		{
			name:  "WARNING admonition",
			input: "> [!WARNING]\n> Be careful!",
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"Warning",
				"Be careful!",
			},
			wantNotContain: []string{"<blockquote>", "[!WARNING]"},
		},
		{
			name:  "TIP admonition",
			input: "> [!TIP]\n> Helpful tip here.",
			wantContains: []string{
				`class="admonition admonition-tip"`,
				"Tip",
				"Helpful tip here.",
			},
		},
		{
			name:  "IMPORTANT admonition",
			input: "> [!IMPORTANT]\n> Do not ignore this.",
			wantContains: []string{
				`class="admonition admonition-important"`,
				"Important",
				"Do not ignore this.",
			},
		},
		{
			name:  "CAUTION admonition",
			input: "> [!CAUTION]\n> Danger ahead.",
			wantContains: []string{
				`class="admonition admonition-caution"`,
				"Caution",
				"Danger ahead.",
			},
		},
		{
			name:         "regular blockquote not affected",
			input:        "> This is just a regular quote.",
			wantContains: []string{"<blockquote>", "This is just a regular quote."},
			wantNotContain: []string{
				"admonition",
			},
		},
		{
			name:  "multi-line admonition content",
			input: "> [!NOTE]\n> First line.\n> Second line.\n> Third line.",
			wantContains: []string{
				`class="admonition admonition-note"`,
				"First line.",
				"Second line.",
				"Third line.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name:  "multi-paragraph admonition",
			input: "> [!WARNING]\n> First paragraph.\n>\n> Second paragraph.",
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"First paragraph.",
				"Second paragraph.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name:  "marker only, no content",
			input: "> [!TIP]",
			wantContains: []string{
				`class="admonition admonition-tip"`,
				`class="admonition-title"`,
				"Tip",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name:  "multiple admonitions in same document",
			input: "> [!NOTE]\n> A note.\n\nSome text between.\n\n> [!WARNING]\n> A warning.",
			wantContains: []string{
				"admonition-note",
				"admonition-warning",
				"A note.",
				"A warning.",
				"Some text between.",
			},
			wantNotContain: []string{"<blockquote>"},
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

func TestAdmonitions_Disabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("globally disabled", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"admonitions": false}})
		result, err := r.Render(ctx, []byte("> [!NOTE]\n> Content"), &enricher.EnrichmentData{})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		output := string(result.Content)
		if strings.Contains(output, "admonition") {
			t.Errorf("admonitions should not appear when globally disabled\nGot: %s", output)
		}
		if !strings.Contains(output, "<blockquote>") {
			t.Errorf("blockquote should be preserved when admonitions disabled\nGot: %s", output)
		}
	})

	t.Run("globally enabled but page disables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(ctx, []byte("> [!NOTE]\n> Content"), &enricher.EnrichmentData{
			Features: map[string]bool{"admonitions": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "admonition") {
			t.Error("admonitions should not appear when page disables them")
		}
	})

	t.Run("globally disabled but page enables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"admonitions": false}})
		result, err := r.Render(ctx, []byte("> [!NOTE]\n> Content"), &enricher.EnrichmentData{
			Features: map[string]bool{"admonitions": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "admonition") {
			t.Error("admonitions should appear when page enables them")
		}
	})
}
