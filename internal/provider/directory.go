package provider

import (
	"io/fs"
	"path"

	"github.com/monolithiclab/gomddoc/internal/negotiate"
)

// handleDirectory implements the shared directory handling algorithm:
// try index file → check dirIndex flag → generate listing.
// The tryIndex and listEntries callbacks abstract the backend-specific file access.
// dirPath is the directory relative to the content root ("." for the root);
// the index file's path under it is what ReadFile reports as the file served.
func handleDirectory(
	requestPath string,
	dirPath string,
	defaultIndex string,
	dirIndex bool,
	excludePatterns []string,
	tryIndex func() ([]byte, error),
	listEntries func() ([]fs.DirEntry, error),
) ([]byte, string, string, error) {
	// Try default index file first (always)
	if content, err := tryIndex(); err == nil {
		mimeType := negotiate.DetectMIME(defaultIndex)
		return content, mimeType, path.Join(dirPath, defaultIndex), nil
	}

	// Directory listing disabled (secure by default)
	if !dirIndex {
		return nil, "", "", &PathError{Op: "list", Path: requestPath, Err: ErrDirListingDisabled}
	}

	// Generate directory listing
	entries, err := listEntries()
	if err != nil {
		return nil, "", "", &PathError{Op: "list", Path: requestPath, Err: err}
	}

	content := GenerateMarkdownListing(requestPath, entries, excludePatterns)
	return content, "text/markdown; charset=utf-8", "", nil
}
