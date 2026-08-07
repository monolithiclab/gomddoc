package search

import (
	"cmp"
	"html"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
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

	bestPos := findBestWindow(content, queryTokens, maxLen)

	// Extract the window
	start := bestPos
	end := min(bestPos+maxLen, len(content))

	// Adjust to word boundaries (rune-aware)
	if start > 0 {
		// Move start forward to next word boundary
		for start < end {
			r, size := utf8.DecodeRuneInString(content[start:])
			if unicode.IsSpace(r) {
				break
			}
			start += size
		}
		for start < end {
			r, size := utf8.DecodeRuneInString(content[start:])
			if !unicode.IsSpace(r) {
				break
			}
			start += size
		}
	}
	if end < len(content) {
		// Move end backward to word boundary
		for end > start {
			r, size := utf8.DecodeLastRuneInString(content[:end])
			if unicode.IsSpace(r) {
				break
			}
			end -= size
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

// alignRuneStart returns the smallest offset >= from that begins a rune in s,
// or len(s) if the walk runs off the end.
//
// findBestWindow aligns twice per iteration, and the two copies had drifted:
// the window start was bounded by `from > 0` — always true once the walk has
// advanced a byte — so a window small enough to leave the start inside the
// body's last multi-byte rune indexed past the end of the string. One helper
// means one bound.
func alignRuneStart(s string, from int) int {
	for from < len(s) && !utf8.RuneStart(s[from]) {
		from++
	}
	return from
}

// findBestWindow finds the starting byte position of the window with the
// highest density of query term occurrences.
func findBestWindow(content string, queryTokens []string, windowSize int) int {
	if len(content) <= windowSize {
		return 0
	}

	// Lowered once for the whole body, then sliced per window: the sampling
	// loop below runs ~50 times, and lowering each window separately allocated
	// a copy every iteration. strings.ToLower returns content itself when there
	// is nothing to fold, so an all-lowercase body costs nothing at all.
	//
	// Case folding that changes byte length (İ is 2 bytes, folds to 'i') shifts
	// every position after it, so lower[pos:end] would score a region that is
	// not content[pos:end] while we return pos — and runs past the end of lower
	// once enough has been lost. Those bodies keep the per-window lowering.
	lower := strings.ToLower(content)
	aligned := len(lower) == len(content)

	bestPos := 0
	bestScore := -1

	// Sample positions at regular intervals for efficiency
	step := max(len(content)/50, 1)

	for pos := 0; pos <= len(content)-windowSize; pos += step {
		pos = alignRuneStart(content, pos)
		if pos > len(content)-windowSize {
			break
		}
		end := alignRuneStart(content, pos+windowSize)
		window := content[pos:end]
		if aligned {
			window = lower[pos:end]
		} else {
			window = strings.ToLower(window)
		}
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
// The surrounding text is HTML-escaped. Matching is case-insensitive,
// with a fallback path for Unicode case folding that changes byte length.
func highlightTerms(text string, queryTokens []string) string {
	var spans []span

	// Lowered once, not once per token: the fold does not depend on the token.
	lower := strings.ToLower(text)
	for _, token := range queryTokens {
		spans = findTokenSpans(text, lower, token, spans)
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

// findTokenSpans finds all case-insensitive occurrences of token in text at
// word boundaries, appending to spans. lower must be strings.ToLower(text);
// the caller folds once and reuses it across tokens. Positions refer to bytes
// in text (not the lowered copy), avoiding byte-length mismatches from case
// folding.
func findTokenSpans(text, lower, token string, spans []span) []span {
	// If lowering changed byte length, fall back to rune-mapped search.
	if len(lower) != len(text) {
		return findTokenSpansRunewise(text, lower, token, spans)
	}

	offset := 0
	for {
		idx := strings.Index(lower[offset:], token)
		if idx < 0 {
			break
		}
		absStart := offset + idx
		absEnd := absStart + len(token)
		if isWordBoundary(lower, absStart) && isWordBoundary(lower, absEnd) {
			spans = append(spans, span{absStart, absEnd})
		}
		offset = absEnd
	}
	return spans
}

// findTokenSpansRunewise is the slow path for texts where ToLower changes byte
// length. It maps byte positions from the lowered text back to the original
// using rune-to-byte offset tables.
func findTokenSpansRunewise(text, lower, token string, spans []span) []span {
	// Build rune-to-byte offset table for original text.
	runeOffsets := make([]int, 0, utf8.RuneCountInString(text)+1)
	for i := 0; i < len(text); {
		runeOffsets = append(runeOffsets, i)
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	runeOffsets = append(runeOffsets, len(text))

	// Build rune-to-byte offset table for lowered text.
	lowerRuneOffsets := make([]int, 0, len(runeOffsets))
	for i := 0; i < len(lower); {
		lowerRuneOffsets = append(lowerRuneOffsets, i)
		_, size := utf8.DecodeRuneInString(lower[i:])
		i += size
	}
	lowerRuneOffsets = append(lowerRuneOffsets, len(lower))

	// Find token in lowered text, map positions back to original via rune index.
	offset := 0
	for {
		idx := strings.Index(lower[offset:], token)
		if idx < 0 {
			break
		}
		lowerStart := offset + idx
		lowerEnd := lowerStart + len(token)

		// Convert lower byte positions to rune indices.
		startRune := byteToRuneIndex(lowerRuneOffsets, lowerStart)
		endRune := byteToRuneIndex(lowerRuneOffsets, lowerEnd)

		if startRune >= 0 && endRune >= 0 && endRune <= len(runeOffsets)-1 {
			absStart := runeOffsets[startRune]
			absEnd := runeOffsets[endRune]
			if isWordBoundary(text, absStart) && isWordBoundary(text, absEnd) {
				spans = append(spans, span{absStart, absEnd})
			}
		}
		offset = lowerEnd
	}
	return spans
}

// byteToRuneIndex finds the rune index for a byte offset using binary search
// on a pre-built, sorted offset table.
func byteToRuneIndex(offsets []int, bytePos int) int {
	i, found := slices.BinarySearch(offsets, bytePos)
	if found {
		return i
	}
	return -1
}

// isWordBoundary reports whether position pos in text is at a word boundary.
func isWordBoundary(text string, pos int) bool {
	if pos <= 0 || pos >= len(text) {
		return true
	}
	prevR, _ := utf8.DecodeLastRuneInString(text[:pos])
	nextR, _ := utf8.DecodeRuneInString(text[pos:])
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	return !isWord(prevR) || !isWord(nextR)
}

// sortSpans sorts spans by start position using insertion sort (small slices).
func sortSpans(spans []span) {
	slices.SortFunc(spans, func(a, b span) int {
		return cmp.Compare(a.start, b.start)
	})
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

// truncateAtWord truncates text to maxLen bytes at a word boundary.
func truncateAtWord(text string, maxLen int) string {
	if len(text) <= maxLen {
		return html.EscapeString(text)
	}
	end := maxLen
	// Back up to avoid splitting a multi-byte rune at the boundary
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	runeAligned := end
	for end > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:end])
		if unicode.IsSpace(r) {
			break
		}
		end -= size
	}
	if end == 0 {
		end = runeAligned
	}
	return html.EscapeString(strings.TrimSpace(text[:end])) + "..."
}
