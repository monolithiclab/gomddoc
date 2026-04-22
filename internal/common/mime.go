package common

import (
	"mime"
	"path/filepath"
)

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
