package text

import (
	"testing"
)

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard frontmatter",
			input: "---\ntitle: Hello\n---\nBody text",
			want:  "Body text",
		},
		{
			name:  "no frontmatter",
			input: "Just body text",
			want:  "Just body text",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "opening delimiter only",
			input: "---\ntitle: Hello\nNo closing",
			want:  "---\ntitle: Hello\nNo closing",
		},
		{
			name:  "CRLF line endings",
			input: "---\r\ntitle: Hello\r\n---\r\nBody text",
			want:  "Body text",
		},
		{
			name:  "LF after closing delimiter",
			input: "---\ntitle: Hello\n---\n",
			want:  "",
		},
		{
			name:  "CRLF after closing delimiter",
			input: "---\r\ntitle: Hello\r\n---\r\n",
			want:  "",
		},
		{
			name:  "no newline after closing delimiter",
			input: "---\ntitle: Hello\n---",
			want:  "",
		},
		{
			name:  "empty frontmatter",
			input: "---\n---\nBody",
			want:  "Body",
		},
		{
			name:  "multiple dashes in body",
			input: "---\ntitle: X\n---\nSome --- in body",
			want:  "Some --- in body",
		},
		{
			name:  "delimiter not at start",
			input: "hello\n---\ntitle: X\n---\nBody",
			want:  "hello\n---\ntitle: X\n---\nBody",
		},
		{
			name:  "multiple frontmatter blocks returns after first",
			input: "---\ntitle: X\n---\n---\nsecond: Y\n---\nBody",
			want:  "---\nsecond: Y\n---\nBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(StripFrontmatter([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("StripFrontmatter(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
