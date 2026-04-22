package provider

import (
	"io/fs"
	"path"
	"strings"
)

// IsHiddenPath reports whether a file path contains a hidden segment
// (starting with "."). Hidden files include .gomddoc/, .git/, .env, etc.
// Exception: ".well-known" is explicitly allowed (IETF RFC 8615).
//
// This is a security invariant — always enforced regardless of configuration.
func IsHiddenPath(filePath string) bool {
	clean := strings.TrimPrefix(filePath, "/")
	for segment := range strings.SplitSeq(clean, "/") {
		if segment == "" {
			continue
		}
		if segment == ".well-known" {
			continue
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

// IsRestrictedPath reports whether a file path is hidden or matches any
// user-configured exclude pattern. Combines IsHiddenPath and IsExcludedPath
// into a single check for use at all access control points.
func IsRestrictedPath(filePath string, excludePatterns []string) bool {
	return IsHiddenPath(filePath) || IsExcludedPath(filePath, excludePatterns)
}

// SkipWalkEntry checks whether a WalkDir entry should be skipped based on
// hidden path rules and exclude patterns. Returns fs.SkipDir for excluded
// directories, nil for excluded files, and a sentinel false when not excluded.
// Usage: if skip, err := provider.SkipWalkEntry(...); skip { return err }
func SkipWalkEntry(entryPath, name string, isDir bool, excludePatterns []string) (skip bool, err error) {
	if name == "." {
		return false, nil
	}
	if IsHiddenPath(name) || IsExcludedPath(entryPath, excludePatterns) {
		if isDir {
			return true, fs.SkipDir
		}
		return true, nil
	}
	return false, nil
}

// IsExcludedPath reports whether a file path matches any pattern in the
// exclude list. Patterns use path.Match syntax (*, ?, []).
//
// A pattern without "/" is matched against each path segment individually
// (e.g., "*.bak" matches "docs/notes.bak"). A pattern containing "/" is
// matched against the full cleaned path (e.g., "drafts/" matches
// "drafts/secret.md" but not "docs/drafts.md").
//
// A trailing "/" on a pattern means it only matches directory prefixes
// (any path that starts with that directory).
func IsExcludedPath(filePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	clean := strings.TrimPrefix(filePath, "/")
	if clean == "" {
		return false
	}

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		// Pattern with "/" — match against the full path.
		if strings.Contains(pattern, "/") {
			if before, ok := strings.CutSuffix(pattern, "/"); ok {
				// Trailing "/" means directory prefix match.
				dir := before
				if strings.HasPrefix(clean, pattern) || clean == dir {
					return true
				}
			} else if matched, _ := path.Match(pattern, clean); matched {
				return true
			}
			continue
		}

		// Pattern without "/" — match against each segment.
		for segment := range strings.SplitSeq(clean, "/") {
			if segment == "" {
				continue
			}
			if matched, _ := path.Match(pattern, segment); matched {
				return true
			}
		}
	}
	return false
}
