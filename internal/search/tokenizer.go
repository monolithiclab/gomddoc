package search

import (
	"regexp"
	"strings"
	"unicode"
)

// Reusable byte slices for regex replacements.
var (
	replSpace    = []byte(" ")
	replCapture1 = []byte("$1")
)

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
// Uses []byte operations to avoid intermediate string allocations.
func stripMarkdown(text string) string {
	b := []byte(text)
	// Remove code fences entirely (content is code, not prose)
	b = reCodeFence.ReplaceAll(b, replSpace)
	// Images: keep alt text
	b = reImage.ReplaceAll(b, replCapture1)
	// Links: keep link text
	b = reLink.ReplaceAll(b, replCapture1)
	// Headings: remove # prefix
	b = reHeading.ReplaceAll(b, nil)
	// Emphasis: keep inner text (bold before italic to handle *** correctly)
	b = reBoldAst.ReplaceAll(b, replCapture1)
	b = reBoldUnd.ReplaceAll(b, replCapture1)
	b = reItalicAst.ReplaceAll(b, replCapture1)
	b = reItalicUnd.ReplaceAll(b, replCapture1)
	// Inline code: keep code text
	b = reInlineCode.ReplaceAll(b, replCapture1)
	// Blockquotes: remove > prefix
	b = reBlockquote.ReplaceAll(b, nil)
	// Horizontal rules
	b = reHRule.ReplaceAll(b, replSpace)
	// HTML tags
	b = reHTMLTags.ReplaceAll(b, replSpace)

	return string(b)
}

const minTokenBytes = 2

// tokenize splits text into lowercase tokens, dropping any token under
// minTokenBytes. Bytes, not runes: what the threshold is really for is dropping
// single ASCII letters, and counting runes would take single CJK ideographs —
// three bytes, and meaningful search terms — with them. It is a proxy, so it is
// lenient in the other direction too: a one-rune Cyrillic or Greek stopword is
// two bytes and survives. Index and query both go through here, so the two
// agree whichever way it errs.
func tokenize(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	result := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= minTokenBytes {
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
