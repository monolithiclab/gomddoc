package server

import (
	"log/slog"
	"net/http"
	"strings"
)

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
				slog.Debug("Blocked hidden path access",
					slog.String("path", r.URL.Path),
					slog.String("segment", segment),
					slog.String("remote", r.RemoteAddr))

				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				if _, err := w.Write([]byte("File not found")); err != nil {
					slog.Debug("Failed to write response",
						slog.String("path", r.URL.Path),
						slog.Any("error", err))
				}
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
