package text

import (
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// DeriveTitle derives a human-readable title from a URL path.
// It extracts the base filename, strips the extension, replaces
// hyphens and underscores with spaces, and applies English title casing.
//
// Example: "/docs/my-page.md" -> "My Page"
func DeriveTitle(reqPath string) string {
	base := path.Base(reqPath)
	if base == "." || base == "/" {
		return "Home"
	}

	// Remove extension
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Replace hyphens/underscores with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Capitalize Title Case
	return cases.Title(language.English).String(name)
}
