package provider

import "github.com/monolithiclab/gomddoc/internal/config"

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
func NewProvider(dir, defaultIndex string, dirIndex bool, excludePatterns []string, gitCfg ...GitProviderConfig) (Provider, error) {
	if config.IsGitURL(dir) {
		var cfg GitProviderConfig
		if len(gitCfg) > 0 {
			cfg = gitCfg[0]
		}
		return NewGitProvider(dir, defaultIndex, dirIndex, excludePatterns, cfg)
	}
	return NewFilesystemProvider(dir, defaultIndex, dirIndex, excludePatterns)
}
