package negotiate

import (
	"mime"
	"path/filepath"
	"strings"
)

func init() {
	// Register MIME types not present in all OS MIME databases.
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
}

// DetectMIME returns the MIME type for a file path.
// It uses mime.TypeByExtension and defaults to "application/octet-stream"
// if the extension is unknown.
func DetectMIME(path string) string {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

// NormalizeMimeType strips parameters from MIME types.
// Example: "text/html; charset=utf-8" -> "text/html"
//
// Fast-path: if the input contains no semicolon, it is already
// parameter-free and returned without calling mime.ParseMediaType
// (which allocates a map[string]string for parameters).
func NormalizeMimeType(mimeType string) string {
	if !strings.Contains(mimeType, ";") {
		return strings.TrimSpace(mimeType)
	}
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil || mediaType == "" {
		return strings.TrimSpace(mimeType)
	}
	return mediaType
}
