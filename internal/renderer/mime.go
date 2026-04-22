package renderer

import "mime"

// NormalizeMimeType strips parameters from MIME types.
// Example: "text/html; charset=utf-8" → "text/html"
//
// This normalization is essential for registry lookups since Go's
// mime.TypeByExtension() returns full MIME types with parameters
// (e.g., "text/html; charset=utf-8") but renderers register
// normalized types without parameters (e.g., "text/html").
func NormalizeMimeType(mimeType string) string {
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		// Malformed MIME type - return as-is
		return mimeType
	}
	return mediaType
}
