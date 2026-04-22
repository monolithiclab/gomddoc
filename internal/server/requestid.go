package server

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// requestIDKey is the context key for storing request IDs.
type requestIDKey struct{}

// requestCounter is a global atomic counter for generating unique request IDs.
var requestCounter atomic.Uint64

// RequestID is middleware that assigns a unique identifier to each request.
// It first checks for an incoming X-Request-ID header (trusting upstream proxies).
// If not present, it generates one using a timestamp and atomic counter.
// The request ID is stored in the request context and set as a response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = fmt.Sprintf("%d-%d", time.Now().UnixMicro(), requestCounter.Add(1))
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
