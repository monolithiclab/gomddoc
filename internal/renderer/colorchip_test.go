package renderer

import (
	"context"
	"strings"
	"testing"
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
				`class="color-chip"`,
				`class="color-chip-swatch"`,
				`style="background:#FF5733"`,
				`#FF5733`,
			},
			wantNotContain: []string{"<code>#FF5733</code>", `style="color:`},
		},
		{
			name:  "3-digit hex in code tag",
			input: `<p>Red is <code>#f00</code>.</p>`,
			wantContains: []string{
				`class="color-chip"`,
				`style="background:#f00"`,
			},
			wantNotContain: []string{"<code>#f00</code>", `style="color:`},
		},
		{
			name:  "lowercase 6-digit hex",
			input: `<code>#aabbcc</code>`,
			wantContains: []string{
				`style="background:#aabbcc"`,
			},
		},
		{
			name:  "multiple code colors in same paragraph",
			input: `<p>Mix <code>#FF0000</code> with <code>#00FF00</code>.</p>`,
			wantContains: []string{
				`style="background:#FF0000"`,
				`style="background:#00FF00"`,
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
				`class="color-chip"`,
				`style="background:#E91E63"`,
			},
			wantNotContain: []string{`style="color:`},
		},
		{
			name:  "backtick hex color in list",
			input: "- Primary: `#3366FF`\n- Secondary: `#FF6633`",
			wantContains: []string{
				`style="background:#3366FF"`,
				`style="background:#FF6633"`,
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
				`class="color-chip"`,
				`style="background:#f0f"`,
			},
		},
	}

	r := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.Render(ctx, []byte(tt.input))
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
		result, err := r.Render(ctx, []byte("Color: `#FF5733`"))
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when globally disabled")
		}
	})

	t.Run("globally enabled but frontmatter disables", func(t *testing.T) {
		r := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
		input := "---\ncolor_chips: false\n---\nColor: `#FF5733`"
		result, err := r.Render(ctx, []byte(input))
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should not appear when frontmatter disables them")
		}
	})

	t.Run("globally disabled but frontmatter enables", func(t *testing.T) {
		r := NewMarkdownRenderer(MarkdownOptions{})
		input := "---\ncolor_chips: true\n---\nColor: `#FF5733`"
		result, err := r.Render(ctx, []byte(input))
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(string(result.Content), "color-chip") {
			t.Error("color chips should appear when frontmatter enables them")
		}
	})
}
