package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestColorChips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "6-digit hex in backticks",
			input: "The brand color is `#FF5733` and it looks great.",
			wantContains: []string{
				`<color-chip>#FF5733</color-chip>`,
			},
			wantNotContain: []string{"<code>#FF5733</code>"},
		},
		{
			name:  "3-digit hex in backticks",
			input: "Short form: `#f0f`",
			wantContains: []string{
				`<color-chip>#f0f</color-chip>`,
			},
			wantNotContain: []string{"<code>#f0f</code>"},
		},
		{
			name:  "lowercase 6-digit hex",
			input: "Color: `#aabbcc`",
			wantContains: []string{
				`<color-chip>#aabbcc</color-chip>`,
			},
		},
		{
			name:  "multiple hex colors in same paragraph",
			input: "Mix `#FF0000` with `#00FF00`.",
			wantContains: []string{
				`<color-chip>#FF0000</color-chip>`,
				`<color-chip>#00FF00</color-chip>`,
			},
		},
		{
			name:  "hex colors in list items",
			input: "- Primary: `#3366FF`\n- Secondary: `#FF6633`",
			wantContains: []string{
				`<color-chip>#3366FF</color-chip>`,
				`<color-chip>#FF6633</color-chip>`,
			},
		},
		{
			name:           "backtick code with extra text not transformed",
			input:          "Use `color: #FF5733` in your CSS.",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"#FF5733"},
		},
		{
			name:           "bare hex in text not transformed",
			input:          "The color #FF5733 looks good.",
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "hex in fenced code block not transformed",
			input:          "```css\ncolor: #FF5733;\n```",
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "markdown link fragment not transformed",
			input:          "See [section](#overview) for details.",
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "invalid hex too short not transformed",
			input:          "Code: `#ab`",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#ab</code>"},
		},
		{
			name:           "invalid hex too long not transformed",
			input:          "Code: `#1234567`",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#1234567</code>"},
		},
		{
			name:           "code with non-hex chars not transformed",
			input:          "Code: `#GGHHII`",
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#GGHHII</code>"},
		},
		{
			name:           "empty content",
			input:          "",
			wantNotContain: []string{"color-chip"},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": true}})
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

func TestColorChips_Disabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("globally disabled", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": false}})
		result, err := r.Render(ctx, []byte("Color: `#FF5733`"), &enricher.EnrichmentData{})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when globally disabled")
		}
	})

	t.Run("globally enabled but page disables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(ctx, []byte("Color: `#FF5733`"), &enricher.EnrichmentData{
			Features: map[string]bool{"color_chips": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when page disables them")
		}
	})

	t.Run("globally disabled but page enables", func(t *testing.T) {
		t.Parallel()
		r := NewMarkdownRenderer(MarkdownOptions{Features: map[string]bool{"color_chips": false}})
		result, err := r.Render(ctx, []byte("Color: `#FF5733`"), &enricher.EnrichmentData{
			Features: map[string]bool{"color_chips": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should appear when page enables them")
		}
	})
}
