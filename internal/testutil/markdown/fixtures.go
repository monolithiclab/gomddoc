// Package markdown holds the Markdown fixtures shared across the repository's
// tests, so a test that needs "a file with valid frontmatter" reaches for the
// existing one instead of hand-rolling a fourth slightly different spelling.
package markdown

const (
	// ValidFrontmatterWithBody is a standard markdown file with valid YAML frontmatter and content.
	ValidFrontmatterWithBody = `---
title: Hello World
tags: [a, b]
---
# Content`

	// SimpleHeading is a markdown string with just a heading.
	SimpleHeading = "# Just a heading"

	// NoFrontmatter is a markdown string without frontmatter.
	NoFrontmatter = "# No Frontmatter"

	// MalformedFrontmatter is a markdown string with invalid YAML.
	MalformedFrontmatter = "---\ntitle: [unclosed bracket\n---\n# Body"

	// EmptyContent is just frontmatter without body.
	EmptyContent = "---\ntitle: Empty\n---"
)
