package text

import (
	"sync"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	titleCaser     cases.Caser
	titleCaserOnce sync.Once
)

// TitleCase properly handles Unicode capitalization using Title casing.
// The caser is initialized once and cached for performance.
// Uses language.Und (undetermined language) for generic capitalization.
func TitleCase(s string) string {
	titleCaserOnce.Do(func() {
		titleCaser = cases.Title(language.Und)
	})
	return titleCaser.String(s)
}
