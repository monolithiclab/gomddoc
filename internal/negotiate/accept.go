package negotiate

import (
	"sort"
	"strconv"
	"strings"
)

// MediaType represents a parsed MIME type from an Accept header.
// It includes the type, subtype, quality factor, and full MIME type string.
type MediaType struct {
	Type    string  // Main type (e.g., "text")
	Subtype string  // Subtype (e.g., "html")
	Q       float64 // Quality factor (0.0-1.0), defaults to 1.0
}

// Matches checks if this MediaType matches the given MIME type.
//
// Wildcard support:
//   - "*/*" matches any MIME type
//   - "text/*" matches "text/html", "text/plain", etc.
//   - "text/html" matches only "text/html"
//
// The mimeType parameter should be normalized (no charset or other parameters).
func (mt MediaType) Matches(mimeType string) bool {
	before, after, ok := strings.Cut(mimeType, "/")
	if !ok {
		return false
	}

	// */* matches everything
	if mt.Type == "*" {
		return true
	}

	// Type must match
	if mt.Type != before {
		return false
	}

	// type/* matches any subtype
	if mt.Subtype == "*" {
		return true
	}

	// Exact match required
	return mt.Subtype == after
}
func (mt MediaType) String() string {
	return mt.Type + "/" + mt.Subtype
}

// ParseAccept parses an HTTP Accept header and returns a list of MediaType values
// sorted by quality factor (highest first).
//
// Behavior:
//   - Empty header returns "*/*" with q=1.0 (accept anything)
//   - Quality factors parsed from q parameter (defaults to 1.0)
//   - Entries with q=0 are excluded (per HTTP spec, q=0 means "not acceptable")
//   - Invalid entries are silently skipped
//   - Uses sort.SliceStable to preserve client preference order for equal q-values
//
// Examples:
//   - "text/html, application/json" -> [{text/html, q=1.0}, {application/json, q=1.0}]
//   - "text/html;q=0.8, application/json;q=0.9" -> [{application/json, q=0.9}, {text/html, q=0.8}]
//   - "*/*" -> [{*/*, q=1.0}]
func ParseAccept(acceptHeader string) []MediaType {
	if acceptHeader == "" {
		return []MediaType{{Type: "*", Subtype: "*", Q: 1.0}}
	}

	types := make([]MediaType, 0, 8)
	for part := range strings.SplitSeq(acceptHeader, ",") {
		mt, ok := parseMediaEntry(strings.TrimSpace(part))
		if !ok || mt.Q == 0 {
			continue
		}
		types = append(types, mt)
	}

	// Stable sort preserves client preference order for equal q-values
	sort.SliceStable(types, func(i, j int) bool {
		return types[i].Q > types[j].Q
	})

	return types
}

// parseMediaEntry parses a single Accept entry like "text/html;q=0.9"
// without allocating maps. Returns the parsed MediaType and whether
// parsing succeeded.
func parseMediaEntry(s string) (MediaType, bool) {
	// Separate media type from parameters at first semicolon.
	mediaType := s
	q := 1.0
	if before, after, ok := strings.Cut(s, ";"); ok {
		mediaType = strings.TrimSpace(before)
		q = parseQValue(after)
	}

	// Split type/subtype at slash.
	slash := strings.IndexByte(mediaType, '/')
	if slash <= 0 || slash >= len(mediaType)-1 {
		return MediaType{}, false
	}

	return MediaType{
		Type:    mediaType[:slash],
		Subtype: mediaType[slash+1:],
		Q:       q,
	}, true
}

// parseQValue extracts the q= quality factor from a semicolon-separated
// parameter string. Returns 1.0 if no q parameter is found.
func parseQValue(params string) float64 {
	for param := range strings.SplitSeq(params, ";") {
		p := strings.TrimSpace(param)
		if len(p) >= 3 && (p[0] == 'q' || p[0] == 'Q') && p[1] == '=' {
			if v, err := strconv.ParseFloat(p[2:], 64); err == nil {
				return max(0, min(1, v))
			}
		}
	}
	return 1.0
}
