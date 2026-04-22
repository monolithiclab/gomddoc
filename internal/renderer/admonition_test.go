package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func Test_transformAdmonitions(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "NOTE admonition",
			input: "<blockquote>\n<p>[!NOTE]\nThis is a note.</p>\n</blockquote>",
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
			input: "<blockquote>\n<p>[!WARNING]\nBe careful!</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"Warning",
				"Be careful!",
			},
			wantNotContain: []string{"<blockquote>", "[!WARNING]"},
		},
		{
			name:  "TIP admonition",
			input: "<blockquote>\n<p>[!TIP]\nHelpful tip here.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-tip"`,
				"Tip",
				"Helpful tip here.",
			},
		},
		{
			name:  "IMPORTANT admonition",
			input: "<blockquote>\n<p>[!IMPORTANT]\nDo not ignore this.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-important"`,
				"Important",
				"Do not ignore this.",
			},
		},
		{
			name:  "CAUTION admonition",
			input: "<blockquote>\n<p>[!CAUTION]\nDanger ahead.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-caution"`,
				"Caution",
				"Danger ahead.",
			},
		},
		{
			name:  "regular blockquote not affected",
			input: "<blockquote>\n<p>This is a regular blockquote.</p>\n</blockquote>",
			wantContains: []string{
				"<blockquote>",
				"This is a regular blockquote.",
			},
			wantNotContain: []string{"admonition"},
		},
		{
			name:  "multi-line admonition content",
			input: "<blockquote>\n<p>[!NOTE]\nFirst line.\nSecond line.\nThird line.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-note"`,
				"First line.",
				"Second line.",
				"Third line.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name:  "admonition with multiple paragraphs",
			input: "<blockquote>\n<p>[!WARNING]\nFirst paragraph.</p>\n<p>Second paragraph.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"First paragraph.",
				"Second paragraph.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name:  "case insensitive type marker",
			input: "<blockquote>\n<p>[!note]\nLowercase marker.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-note"`,
				"Lowercase marker.",
			},
		},
		{
			name:  "mixed case type marker",
			input: "<blockquote>\n<p>[!Note]\nMixed case.</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-note"`,
				"Mixed case.",
			},
		},
		{
			name:  "admonition with no content after marker",
			input: "<blockquote>\n<p>[!TIP]</p>\n</blockquote>",
			wantContains: []string{
				`class="admonition admonition-tip"`,
				`class="admonition-title"`,
				"Tip",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name: "multiple admonitions in same document",
			input: `<blockquote>
<p>[!NOTE]
A note.</p>
</blockquote>
<p>Some text between.</p>
<blockquote>
<p>[!WARNING]
A warning.</p>
</blockquote>`,
			wantContains: []string{
				`admonition-note`,
				`admonition-warning`,
				"A note.",
				"A warning.",
				"Some text between.",
			},
			wantNotContain: []string{"<blockquote>"},
		},
		{
			name:  "blockquote without marker prefix is preserved",
			input: "<blockquote>\n<p>Just a quote with [!NOTE] in the middle.</p>\n</blockquote>",
			wantContains: []string{
				"<blockquote>",
			},
			wantNotContain: []string{"admonition"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformAdmonitions([]byte(tt.input))
			output := string(result)

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

func Test_transformAdmonitions_EmptyInput(t *testing.T) {
	result := transformAdmonitions([]byte{})
	if len(result) != 0 {
		t.Errorf("expected empty output for empty input, got %q", string(result))
	}
}

func Test_transformAdmonitions_NoBlockquotes(t *testing.T) {
	input := []byte("<p>No blockquotes here.</p>")
	result := transformAdmonitions(input)
	if string(result) != string(input) {
		t.Errorf("expected unchanged output, got %q", string(result))
	}
}

// TestMarkdownRenderer_Admonitions verifies the full pipeline: markdown input through
// the renderer produces admonition HTML output.
func TestMarkdownRenderer_Admonitions(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "NOTE admonition from markdown",
			input: `> [!NOTE]
> This is a note from markdown.`,
			wantContains: []string{
				`class="admonition admonition-note"`,
				`class="admonition-title"`,
				"Note",
				"This is a note from markdown.",
			},
			wantNotContain: []string{"<blockquote>", "[!NOTE]"},
		},
		{
			name: "WARNING admonition from markdown",
			input: `> [!WARNING]
> This is a warning.`,
			wantContains: []string{
				`class="admonition admonition-warning"`,
				"Warning",
				"This is a warning.",
			},
		},
		{
			name:           "regular blockquote preserved",
			input:          `> This is just a regular quote.`,
			wantContains:   []string{"<blockquote>", "This is just a regular quote."},
			wantNotContain: []string{"admonition"},
		},
		{
			name: "multi-paragraph admonition from markdown",
			input: `> [!IMPORTANT]
> First paragraph.
>
> Second paragraph.`,
			wantContains: []string{
				`class="admonition admonition-important"`,
				"Important",
				"First paragraph.",
				"Second paragraph.",
			},
		},
	}

	renderer := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderer.Render(ctx, []byte(tt.input), &enricher.EnrichmentData{})
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
