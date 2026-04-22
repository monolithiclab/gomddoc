package renderer

import (
	"bytes"
	"context"
	"log/slog"
	"mime"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"gopkg.in/yaml.v3"
)

func init() {
	// Register markdown MIME types with the standard library.
	// This ensures mime.TypeByExtension() returns "text/markdown"
	// for .md and .markdown files.
	//
	// Co-located with MarkdownRenderer as it owns this mapping.
	// Errors are ignored as these are standard MIME types that should always succeed.
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
	_ = mime.AddExtensionType(".markdown", "text/markdown; charset=utf-8")
}

// MarkdownRenderer transforms markdown content to HTML.
// It uses the gomarkdown library with CommonExtensions and AutoHeadingIDs.
//
// The renderer is stateless and thread-safe. Each Render() call creates
// fresh parser and HTML renderer instances to avoid shared state.
type MarkdownRenderer struct {
	extensions parser.Extensions
	htmlFlags  html.Flags
	htmlOpts   html.RendererOptions
}

// NewMarkdownRenderer creates a new markdown renderer with standard settings.
//
// Parser configuration:
//   - CommonExtensions: Tables, fenced code blocks, autolinks, strikethrough
//   - AutoHeadingIDs: Automatic ID generation for headings
//   - NoEmptyLineBeforeBlock: Allows lists/blocks without blank lines
//
// HTML renderer configuration:
//   - CommonFlags: Standard HTML output flags
func NewMarkdownRenderer() *MarkdownRenderer {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}

	return &MarkdownRenderer{
		extensions: extensions,
		htmlFlags:  htmlFlags,
		htmlOpts:   opts,
	}
}

// SupportedMimeTypes returns the MIME types this renderer handles.
func (m *MarkdownRenderer) SupportedMimeTypes() []string {
	return []string{"text/markdown"}
}

// Render converts markdown content to HTML.
//
// Context handling:
//   - Checks ctx.Err() before parsing (expensive operation)
//   - Checks ctx.Err() after parsing, before rendering
//   - Returns context.Canceled or context.DeadlineExceeded if cancelled
//
// Thread safety: Creates fresh parser and HTML renderer for each call.
// Both components maintain internal state and must not be shared across
// concurrent renders. This design ensures the MarkdownRenderer itself
// is stateless and thread-safe.
func (m *MarkdownRenderer) Render(ctx context.Context, content []byte) (*RenderResult, error) {
	// Check context before expensive operations
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Parse front matter
	mdContent, metadata, err := m.parseFrontMatter(content)
	if err != nil {
		// Log warning but continue rendering content
		slog.Warn("Failed to parse front matter", slog.Any("error", err))
		mdContent = content // Fallback to full content
		metadata = nil
	}

	// Create new parser for each render (library constraint - DO NOT pool)
	p := parser.NewWithExtensions(m.extensions)
	doc := p.Parse(mdContent)

	// Check context after parsing, before rendering
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Create new HTML renderer for each render (thread safety - has internal state)
	htmlRenderer := html.NewRenderer(m.htmlOpts)
	htmlOutput := markdown.Render(doc, htmlRenderer)

	return &RenderResult{
		Content:  htmlOutput,
		MimeType: "text/html; charset=utf-8",
		Metadata: metadata,
	}, nil
}

// parseFrontMatter extracts and parses YAML front matter from the content.
// It returns the remaining markdown content and the parsed metadata.
func (m *MarkdownRenderer) parseFrontMatter(content []byte) ([]byte, map[string]interface{}, error) {
	// Normalize line endings to \n to simplify parsing
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	// Check for front matter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return content, nil, nil
	}

	// Find the closing delimiter
	// We start searching from index 3 to skip the opening "---"
	end := bytes.Index(content[3:], []byte("\n---"))
	if end == -1 {
		return content, nil, nil
	}

	// Adjust end index to be relative to content start
	// end returned by Index is relative to slice [3:], so add 3
	end += 3

	// Extract YAML block (skip first 4 bytes "---\n")
	yamlStart := 4
	yamlBlock := content[yamlStart:end]

	// Skip the closing delimiter (\n---) and potential newline after it
	// We need to find where the markdown actually starts.
	// `end` points to the `\n` before `---`.
	closeDelimLen := 4 // \n---
	mdStart := end + closeDelimLen

	// Consume one optional newline after the closing delimiter
	if mdStart < len(content) && content[mdStart] == '\n' {
		mdStart++
	}

	mdContent := content[mdStart:]

	var metadata map[string]interface{}
	if err := yaml.Unmarshal(yamlBlock, &metadata); err != nil {
		return content, nil, err
	}

	return mdContent, metadata, nil
}
