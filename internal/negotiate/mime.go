package negotiate

import (
	"mime"
	stdpath "path"
	"strings"
)

func init() {
	// Register MIME types not present in all OS MIME databases.
	//
	// Markdown lives here rather than beside MarkdownRenderer, which owns the
	// format, because ownership is not what decides this: an init() only runs
	// if the package is linked, and the consumers are DetectMIME's callers.
	// internal/resolve is one of them and does not import internal/renderer, so
	// the entire clean-URL feature depended on some *other* package pulling
	// renderer into the binary. Its tests had to re-register .md themselves to
	// pass — which is the tell.
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
	_ = mime.AddExtensionType(".markdown", "text/markdown; charset=utf-8")
}

// DetectMIME returns the MIME type for a file path.
// It uses mime.TypeByExtension and defaults to "application/octet-stream"
// if the extension is unknown.
func DetectMIME(path string) string {
	mimeType := mime.TypeByExtension(stdpath.Ext(path))
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
		return strings.ToLower(strings.TrimSpace(mimeType))
	}
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil || mediaType == "" {
		return strings.ToLower(strings.TrimSpace(mimeType))
	}
	return mediaType
}
