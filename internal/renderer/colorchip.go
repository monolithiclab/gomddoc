package renderer

import "regexp"

// colorCodePattern matches a <code> tag whose sole content is a hex color code
// (3-digit or 6-digit). This corresponds to backtick-wrapped hex codes in
// markdown, e.g. `#FF5733` or `#f00`.
var colorCodePattern = regexp.MustCompile(
	`<code>(#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3}))</code>`,
)

// transformColorChips replaces <code>#HEX</code> elements with <color-chip>
// web components. The custom element handles its own rendering via shadow DOM.
//
// Only <code> tags containing exactly a hex color code are transformed.
// Code blocks with other content, fenced code blocks, and colors in plain
// text are left unchanged.
func transformColorChips(html []byte) []byte {
	return colorCodePattern.ReplaceAllFunc(html, func(match []byte) []byte {
		sub := colorCodePattern.FindSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		hex := string(sub[1]) // e.g. "#FF5733"
		return []byte(`<color-chip>` + hex + `</color-chip>`)
	})
}
