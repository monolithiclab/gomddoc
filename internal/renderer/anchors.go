package renderer

import "regexp"

// headingWithIDPattern matches opening heading tags that have an id attribute,
// and captures: (1) the tag prefix including id value, (2) the heading level number,
// (3) the id value, (4) the inner content, (5) the closing tag.
//
// Example match: <h2 id="foo">Title</h2>
//   - Group 1: <h2 id="foo">
//   - Group 2: 2
//   - Group 3: foo
//   - Group 4: Title
//   - Group 5: </h2>
var headingWithIDPattern = regexp.MustCompile(
	`(<h([1-6])\s+id="([^"]+)"[^>]*>)(.*?)(</h[1-6]>)`,
)

// addHeadingAnchors post-processes rendered HTML to insert anchor links
// into headings that have an id attribute. The anchor link uses the "#"
// symbol and is revealed on hover via CSS.
//
// Only headings with id attributes are modified. Headings without ids
// are left unchanged.
func addHeadingAnchors(html []byte) []byte {
	return headingWithIDPattern.ReplaceAll(html, []byte(
		`${1}${4} <a href="#${3}" class="heading-anchor" aria-hidden="true" tabindex="-1">#</a>${5}`,
	))
}
