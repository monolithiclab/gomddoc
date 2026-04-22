package processor

import (
	"strings"
	"testing"
)

func TestNewMarkdownProcessor(t *testing.T) {
	processor := NewMarkdownProcessor()
	if processor == nil {
		t.Fatal("Processor should not be nil")
	}

	if processor.ContentType() != "text/html; charset=utf-8" {
		t.Errorf("Expected content type 'text/html; charset=utf-8', got %q", processor.ContentType())
	}
}

func TestMarkdownProcessorProcess(t *testing.T) {
	processor := NewMarkdownProcessor()

	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "basic header",
			input:    "# Test Header",
			contains: []string{"<h1", "Test Header", "</h1>"},
		},
		{
			name:     "paragraph",
			input:    "This is a paragraph.",
			contains: []string{"<p>", "This is a paragraph.", "</p>"},
		},
		{
			name:     "code block",
			input:    "```go\nfunc main() {}\n```",
			contains: []string{"<pre>", "<code", "func main()"},
		},
		{
			name:     "link",
			input:    "[link text](http://example.com)",
			contains: []string{"<a", "href=\"http://example.com\"", "link text", "</a>"},
		},
		{
			name:     "complex markdown",
			input:    "# Header\n\n**Bold** and *italic*\n\n- List item\n- Another item",
			contains: []string{"<h1", "<strong>", "<em>", "<ul>", "<li>"},
		},
		{
			name:     "empty input",
			input:    "",
			contains: []string{}, // Empty content should render as empty HTML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Process([]byte(tt.input))
			if err != nil {
				t.Errorf("Process should not return error, got: %v", err)
			}

			resultStr := string(result)
			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected HTML to contain %q, got: %s", expected, resultStr)
				}
			}
		})
	}
}

func TestMarkdownProcessorSpecialCharacters(t *testing.T) {
	processor := NewMarkdownProcessor()

	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "markdown with special characters",
			input:    "# Test with símβols & entities\n\n> Quote block",
			contains: []string{"<h1", "símβols", "&amp;", "<blockquote>"},
		},
		{
			name:     "table",
			input:    "| Table | Header |\n|-------|--------|\n| Cell  | Data   |",
			contains: []string{"<table>", "<th>", "<td>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.Process([]byte(tt.input))
			if err != nil {
				t.Errorf("Process should not return error, got: %v", err)
			}

			resultStr := string(result)
			for _, expected := range tt.contains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected HTML to contain %q, got: %s", expected, resultStr)
				}
			}
		})
	}
}
