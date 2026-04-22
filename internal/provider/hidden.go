package provider

import "strings"

// IsHiddenPath reports whether a file path contains a hidden segment
// (starting with "."). Hidden files include .gomddoc/, .git/, .env, etc.
// Exception: ".well-known" is explicitly allowed (IETF RFC 8615).
//
// This function is used by both the HTTP middleware (BlockHiddenPaths) and
// the MCP tools to enforce consistent path restrictions across all entry points.
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
