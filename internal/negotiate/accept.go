package negotiate

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// MediaType represents a parsed MIME type from an Accept header.
type MediaType struct {
	Type    string  // Main type (e.g., "text")
	Subtype string  // Subtype (e.g., "html")
	Q       float64 // Quality factor (0.0-1.0), defaults to 1.0
}

// Specificity ranks a media range per RFC 9110 §12.5.1, "media ranges can be
// overridden by more specific media ranges": exact=3, type/*=2, */*=1.
//
// This is the one implementation of that ladder. internal/renderer's
// outputMatchScore hand-rolled a second copy on the same scale, and ParseAccept
// briefly grew a third on a 0/1/2 scale of its own; a (MediaType).Matches
// method was deleted once before for the same reason.
func (mt MediaType) Specificity() int {
	switch {
	case mt.Type == "*":
		return 1
	case mt.Subtype == "*":
		return 2
	default:
		return 3
	}
}

// ParseAccept parses an HTTP Accept header and returns a list of MediaType
// values sorted by quality factor (highest first), then by specificity per
// RFC 9110 §12.5.1.
//
// Behavior:
//   - Empty header returns "*/*" with q=1.0 (accept anything)
//   - Quality factors parsed from q parameter (defaults to 1.0)
//   - Entries with q=0 are excluded (per HTTP spec, q=0 means "not acceptable")
//   - Invalid entries are silently skipped
//   - At equal q, "type/subtype" outranks "type/*" outranks "*/*"
//   - Uses slices.SortStableFunc, so client order breaks any remaining tie
//
// Examples:
//   - "text/html, application/json" -> [{text/html, q=1.0}, {application/json, q=1.0}]
//   - "text/html;q=0.8, application/json;q=0.9" -> [{application/json, q=0.9}, {text/html, q=0.8}]
//   - "*/*, text/markdown" -> [{text/markdown, q=1.0}, {*/*, q=1.0}]
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

	// Clients routinely send an explicit type alongside "*/*" at the same q —
	// "*/*, text/markdown" is the shape an LLM client asking for markdown
	// produces — and sorting by q alone left "*/*" first, so the registry
	// resolved at the wildcard and served HTML. Only the q tie needs
	// Specificity; a lower q on a more specific range still loses, which is
	// what "q=0.5" on that range means.
	//
	// Stable sort preserves client preference order for equal q and specificity.
	slices.SortStableFunc(types, func(a, b MediaType) int {
		if c := cmp.Compare(b.Q, a.Q); c != 0 {
			return c
		}
		return cmp.Compare(b.Specificity(), a.Specificity())
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
