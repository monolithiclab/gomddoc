package search

import (
	"html"
	"strings"
	"unicode"
)

const defaultSnippetLen = 160

// span represents a character range in text.
type span struct{ start, end int }

// generateSnippet extracts a text snippet from content centered around the densest
// cluster of query terms. Matched terms are wrapped in <mark> tags. The surrounding
// text is HTML-escaped. Ellipsis is added at truncation boundaries.
func generateSnippet(content string, queryTokens []string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = defaultSnippetLen
	}
	if len(queryTokens) == 0 || content == "" {
		return truncateAtWord(content, maxLen)
	}

	lower := strings.ToLower(content)
	bestPos := findBestWindow(lower, queryTokens, maxLen)

	// Extract the window
	start := bestPos
	end := min(bestPos+maxLen, len(content))

	// Adjust to word boundaries
	if start > 0 {
		// Move start forward to next word boundary
		for start < end && !unicode.IsSpace(rune(content[start])) {
			start++
		}
		for start < end && unicode.IsSpace(rune(content[start])) {
			start++
		}
	}
	if end < len(content) {
		// Move end backward to word boundary
		for end > start && !unicode.IsSpace(rune(content[end-1])) {
			end--
		}
	}

	if start >= end {
		// Fallback: just take from the beginning
		return truncateAtWord(content, maxLen)
	}

	snippet := content[start:end]
	snippet = strings.TrimSpace(snippet)

	// Highlight query terms
	snippet = highlightTerms(snippet, queryTokens)

	// Add ellipsis
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet += "..."
	}

	return snippet
}

// findBestWindow finds the starting position of the window with the highest
// density of query term occurrences.
func findBestWindow(lowerContent string, queryTokens []string, windowSize int) int {
	if len(lowerContent) <= windowSize {
		return 0
	}

	bestPos := 0
	bestScore := -1

	// Sample positions at regular intervals for efficiency
	step := max(len(lowerContent)/50, 1)

	for pos := 0; pos <= len(lowerContent)-windowSize; pos += step {
		window := lowerContent[pos : pos+windowSize]
		score := 0
		for _, token := range queryTokens {
			score += strings.Count(window, token)
		}
		if score > bestScore {
			bestScore = score
			bestPos = pos
		}
	}

	return bestPos
}

// highlightTerms wraps occurrences of query tokens in <mark> tags.
// The surrounding text is HTML-escaped.
func highlightTerms(text string, queryTokens []string) string {
	// Build a set of token positions to highlight
	lower := strings.ToLower(text)
	var spans []span

	for _, token := range queryTokens {
		offset := 0
		for {
			idx := strings.Index(lower[offset:], token)
			if idx < 0 {
				break
			}
			absStart := offset + idx
			absEnd := absStart + len(token)
			// Only match at word boundaries
			if isWordBoundary(lower, absStart) && isWordBoundary(lower, absEnd) {
				spans = append(spans, span{absStart, absEnd})
			}
			offset = absEnd
		}
	}

	if len(spans) == 0 {
		return html.EscapeString(text)
	}

	// Sort spans by start position and merge overlaps
	sortSpans(spans)
	merged := mergeSpans(spans)

	// Build result with highlighting
	var b strings.Builder
	prev := 0
	for _, s := range merged {
		b.WriteString(html.EscapeString(text[prev:s.start]))
		b.WriteString("<mark>")
		b.WriteString(html.EscapeString(text[s.start:s.end]))
		b.WriteString("</mark>")
		prev = s.end
	}
	b.WriteString(html.EscapeString(text[prev:]))

	return b.String()
}

// isWordBoundary reports whether position pos in text is at a word boundary.
func isWordBoundary(text string, pos int) bool {
	if pos <= 0 || pos >= len(text) {
		return true
	}
	return !unicode.IsLetter(rune(text[pos-1])) || !unicode.IsLetter(rune(text[pos]))
}

// sortSpans sorts spans by start position using insertion sort (small slices).
func sortSpans(spans []span) {
	for i := 1; i < len(spans); i++ {
		key := spans[i]
		j := i - 1
		for j >= 0 && spans[j].start > key.start {
			spans[j+1] = spans[j]
			j--
		}
		spans[j+1] = key
	}
}

// mergeSpans merges overlapping spans.
func mergeSpans(spans []span) []span {
	if len(spans) == 0 {
		return nil
	}
	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}
	return merged
}

// truncateAtWord truncates text to maxLen characters at a word boundary.
func truncateAtWord(text string, maxLen int) string {
	if len(text) <= maxLen {
		return html.EscapeString(text)
	}
	end := maxLen
	for end > 0 && !unicode.IsSpace(rune(text[end-1])) {
		end--
	}
	if end == 0 {
		end = maxLen
	}
	return html.EscapeString(strings.TrimSpace(text[:end])) + "..."
}
