package server

import (
	"log/slog"
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
// the provided etag using weak comparison per RFC 9110 §8.8.3.2.
// Returns true if the client's cached version is still valid (caller
// should return 304 Not Modified).
//
// Weak comparison: two ETags are equivalent if their opaque-tags match,
// regardless of whether either is tagged as weak. So W/"abc" matches "abc".
func checkETag(r *http.Request, etag string) bool {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}

	// Wildcard matches everything
	if strings.TrimSpace(ifNoneMatch) == "*" {
		return true
	}

	// Weak comparison: strip W/ prefix to compare opaque-tags
	etagOpaque := stripWeakPrefix(etag)

	// Check each comma-separated value
	for value := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate := stripWeakPrefix(strings.TrimSpace(value))
		if candidate == etagOpaque {
			return true
		}
	}

	return false
}

// stripWeakPrefix removes the W/ weak validator prefix from an ETag value.
// W/"abc" -> "abc", "abc" -> "abc".
func stripWeakPrefix(etag string) string {
	if len(etag) >= 2 && etag[0] == 'W' && etag[1] == '/' {
		return etag[2:]
	}
	return etag
}

// serveWithETag generates an ETag, sets cache headers, handles conditional
// requests, and writes the response body. Returns true if the response was
// served (either 200 or 304), false should not happen in practice.
func serveWithETag(w http.ResponseWriter, r *http.Request, content []byte, contentType, cacheControl string) {
	etag := generateETag(content)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", cacheControl)

	if checkETag(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(content) // #nosec G705 -- content served with correct Content-Type; callers control the content source
	if writeErr != nil {
		slog.Error("Cannot write response",
			slog.String("request_id", GetRequestID(r.Context())),
			slog.Any("error", writeErr))
	}
}
