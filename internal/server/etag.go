package server

import (
	"net/http"
	"strconv"
	"strings"
)

// generateETag creates a weak ETag from content using inline FNV-64a hash.
// Returns a quoted hex string, e.g. W/"a1b2c3d4e5f6".
// Uses weak validator (W/) since content may be served with different
// transfer encodings (e.g., gzip compression).
//
// Inlines FNV-64a to avoid hash.Hash interface allocation. Uses a
// stack-allocated buffer with strconv.AppendUint instead of fmt.Sprintf,
// reducing heap allocations to one (the returned string).
func generateETag(content []byte) string {
	// FNV-64a: offset basis and prime per spec.
	var hash uint64 = 14695981039346656037
	for _, b := range content {
		hash ^= uint64(b)
		hash *= 1099511628211
	}
	// W/"<hex>" — max 20 bytes (W/" + 16 hex digits + closing quote).
	var buf [20]byte
	buf[0], buf[1], buf[2] = 'W', '/', '"'
	hex := strconv.AppendUint(buf[:3], hash, 16)
	return string(append(hex, '"'))
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
