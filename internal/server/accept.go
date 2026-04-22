package server

import (
	"mime"
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
//   - "text/html, application/json" → [{text/html, q=1.0}, {application/json, q=1.0}]
//   - "text/html;q=0.8, application/json;q=0.9" → [{application/json, q=0.9}, {text/html, q=0.8}]
//   - "*/*" → [{*/*, q=1.0}]
func ParseAccept(acceptHeader string) []MediaType {
	if acceptHeader == "" {
		return []MediaType{{Type: "*", Subtype: "*", Q: 1.0}}
	}

	var types []MediaType
	for part := range strings.SplitSeq(acceptHeader, ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue // Skip invalid entries
		}

		// Parse quality factor
		q := 1.0
		if qStr, ok := params["q"]; ok {
			if parsed, err := strconv.ParseFloat(qStr, 64); err == nil {
				q = parsed
			}
		}

		// Split into type and subtype
		parts := strings.Split(mediaType, "/")
		if len(parts) != 2 {
			continue // Invalid MIME type format
		}

		// Per HTTP spec, q=0 means "not acceptable" — skip these entries
		if q == 0 {
			continue
		}

		types = append(types, MediaType{
			Type:    parts[0],
			Subtype: parts[1],
			Q:       q,
		})
	}

	// Stable sort preserves client preference order for equal q-values
	sort.SliceStable(types, func(i, j int) bool {
		return types[i].Q > types[j].Q
	})

	return types
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
	parts := strings.Split(mimeType, "/")
	if len(parts) != 2 {
		return false
	}

	// */* matches everything
	if mt.Type == "*" {
		return true
	}

	// Type must match
	if mt.Type != parts[0] {
		return false
	}

	// type/* matches any subtype
	if mt.Subtype == "*" {
		return true
	}

	// Exact match required
	return mt.Subtype == parts[1]
}
