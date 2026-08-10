package metadata

import "time"

// ParseFrontmatterDate interprets a raw frontmatter `date` value, returning the
// zero time when there is nothing usable. It exists so the index and the
// JSON-LD builder read the same key the same way: the index consumes YAML
// already decoded by gopkg.in/yaml.v3, the renderer consumes the map
// goldmark-meta produced, and both hand back whichever of time.Time or string
// the document happened to yield — an unquoted `date: 2024-01-01` decodes to a
// time.Time, a quoted one stays a string.
//
// RFC 3339 is accepted alongside the bare date because a quoted timestamp is
// the natural way to write one and silently dropping it produced a page with no
// date at all rather than a parse error anyone could see.
func ParseFrontmatterDate(v any) time.Time {
	switch d := v.(type) {
	case time.Time:
		return d
	case string:
		if t, err := time.Parse(time.DateOnly, d); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, d); err == nil {
			return t
		}
	}
	return time.Time{}
}
