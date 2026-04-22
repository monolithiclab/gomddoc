package text

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TitleCase properly handles Unicode capitalization using Title casing.
// Thread-safe: creates a new Caser for each call since cases.Caser is not
// thread-safe and maintains internal mutable state during transformation.
// Uses language.Und (undetermined language) for generic capitalization.
func TitleCase(s string) string {
	return cases.Title(language.Und).String(s)
}
