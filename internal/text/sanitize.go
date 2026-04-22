package text

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode"
)

// SafeString is a string type that sanitizes control characters lazily
// when used as a slog.LogValuer. This prevents log injection attacks
// where user-controlled input (URL paths, env vars) could contain
// newlines or control characters to forge log entries.
//
// Sanitization is deferred until the log record is actually emitted,
// so no work is done if the log level is disabled.
type SafeString string

// LogValue implements slog.LogValuer, sanitizing control characters
// only when the log handler resolves the value for output.
func (s SafeString) LogValue() slog.Value {
	return slog.StringValue(Sanitize(string(s)))
}

// Safe creates a slog.Attr with a lazily-sanitized string value.
// Use this instead of slog.String() for any user-controlled input.
func Safe(key, val string) slog.Attr {
	return slog.Any(key, SafeString(val))
}

// Sanitize replaces control characters in s with their Go escape sequences.
// It preserves printable characters, spaces, tabs, and valid multibyte Unicode.
// Uses a single pass with lazy allocation — no heap allocation for clean strings.
func Sanitize(s string) string {
	// Find the first control character; return early if none found.
	firstControl := strings.IndexFunc(s, isUnsafeControl)
	if firstControl < 0 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s) + 8) // slight extra room for escape sequences
	b.WriteString(s[:firstControl])

	for _, r := range s[firstControl:] {
		if isUnsafeControl(r) {
			switch r {
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\x00':
				b.WriteString(`\x00`)
			default:
				switch {
				case r <= 0xFF:
					fmt.Fprintf(&b, `\x%02x`, r)
				case r <= 0xFFFF:
					fmt.Fprintf(&b, `\u%04x`, r)
				default:
					fmt.Fprintf(&b, `\U%08x`, r)
				}
			}
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// isUnsafeControl reports whether r is a control character that should be
// sanitized. Tabs are allowed as they are common in legitimate values.
func isUnsafeControl(r rune) bool {
	return r != '\t' && unicode.IsControl(r)
}
