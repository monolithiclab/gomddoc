package markdown

// Standardized markdown fixtures for testing across the codebase.
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
