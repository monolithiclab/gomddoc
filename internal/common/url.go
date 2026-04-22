package common

import "strings"

// IsGitURL checks if a string is a Git URL.
// Returns true for URLs starting with git://, git+ssh://, or git+https://.
func IsGitURL(s string) bool {
	return strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "git+ssh://") ||
		strings.HasPrefix(s, "git+https://")
}
