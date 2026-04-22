package server

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// CredentialStore holds parsed htpasswd credentials for Basic Auth lookup.
type CredentialStore struct {
	creds map[string][]byte // username -> bcrypt hash
}

// ParseHTPasswd parses an htpasswd-format reader into a CredentialStore.
// Only bcrypt hashes ($2y$, $2a$, $2b$) are supported.
// Lines starting with # are comments. Blank lines are skipped.
// Format: username:$2y$cost$hash
func ParseHTPasswd(r io.Reader) (*CredentialStore, error) {
	creds := make(map[string][]byte)
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		username, hash, ok := strings.Cut(line, ":")
		if !ok || username == "" || hash == "" {
			return nil, fmt.Errorf("line %d: invalid format (expected user:hash)", lineNum)
		}

		if !isBcryptHash(hash) {
			return nil, fmt.Errorf("line %d: unsupported hash format for user %q (only bcrypt $2y$/$2a$/$2b$ supported)", lineNum, username)
		}

		creds[username] = []byte(hash)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading htpasswd: %w", err)
	}

	return &CredentialStore{creds: creds}, nil
}

// Validate checks if the given username and password match stored credentials.
// Returns false for unknown users or wrong passwords.
// Uses bcrypt.CompareHashAndPassword which is constant-time by design.
func (s *CredentialStore) Validate(username, password string) bool {
	hash, ok := s.creds[username]
	if !ok {
		// Perform a dummy bcrypt comparison to prevent timing leaks
		// that reveal whether a username exists.
		_ = bcrypt.CompareHashAndPassword([]byte("$2y$10$000000000000000000000u"), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}

// Len returns the number of credentials in the store.
func (s *CredentialStore) Len() int {
	return len(s.creds)
}

// isBcryptHash checks if a hash string uses a bcrypt prefix.
func isBcryptHash(hash string) bool {
	return strings.HasPrefix(hash, "$2y$") ||
		strings.HasPrefix(hash, "$2a$") ||
		strings.HasPrefix(hash, "$2b$")
}
