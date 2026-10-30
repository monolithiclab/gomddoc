package seo

import "strings"

// Noindex reports whether a page's robots frontmatter asks search engines not
// to index it, which drops the page from the sitemap, the feed and llms.txt.
func Noindex(meta map[string]any) bool {
	robots, ok := meta["robots"].(string)
	return ok && strings.Contains(robots, "noindex")
}
