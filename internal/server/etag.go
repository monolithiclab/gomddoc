package server

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
)

// generateETag creates a weak ETag from content using FNV-64a hash.
// Returns a quoted hex string, e.g. W/"a1b2c3d4e5f6".
// Uses weak validator (W/) since content may be served with different
// transfer encodings (e.g., gzip compression).
func generateETag(content []byte) string {
	h := fnv.New64a()
	h.Write(content) // #nosec G104 -- fnv hash.Write never returns an error
	return fmt.Sprintf(`W/"%x"`, h.Sum64())
}

// checkETag checks whether the request's If-None-Match header matches
// the provided etag. Returns true if the client's cached version is
// still valid (caller should return 304 Not Modified).
func checkETag(r *http.Request, etag string) bool {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}

	// Wildcard matches everything
	if strings.TrimSpace(ifNoneMatch) == "*" {
		return true
	}

	// Check each comma-separated value
	for value := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate := strings.TrimSpace(value)
		if candidate == etag {
			return true
		}
	}

	return false
}
