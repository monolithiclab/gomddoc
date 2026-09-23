package metadata

import "slices"

// FrontmatterField is a frontmatter key gomddoc gives special behaviour.
// Anything not listed lands in PageInfo.Meta untouched.
type FrontmatterField struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// FrontmatterFields lists every special frontmatter key, wherever it is read
// (metadata, template, seo, resolve). It is the list agents see through the
// capabilities report; a drift test holds it equal to the guide's Standard
// Fields table.
var FrontmatterFields = []FrontmatterField{
	{"title", "string", "Page title: <title>, Open Graph, search results, navigation label."},
	{"description", "string", "Page description: <meta name=description>, Open Graph, search results."},
	{"tags", "[]string", "Tags, lowercased and trimmed: tag pages, /api/tags, related pages, tag: search."},
	{"date", "date", "Publication date (YYYY-MM-DD or RFC 3339): feed, JSON-LD datePublished, sitemap fallback."},
	{"author", "string", "Author name for the page's JSON-LD structured data."},
	{"og_type", "string", "Open Graph type (default article)."},
	{"robots", "string", "Per-page <meta name=robots>; noindex also drops the page from sitemap and feed."},
	{"lang", "string", "BCP 47 language for this page's <html lang>, overriding site.language."},
	{"layout", "string", "Alternate layout template: layout: wide renders with wide.html.tmpl."},
	{"redirect_from", "[]string", "Old URL paths that 301-redirect to this page."},
	{"features", "map[string]bool", "Per-page theme feature overrides, merged over site.theme.features."},
}

// FrontmatterFieldList returns a copy of FrontmatterFields.
func FrontmatterFieldList() []FrontmatterField { return slices.Clone(FrontmatterFields) }
