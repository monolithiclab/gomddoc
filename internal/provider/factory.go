package provider

import "github.com/monolithiclab/gomddoc/internal/common"

// NewProvider creates the appropriate provider based on the directory/URL.
//
// For Git URLs (git://, git+ssh://, git+https://), it creates a GitProvider
// that clones the repository into memory and serves files from it.
//
// For filesystem paths, it creates a FilesystemProvider that serves files
// directly from the local filesystem.
//
// Parameters:
//   - dir: Either a filesystem path or a Git URL
//   - defaultIndex: Default index file name (e.g., "README.md")
//   - dirIndex: Enable directory listing generation (false = secure by default)
//
// Returns an error if the provider cannot be created. For Git URLs, URL
// parsing errors are returned immediately; clone errors occur on first access.
func NewProvider(dir, defaultIndex string, dirIndex bool, opts ...GitProviderOption) (Provider, error) {
	if common.IsGitURL(dir) {
		return NewGitProvider(dir, defaultIndex, dirIndex, opts...)
	}
	return NewFilesystemProvider(dir, defaultIndex, dirIndex)
}
