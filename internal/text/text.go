package text

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// CompareTitles is a case-insensitive comparator for two title strings, for use
// with slices.SortFunc. It is the single source of truth for title ordering,
// shared by the metadata index, search, and related-docs sorting.
func CompareTitles(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// TitleCase properly handles Unicode capitalization using Title casing.
// Thread-safe: creates a new Caser for each call since cases.Caser is not
// thread-safe and maintains internal mutable state during transformation.
// Uses language.English, consistent with DeriveTitle.
func TitleCase(s string) string {
	return cases.Title(language.English).String(s)
}
