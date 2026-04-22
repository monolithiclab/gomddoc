package renderer

import (
	"bytes"
	"regexp"
	"strings"
)

// admonitionType represents a supported GitHub-style admonition type.
type admonitionType struct {
	// Name is the canonical lowercase name used in CSS classes.
	Name string
	// Title is the display title shown in the rendered admonition.
	Title string
}

// supportedAdmonitions maps uppercase marker names to their display properties.
var supportedAdmonitions = map[string]admonitionType{
	"NOTE":      {Name: "note", Title: "Note"},
	"TIP":       {Name: "tip", Title: "Tip"},
	"IMPORTANT": {Name: "important", Title: "Important"},
	"WARNING":   {Name: "warning", Title: "Warning"},
	"CAUTION":   {Name: "caution", Title: "Caution"},
}

// admonitionPattern matches a blockquote whose first paragraph starts with [!TYPE].
// It captures:
//   - Group 1: The admonition type (e.g., NOTE, WARNING)
//   - Group 2: Optional remaining content after the [!TYPE] marker (may span multiple lines)
//   - Group 3: Any additional content after the first paragraph within the blockquote
//
// The pattern handles both single-line and multi-line blockquote content,
// and accounts for optional newlines after the marker.
var admonitionPattern = regexp.MustCompile(
	`(?is)<blockquote>\s*<p>\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*\n?(.*?)</p>(.*?)</blockquote>`,
)

// TransformAdmonitions converts GitHub-style admonition blockquotes in rendered HTML
// to styled admonition divs.
//
// It transforms patterns like:
//
//	<blockquote><p>[!NOTE]
//	Content here</p></blockquote>
//
// Into:
//
//	<div class="admonition admonition-note"><p class="admonition-title">Note</p>
//	<p>Content here</p></div>
func TransformAdmonitions(html []byte) []byte {
	return admonitionPattern.ReplaceAllFunc(html, func(match []byte) []byte {
		submatches := admonitionPattern.FindSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		typeKey := strings.ToUpper(string(submatches[1]))
		adm, ok := supportedAdmonitions[typeKey]
		if !ok {
			return match
		}

		content := bytes.TrimSpace(submatches[2])
		extra := bytes.TrimSpace(submatches[3])

		var buf bytes.Buffer
		buf.WriteString(`<div class="admonition admonition-`)
		buf.WriteString(adm.Name)
		buf.WriteString(`"><p class="admonition-title">`)
		buf.WriteString(adm.Title)
		buf.WriteString(`</p>`)

		if len(content) > 0 {
			buf.WriteString("\n<p>")
			buf.Write(content)
			buf.WriteString("</p>")
		}

		if len(extra) > 0 {
			buf.WriteByte('\n')
			buf.Write(extra)
		}

		buf.WriteString("\n</div>")

		return buf.Bytes()
	})
}
