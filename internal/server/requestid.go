package server

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// requestIDKey is the context key for storing request IDs.
type requestIDKey struct{}

// requestCounter is a global atomic counter for generating unique request IDs.
var requestCounter atomic.Uint64

// maxRequestIDLen is the maximum allowed length for an incoming X-Request-ID header.
const maxRequestIDLen = 128

// RequestID is middleware that assigns a unique identifier to each request.
// It checks for an incoming X-Request-ID header and validates it: the value must
// be at most 128 characters and contain only alphanumeric characters, dashes, and
// underscores. Invalid or missing values are replaced with a generated ID.
// The request ID is stored in the request context and set as a response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !isValidRequestID(id) {
			id = strconv.FormatInt(time.Now().UnixMicro(), 10) + "-" + strconv.FormatUint(requestCounter.Add(1), 10)
		}

		// Set response header
		w.Header().Set("X-Request-ID", id)

		// Store in context and continue
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from a context.
// Returns an empty string if no request ID is present.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// isValidRequestID checks that an ID is non-empty, within length limits,
// and contains only safe characters (alphanumeric, dash, underscore).
func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}
