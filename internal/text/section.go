package text

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"unicode"
)

// ErrSectionNotFound is returned when the requested heading ID does not exist.
var ErrSectionNotFound = errors.New("section not found")

// ExtractSection extracts markdown content under a specific heading ID.
// Returns the heading line and all content until the next heading at the
// same or higher level (or EOF). Frontmatter is stripped before scanning.
func ExtractSection(content []byte, headingID string) ([]byte, error) {
	lines, heads := scanHeadings(content)

	start := slices.IndexFunc(heads, func(h heading) bool { return slugifyHeading(h.text) == headingID })
	if start < 0 {
		return nil, ErrSectionNotFound
	}

	// The section runs to the next heading at the same or higher level.
	endIdx := len(lines)
	for _, h := range heads[start+1:] {
		if h.level <= heads[start].level {
			endIdx = h.line
			break
		}
	}

	result := bytes.Join(lines[heads[start].line:endIdx], []byte("\n"))
	return bytes.TrimRight(result, "\n"), nil
}

// heading is an ATX heading found by scanHeadings.
type heading struct {
	line  int // index into the lines scanHeadings returned
	level int
	text  string
}

// scanHeadings splits content (frontmatter stripped) into lines and returns
// its headings. Lines inside fenced code blocks are skipped: a "# comment" in
// a YAML or shell example is code, and treating it as a level-1 heading ends
// the enclosing section at the comment.
func scanHeadings(content []byte) ([][]byte, []heading) {
	lines := bytes.Split(StripFrontmatter(content), []byte("\n"))
	var heads []heading
	var fence string // the open fence's run of ` or ~; "" outside a block
	for i, line := range lines {
		if run, ok := fenceRun(string(line)); ok {
			switch {
			case fence == "":
				fence = run
			case run[0] == fence[0] && len(run) >= len(fence):
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if level, title, ok := headingLevel(string(line)); ok {
			heads = append(heads, heading{i, level, title})
		}
	}
	return lines, heads
}

// fenceRun reports whether line opens or closes a fenced code block
// (CommonMark: up to 3 spaces, then 3+ backticks or tildes) and returns the
// run of fence characters.
func fenceRun(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return "", false
	}
	n := len(trimmed) - len(strings.TrimLeft(trimmed, trimmed[:1]))
	if n < 3 {
		return "", false
	}
	return trimmed[:n], true
}

// headingLevel parses an ATX heading line and returns its level and text.
// For example, "## Installation" returns (2, "Installation", true).
func headingLevel(line string) (int, string, bool) {
	// ATX headings allow up to 3 spaces of indentation. A tab counts as 4
	// (CommonMark), so any leading tab makes the line indented code, not a
	// heading. Measure the leading indent treating tabs as 4 before trimming.
	indent := 0
	for _, c := range line {
		switch c {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
		}
		if c != ' ' && c != '\t' {
			break
		}
	}
	if indent > 3 {
		return 0, "", false
	}
	trimmed := strings.TrimLeft(line, " \t")

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

	heading := strings.TrimSpace(rest)
	// Strip optional closing hashes: "## Heading ##" -> "Heading"
	heading = strings.TrimRight(heading, "#")
	heading = strings.TrimRight(heading, " ")
	return level, heading, true
}

// slugifyHeading converts heading text to a URL-friendly anchor ID,
// matching goldmark's parser.WithAutoHeadingID() behavior:
// lowercase, non-alphanumeric characters replaced with hyphens,
// consecutive hyphens collapsed, leading/trailing hyphens trimmed.
func slugifyHeading(heading string) string {
	var b strings.Builder
	b.Grow(len(heading))
	prevHyphen := true // Start true to trim leading hyphens.

	for _, r := range heading {
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

// HeadingIDs lists the anchor IDs ExtractSection accepts for content, in
// document order. It shares ExtractSection's scanner, so every ID it returns
// is extractable and none comes from inside a code block.
func HeadingIDs(content []byte) []string {
	_, heads := scanHeadings(content)
	ids := make([]string, 0, len(heads))
	for _, h := range heads {
		ids = append(ids, slugifyHeading(h.text))
	}
	return ids
}
