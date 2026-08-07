package mcp

import (
	"errors"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/text"
)

func TestSlugifyHeading(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{"simple", "Installation", "installation"},
		{"with spaces", "Getting Started", "getting-started"},
		{"mixed case", "API Reference", "api-reference"},
		{"special chars", "Hello, World!", "hello-world"},
		{"consecutive specials", "foo---bar", "foo-bar"},
		{"leading special", " -hello", "hello"},
		{"trailing special", "hello- ", "hello"},
		{"numbers", "Phase 10a", "phase-10a"},
		{"unicode", "Über Cool", "über-cool"},
		{"empty", "", ""},
		{"only specials", "---", ""},
		{"backticks", "`config.yml`", "config-yml"},
		{"parentheses", "foo (bar)", "foo-bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := slugifyHeading(tt.text)
			if got != tt.want {
				t.Errorf("slugifyHeading(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestHeadingLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		line      string
		wantLevel int
		wantText  string
		wantOK    bool
	}{
		{"h1", "# Title", 1, "Title", true},
		{"h2", "## Section", 2, "Section", true},
		{"h3", "### Subsection", 3, "Subsection", true},
		{"h6", "###### Deep", 6, "Deep", true},
		{"too deep", "####### Invalid", 0, "", false},
		{"no space", "#NoSpace", 0, "", false},
		{"not heading", "regular text", 0, "", false},
		{"empty line", "", 0, "", false},
		{"closing hashes", "## Heading ##", 2, "Heading", true},
		{"bare heading", "##", 2, "", true},
		{"leading spaces ok", "  ## Indented", 2, "Indented", true},
		{"three spaces ok", "   ## Indented", 2, "Indented", true},
		{"too much indent", "    ## Code block", 0, "", false},
		{"leading tab is code block", "\t## Tabbed", 0, "", false},
		{"space then tab is code block", " \t## Tabbed", 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			level, heading, ok := headingLevel(tt.line)
			if ok != tt.wantOK || level != tt.wantLevel || heading != tt.wantText {
				t.Errorf("headingLevel(%q) = (%d, %q, %v), want (%d, %q, %v)",
					tt.line, level, heading, ok, tt.wantLevel, tt.wantText, tt.wantOK)
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	t.Parallel()

	const doc = `---
title: Test
---
# Main Title

Introduction text.

## Installation

Run the installer:

` + "```bash" + `
go install
` + "```" + `

### Prerequisites

You need Go 1.21+.

## Usage

Run the command:

` + "```bash" + `
gomddoc serve
` + "```" + `

## FAQ

Questions and answers.
`

	tests := []struct {
		name      string
		content   string
		headingID string
		wantErr   error
		contains  []string
		excludes  []string
	}{
		{
			name:      "extract h2 with nested h3",
			content:   doc,
			headingID: "installation",
			contains:  []string{"## Installation", "Run the installer", "### Prerequisites", "Go 1.21+"},
			excludes:  []string{"## Usage", "gomddoc serve"},
		},
		{
			name:      "extract h2 without children",
			content:   doc,
			headingID: "usage",
			contains:  []string{"## Usage", "gomddoc serve"},
			excludes:  []string{"## FAQ", "## Installation"},
		},
		{
			name:      "extract last section (EOF)",
			content:   doc,
			headingID: "faq",
			contains:  []string{"## FAQ", "Questions and answers."},
			excludes:  []string{"## Usage"},
		},
		{
			name:      "extract nested h3",
			content:   doc,
			headingID: "prerequisites",
			contains:  []string{"### Prerequisites", "Go 1.21+"},
			excludes:  []string{"## Usage", "## Installation"},
		},
		{
			name:      "extract h1",
			content:   doc,
			headingID: "main-title",
			contains:  []string{"# Main Title", "Introduction text."},
		},
		{
			name:      "heading not found",
			content:   doc,
			headingID: "nonexistent",
			wantErr:   ErrSectionNotFound,
		},
		{
			name:      "empty content",
			content:   "",
			headingID: "anything",
			wantErr:   ErrSectionNotFound,
		},
		{
			name:      "no frontmatter",
			content:   "# Hello\n\nWorld\n",
			headingID: "hello",
			contains:  []string{"# Hello", "World"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExtractSection([]byte(tt.content), tt.headingID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ExtractSection() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractSection() unexpected error: %v", err)
			}

			result := string(got)
			for _, s := range tt.contains {
				if !containsString(result, s) {
					t.Errorf("result should contain %q, got:\n%s", s, result)
				}
			}
			for _, s := range tt.excludes {
				if containsString(result, s) {
					t.Errorf("result should NOT contain %q, got:\n%s", s, result)
				}
			}
		})
	}
}

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"with frontmatter", "---\ntitle: X\n---\n# Hello", "# Hello"},
		{"no frontmatter", "# Hello\nWorld", "# Hello\nWorld"},
		{"unclosed frontmatter", "---\ntitle: X\n# Hello", "---\ntitle: X\n# Hello"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := string(text.StripFrontmatter([]byte(tt.content)))
			if got != tt.want {
				t.Errorf("text.StripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
