package renderer

import (
	"context"
	"strings"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func TestTransformColorChips(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "6-digit hex in code tag",
			input: `<p>Use <code>#FF5733</code> for the accent.</p>`,
			wantContains: []string{
				`<color-chip>#FF5733</color-chip>`,
			},
			wantNotContain: []string{"<code>#FF5733</code>"},
		},
		{
			name:  "3-digit hex in code tag",
			input: `<p>Red is <code>#f00</code>.</p>`,
			wantContains: []string{
				`<color-chip>#f00</color-chip>`,
			},
			wantNotContain: []string{"<code>#f00</code>"},
		},
		{
			name:  "lowercase 6-digit hex",
			input: `<code>#aabbcc</code>`,
			wantContains: []string{
				`<color-chip>#aabbcc</color-chip>`,
			},
		},
		{
			name:  "multiple code colors in same paragraph",
			input: `<p>Mix <code>#FF0000</code> with <code>#00FF00</code>.</p>`,
			wantContains: []string{
				`<color-chip>#FF0000</color-chip>`,
				`<color-chip>#00FF00</color-chip>`,
			},
		},
		{
			name:           "code tag with non-color content unchanged",
			input:          `<p>Run <code>go test</code> to verify.</p>`,
			wantContains:   []string{"<code>go test</code>"},
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "code tag with hex and extra text unchanged",
			input:          `<p>Set <code>color: #FF5733</code> in CSS.</p>`,
			wantContains:   []string{"<code>color: #FF5733</code>"},
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "bare hex in text not transformed",
			input:          `<p>The color #FF5733 is nice.</p>`,
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"#FF5733"},
		},
		{
			name:           "hex in fenced code block not transformed",
			input:          `<pre><code>color: #FF5733;</code></pre>`,
			wantNotContain: []string{"color-chip"},
		},
		{
			name:           "anchor href not transformed",
			input:          `<a href="#section">link</a>`,
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{`href="#section"`},
		},
		{
			name:           "invalid hex too short not transformed",
			input:          `<code>#ab</code>`,
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#ab</code>"},
		},
		{
			name:           "invalid hex too long not transformed",
			input:          `<code>#1234567</code>`,
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#1234567</code>"},
		},
		{
			name:           "code with non-hex chars not transformed",
			input:          `<code>#GGHHII</code>`,
			wantNotContain: []string{"color-chip"},
			wantContains:   []string{"<code>#GGHHII</code>"},
		},
		{
			name:           "empty input",
			input:          ``,
			wantNotContain: []string{"color-chip"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformColorChips([]byte(tt.input))
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

// TestMarkdownRenderer_ColorChips verifies the full pipeline: markdown with
// backtick-wrapped hex colors produces color chip HTML.
func TestMarkdownRenderer_ColorChips(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:  "backtick hex color in paragraph",
			input: "The brand color is `#E91E63` and it looks great.",
			wantContains: []string{
				`<color-chip>#E91E63</color-chip>`,
			},
		},
		{
			name:  "backtick hex color in list",
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
			name:  "3-digit hex in backticks",
			input: "Short form: `#f0f`",
			wantContains: []string{
				`<color-chip>#f0f</color-chip>`,
			},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestColorChipsEnabled(t *testing.T) {
	tests := []struct {
		name          string
		globalDefault bool
		metadata      map[string]any
		want          bool
	}{
		{
			name:          "global enabled, no frontmatter",
			globalDefault: true,
			metadata:      nil,
			want:          true,
		},
		{
			name:          "global disabled, no frontmatter",
			globalDefault: false,
			metadata:      nil,
			want:          false,
		},
		{
			name:          "global enabled, frontmatter disables",
			globalDefault: true,
			metadata:      map[string]any{"color_chips": false},
			want:          false,
		},
		{
			name:          "global disabled, frontmatter enables",
			globalDefault: false,
			metadata:      map[string]any{"color_chips": true},
			want:          true,
		},
		{
			name:          "frontmatter with unrelated keys",
			globalDefault: true,
			metadata:      map[string]any{"title": "Test"},
			want:          true,
		},
		{
			name:          "frontmatter with non-bool value ignored",
			globalDefault: true,
			metadata:      map[string]any{"color_chips": "yes"},
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorChipsEnabled(tt.globalDefault, tt.metadata)
			if got != tt.want {
				t.Errorf("colorChipsEnabled(%v, %v) = %v, want %v",
					tt.globalDefault, tt.metadata, got, tt.want)
			}
		})
	}
}

func TestMarkdownRenderer_ColorChips_Disabled(t *testing.T) {
	ctx := context.Background()

	t.Run("globally disabled", func(t *testing.T) {
		r := NewMarkdownRenderer(MarkdownOptions{})
		result, err := r.Render(ctx, []byte("Color: `#FF5733`"), &enricher.EnrichmentData{})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when globally disabled")
		}
	})

	t.Run("globally enabled but enrichment metadata disables", func(t *testing.T) {
		r := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
		input := "---\ncolor_chips: false\n---\nColor: `#FF5733`"
		result, err := r.Render(ctx, []byte(input), &enricher.EnrichmentData{
			Metadata: map[string]any{"color_chips": false},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when enrichment metadata disables them")
		}
	})

	t.Run("globally disabled but enrichment metadata enables", func(t *testing.T) {
		r := NewMarkdownRenderer(MarkdownOptions{})
		input := "---\ncolor_chips: true\n---\nColor: `#FF5733`"
		result, err := r.Render(ctx, []byte(input), &enricher.EnrichmentData{
			Metadata: map[string]any{"color_chips": true},
		})
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should appear when enrichment metadata enables them")
		}
	})
}
