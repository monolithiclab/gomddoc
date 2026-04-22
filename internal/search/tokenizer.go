package search

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
)

// stripFrontmatter removes YAML frontmatter delimited by --- from markdown content,
// returning only the body. If no frontmatter is found, the original content is returned.
func stripFrontmatter(content []byte) []byte {
	trimmed := bytes.TrimLeft(content, "\xef\xbb\xbf") // strip BOM
	trimmed = bytes.TrimLeftFunc(trimmed, func(r rune) bool { return r == ' ' || r == '\t' })

	delimiter := []byte("---")
	if !bytes.HasPrefix(trimmed, delimiter) {
		return content
	}

	afterOpen := bytes.Index(trimmed, delimiter) + len(delimiter)
	if afterOpen >= len(trimmed) {
		return content
	}
	if trimmed[afterOpen] != '\n' && trimmed[afterOpen] != '\r' {
		return content
	}
	afterOpen++

	rest := trimmed[afterOpen:]
	for i := 0; i < len(rest); {
		lineEnd := bytes.IndexByte(rest[i:], '\n')
		var line []byte
		if lineEnd < 0 {
			line = rest[i:]
		} else {
			line = rest[i : i+lineEnd]
		}
		line = bytes.TrimRight(line, "\r")
		if bytes.Equal(bytes.TrimSpace(line), delimiter) {
			bodyStart := i + len(line)
			if lineEnd >= 0 {
				bodyStart = i + lineEnd + 1
			}
			return rest[bodyStart:]
		}
		if lineEnd < 0 {
			break
		}
		i += lineEnd + 1
	}

	return content
}

// Compiled regex patterns for markdown stripping.
var (
	reCodeFence  = regexp.MustCompile("(?m)^```[^\n]*\n(?s:.*?)^```\\s*$")
	reImage      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reBoldAst    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reBoldUnd    = regexp.MustCompile(`__([^_]+)__`)
	reItalicAst  = regexp.MustCompile(`\*([^*]+)\*`)
	reItalicUnd  = regexp.MustCompile(`_([^_]+)_`)
	reInlineCode = regexp.MustCompile("`([^`]+)`")
	reBlockquote = regexp.MustCompile(`(?m)^>\s?`)
	reHRule      = regexp.MustCompile(`(?m)^---+\s*$`)
	reHTMLTags   = regexp.MustCompile(`<[^>]+>`)
)

// stripMarkdown removes common markdown syntax, keeping the textual content.
func stripMarkdown(text string) string {
	// Remove code fences entirely (content is code, not prose)
	text = reCodeFence.ReplaceAllString(text, " ")
	// Images: keep alt text
	text = reImage.ReplaceAllString(text, "$1")
	// Links: keep link text
	text = reLink.ReplaceAllString(text, "$1")
	// Headings: remove # prefix
	text = reHeading.ReplaceAllString(text, "")
	// Emphasis: keep inner text (bold before italic to handle *** correctly)
	text = reBoldAst.ReplaceAllString(text, "$1")
	text = reBoldUnd.ReplaceAllString(text, "$1")
	text = reItalicAst.ReplaceAllString(text, "$1")
	text = reItalicUnd.ReplaceAllString(text, "$1")
	// Inline code: keep code text
	text = reInlineCode.ReplaceAllString(text, "$1")
	// Blockquotes: remove > prefix
	text = reBlockquote.ReplaceAllString(text, "")
	// Horizontal rules
	text = reHRule.ReplaceAllString(text, " ")
	// HTML tags
	text = reHTMLTags.ReplaceAllString(text, " ")

	return text
}

// tokenize splits text into lowercase tokens, filtering out tokens shorter than 2 characters.
func tokenize(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	result := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 2 {
			result = append(result, w)
		}
	}
	return result
}

// tokenizeToFreqs tokenizes text and returns a term frequency map and total token count.
func tokenizeToFreqs(text string) (map[string]int, int) {
	tokens := tokenize(text)
	freqs := make(map[string]int, len(tokens))
	for _, t := range tokens {
		freqs[t]++
	}
	return freqs, len(tokens)
}
