package mcp

import (
	"bytes"
	"errors"
	"strings"
	"unicode"

	"github.com/monolithiclab/gomddoc/internal/text"
)

// ErrSectionNotFound is returned when the requested heading ID does not exist.
var ErrSectionNotFound = errors.New("section not found")

// ExtractSection extracts markdown content under a specific heading ID.
// Returns the heading line and all content until the next heading at the
// same or higher level (or EOF). Frontmatter is stripped before scanning.
func ExtractSection(content []byte, headingID string) ([]byte, error) {
	body := text.StripFrontmatter(content)
	lines := bytes.Split(body, []byte("\n"))

	var startIdx int
	var startLevel int
	found := false

	for i, line := range lines {
		level, text, ok := headingLevel(string(line))
		if !ok {
			continue
		}
		if slugifyHeading(text) == headingID {
			startIdx = i
			startLevel = level
			found = true
			break
		}
	}

	if !found {
		return nil, ErrSectionNotFound
	}

	// Collect lines from startIdx until next heading at same-or-higher level.
	endIdx := len(lines)
	for i := startIdx + 1; i < len(lines); i++ {
		level, _, ok := headingLevel(string(lines[i]))
		if ok && level <= startLevel {
			endIdx = i
			break
		}
	}

	result := bytes.Join(lines[startIdx:endIdx], []byte("\n"))
	return bytes.TrimRight(result, "\n"), nil
}

// headingLevel parses an ATX heading line and returns its level and text.
// For example, "## Installation" returns (2, "Installation", true).
func headingLevel(line string) (int, string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	// ATX headings allow up to 3 leading spaces.
	if len(line)-len(trimmed) > 3 {
		return 0, "", false
	}

	level := 0
	for _, c := range trimmed {
		if c == '#' {
			level++
		} else {
			break
		}
	}

	if level == 0 || level > 6 {
		return 0, "", false
	}

	rest := trimmed[level:]
	if len(rest) == 0 {
		// Bare heading like "##" with no text — valid but empty.
		return level, "", true
	}
	if rest[0] != ' ' && rest[0] != '\t' {
		// "#text" without a space is not a heading.
		return 0, "", false
	}

	text := strings.TrimSpace(rest)
	// Strip optional closing hashes: "## Heading ##" -> "Heading"
	text = strings.TrimRight(text, "#")
	text = strings.TrimRight(text, " ")
	return level, text, true
}

// slugifyHeading converts heading text to a URL-friendly anchor ID,
// matching goldmark's parser.WithAutoHeadingID() behavior:
// lowercase, non-alphanumeric characters replaced with hyphens,
// consecutive hyphens collapsed, leading/trailing hyphens trimmed.
func slugifyHeading(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	prevHyphen := true // Start true to trim leading hyphens.

	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevHyphen = false
		default:
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}

	s := b.String()
	return strings.TrimRight(s, "-")
}
