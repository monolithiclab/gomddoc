package text

import "bytes"

// StripFrontmatter removes YAML frontmatter delimited by --- from markdown content,
// returning only the body. If no frontmatter is found, the original content is returned.
// The opening delimiter must be the first three bytes of the content. The closing
// delimiter must appear on its own line (preceded by \n). Exactly one trailing
// newline after the closing delimiter is consumed.
func StripFrontmatter(content []byte) []byte {
	if !bytes.HasPrefix(content, []byte("---")) {
		return content
	}

	// Find closing delimiter
	rest := content[3:]
	_, after, ok := bytes.Cut(rest, []byte("\n---"))
	if !ok {
		return content
	}

	// Skip exactly one newline after the closing delimiter
	if len(after) > 0 && after[0] == '\n' {
		return after[1:]
	}
	if len(after) > 1 && after[0] == '\r' && after[1] == '\n' {
		return after[2:]
	}
	return after
}
