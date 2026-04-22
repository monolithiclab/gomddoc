package renderer

import (
	"context"
	"strings"
	"testing"
)

func TestMarkdownRenderer_SyntaxHighlighting(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
	}{
		{
			name:  "go code block",
			input: "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
			wantContains: []string{
				"<pre",
				"<code",
				"fmt",     // function name preserved
				"Println", // method name preserved
			},
		},
		{
			name:  "python code block",
			input: "```python\ndef greet(name):\n    print(f\"Hello {name}\")\n```",
			wantContains: []string{
				"<pre",
				"<code",
				"greet",
				"print",
			},
		},
		{
			name:  "javascript code block",
			input: "```js\nconst x = 42;\nconsole.log(x);\n```",
			wantContains: []string{
				"<pre",
				"<code",
				"console",
			},
		},
		{
			name:  "inline styles present",
			input: "```go\npackage main\n```",
			wantContains: []string{
				"style=", // inline styles from Chroma
			},
		},
		{
			name:  "unknown language falls back gracefully",
			input: "```unknownlang\nsome content here\n```",
			wantContains: []string{
				"<pre",
				"<code",
				"some content here",
			},
		},
		{
			name:  "no language specified",
			input: "```\nplain code block\n```",
			wantContains: []string{
				"<pre",
				"<code",
				"plain code block",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewMarkdownRenderer(MarkdownOptions{ColorChips: true})
			result, err := renderer.Render(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			output := string(result.Content)
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestMarkdownRenderer_HighlightThemes(t *testing.T) {
	themes := []string{"github", "monokai", "dracula", ""}

	input := []byte("```go\nfunc main() {}\n```")

	for _, theme := range themes {
		t.Run("theme_"+theme, func(t *testing.T) {
			renderer := NewMarkdownRenderer(MarkdownOptions{HighlightTheme: theme, ColorChips: true})
			result, err := renderer.Render(context.Background(), input)
			if err != nil {
				t.Fatalf("Render() with theme %q error = %v", theme, err)
			}

			output := string(result.Content)
			if !strings.Contains(output, "<pre") {
				t.Errorf("theme %q: output missing <pre>\nGot: %s", theme, output)
			}
			if !strings.Contains(output, "style=") {
				t.Errorf("theme %q: output missing inline styles\nGot: %s", theme, output)
			}
		})
	}
}
