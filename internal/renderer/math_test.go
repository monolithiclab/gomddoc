package renderer

import (
	"context"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/enricher"
)

func renderMath(t *testing.T, features map[string]bool, pageFeatures map[string]bool, input string) string {
	t.Helper()
	r := NewMarkdownRenderer(MarkdownOptions{Features: features})
	result, err := r.Render(context.Background(), []byte(input), &enricher.EnrichmentData{Features: pageFeatures})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return string(result.Content)
}

func TestMath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "dollar amounts stay text",
			input: "Between $1,200 and $3,400, or $5 and $10.",
			want:  "<p>Between $1,200 and $3,400, or $5 and $10.</p>\n",
		},
		{
			name:  "single dollar delimiters are not math",
			input: `Energy: $E = mc^2$.`,
			want:  "<p>Energy: $E = mc^2$.</p>\n",
		},
		{
			name:  "inline math",
			input: "Energy: $`E = mc^2`$.",
			want:  "<p>Energy: <gmd-math>E = mc^2</gmd-math>.</p>\n",
		},
		{
			name:  "inline math keeps backslashes and escapes HTML",
			input: "$`\\frac{a}{b} < \\sqrt{x} & y`$",
			want:  "<p><gmd-math>\\frac{a}{b} &lt; \\sqrt{x} &amp; y</gmd-math></p>\n",
		},
		{
			name:  "inline math with double backticks",
			input: "$``a`b``$",
			want:  "<p><gmd-math>a`b</gmd-math></p>\n",
		},
		{
			name:  "adjacent inline math shares no sign",
			input: "$`a`$`b`$",
			want:  "<p><gmd-math>a</gmd-math><code>b</code>$</p>\n",
		},
		{
			name:  "escaped opening sign keeps code",
			input: "\\$`a`$",
			want:  "<p>$<code>a</code>$</p>\n",
		},
		{
			name:  "escaped backslash before the sign is math",
			input: "\\\\$`a`$",
			want:  "<p>\\<gmd-math>a</gmd-math></p>\n",
		},
		{
			name:  "one-sided sign keeps code",
			input: "Price $`5` or `5`$",
			want:  "<p>Price $<code>5</code> or <code>5</code>$</p>\n",
		},
		{
			name:  "code span containing dollars is code",
			input: "Hashes look like `$2y$`.",
			want:  "<p>Hashes look like <code>$2y$</code>.</p>\n",
		},
		{
			name:  "display math block",
			input: "$$\n\\int_0^1 x_i^2 \\, dx\n\\\\ a * b * c\n$$",
			want:  "<gmd-math display>\\int_0^1 x_i^2 \\, dx\n\\\\ a * b * c</gmd-math>\n",
		},
		{
			name:  "display math on one line",
			input: "$$ x < y $$",
			want:  "<gmd-math display>x &lt; y</gmd-math>\n",
		},
		{
			name:  "display math with content on the marker lines",
			input: "$$ a = b\n+ c\nd $$\n\nafter",
			want:  "<gmd-math display>a = b\n+ c\nd</gmd-math>\n<p>after</p>\n",
		},
		{
			name:  "display math interrupts a paragraph",
			input: "Gaussian:\n$$\ne^{-x^2}\n$$\nDone.",
			want:  "<p>Gaussian:</p>\n<gmd-math display>e^{-x^2}</gmd-math>\n<p>Done.</p>\n",
		},
		{
			name:  "display math keeps blank lines out",
			input: "$$\na\n\nb\n$$",
			want:  "<gmd-math display>a\nb</gmd-math>\n",
		},
		{
			name:  "display math in a blockquote",
			input: "> $$\n> x\n> $$",
			want:  "<blockquote>\n<gmd-math display>x</gmd-math>\n</blockquote>\n",
		},
		{
			name:  "display math in a list item",
			input: "- item\n\n  $$\n  x\n  $$",
			want:  "<ul>\n<li>\n<p>item</p>\n<gmd-math display>x</gmd-math>\n</li>\n</ul>\n",
		},
		{
			name:  "display math with CRLF line endings",
			input: "$$\r\na\r\nb $$\r\n",
			want:  "<gmd-math display>a\nb</gmd-math>\n",
		},
		{
			name:  "double dollar mid-paragraph is text",
			input: "It costs $$5 here.",
			want:  "<p>It costs $$5 here.</p>\n",
		},
		{
			name:  "dollars in fenced code are code",
			input: "```\n$$\nx\n$$\n```",
			want:  "<pre><code>$$\nx\n$$\n</code></pre>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderMath(t, nil, nil, tt.input); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestMath_Disabled(t *testing.T) {
	t.Parallel()

	const input = "$`x`$\n\n$$\ny\n$$"
	const want = "<p>$<code>x</code>$</p>\n<p>$$\ny\n$$</p>\n"

	tests := []struct {
		name         string
		features     map[string]bool
		pageFeatures map[string]bool
		want         string
	}{
		{name: "site off", features: map[string]bool{"katex": false}, want: want},
		{name: "page off", pageFeatures: map[string]bool{"katex": false}, want: want},
		{
			name:         "page on over site off",
			features:     map[string]bool{"katex": false},
			pageFeatures: map[string]bool{"katex": true},
			want:         "<p><gmd-math>x</gmd-math></p>\n<gmd-math display>y</gmd-math>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderMath(t, tt.features, tt.pageFeatures, input); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}
