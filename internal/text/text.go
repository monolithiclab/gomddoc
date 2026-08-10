// Package text holds the small string utilities shared across packages: title
// derivation and casing, frontmatter stripping, and log-value sanitization.
//
// Safe and SafeString exist for slog. A value interpolated into a log line can
// carry newlines and control characters, which is how one log entry forges a
// second; SafeString defers the scrub to slog.LogValuer, so a disabled level
// costs nothing.
package text

import (
	"cmp"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// CompareTitles is a case-insensitive comparator for two title strings, for use
// with slices.SortFunc. It is the single source of truth for title ordering,
// shared by the metadata index, search, and related-docs sorting.
//
// Equivalent to strings.Compare(strings.ToLower(a), strings.ToLower(b)) —
// UTF-8 is order-preserving, so comparing lowered runes matches comparing the
// lowered encodings byte for byte — but without the two copies ToLower makes
// whenever a title contains an uppercase rune. Titles almost always do, and
// this runs once per candidate in related-docs and O(n log n) times per sort.
func CompareTitles(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		ar, aw := utf8.DecodeRuneInString(a)
		br, bw := utf8.DecodeRuneInString(b)
		if c := cmp.Compare(unicode.ToLower(ar), unicode.ToLower(br)); c != 0 {
			return c
		}
		a, b = a[aw:], b[bw:]
	}
	return cmp.Compare(len(a), len(b))
}

// TitleCase properly handles Unicode capitalization using Title casing.
// Thread-safe: creates a new Caser for each call since cases.Caser is not
// thread-safe and maintains internal mutable state during transformation.
// Uses language.English, consistent with DeriveTitle.
func TitleCase(s string) string {
	return cases.Title(language.English).String(s)
}
