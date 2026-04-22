package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/monolithiclab/gomddoc/internal/text"
)

type MiddlewareFunc func(http.Handler) http.Handler

// NewBasicAuthMiddleware returns a middleware that enforces HTTP Basic
// Authentication using a CredentialStore for credential validation. The
// realm is used in the WWW-Authenticate header.
func NewBasicAuthMiddleware(store *CredentialStore, realm string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if !store.Validate(user, pass) {
				slog.Debug("Basic auth failed", text.Safe("remote", r.RemoteAddr)) // #nosec G706 -- value sanitized via text.Safe (slog.LogValuer)
				w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewMethodFilterMiddleware returns middleware that only allows the specified HTTP methods.
// Responds with 405 Method Not Allowed and an Allow header for disallowed methods.
func NewMethodFilterMiddleware(allowedMethods ...string) MiddlewareFunc {
	allowed := make(map[string]bool, len(allowedMethods))
	for _, m := range allowedMethods {
		allowed[m] = true
	}
	allowHeader := strings.Join(allowedMethods, ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed[r.Method] {
				w.Header().Set("Allow", allowHeader)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds security headers to HTTP responses for defense-in-depth
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		w.Header().Set("X-Frame-Options", "DENY")

		// Content Security Policy - restricts resource loading to prevent XSS
		// Too restrictive, so disabled for now. Should be configurable or let an HTTP Reverse Proxy set those
		// w.Header().Set("Content-Security-Policy",
		// 	"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'")

		// Control referrer information sent to other sites
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Restrict access to browser features
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// HSTS - only enable if using HTTPS
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// BlockHiddenPaths blocks HTTP access to hidden files and directories
// Hidden files/directories are those starting with a dot (.) in Unix
// This protects .gomddoc/, .git/, .env, .htaccess, .ssh/, etc.
// Exception: .well-known/ is explicitly allowed (IETF RFC 8615 standard)
func BlockHiddenPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Split path into segments
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Check each segment for hidden files/directories
		for segment := range strings.SplitSeq(path, "/") {
			if segment == "" {
				continue
			}

			// Allow .well-known (IETF RFC 8615 - used for Let's Encrypt, security.txt, etc.)
			if segment == ".well-known" {
				continue
			}

			// Block if segment starts with a dot
			if strings.HasPrefix(segment, ".") {
				slog.Debug("Blocked hidden path access", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
					text.Safe("path", r.URL.Path),
					text.Safe("segment", segment),
					slog.String("remote", r.RemoteAddr))

				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				if _, err := w.Write([]byte("File not found")); err != nil {
					slog.Debug("Failed to write response", // #nosec G706 -- path sanitized via text.Safe (slog.LogValuer)
						text.Safe("path", r.URL.Path),
						slog.Any("error", err))
				}
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
