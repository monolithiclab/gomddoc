package server

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// testHash generates a bcrypt hash for testing.
func testHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}
	return string(hash)
}

func TestParseHTPasswd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantUsers int
		wantErr   bool
	}{
		{
			name:      "single user",
			input:     "admin:" + testHash(t, "secret"),
			wantUsers: 1,
		},
		{
			name:      "multiple users",
			input:     "admin:" + testHash(t, "secret") + "\nuser:" + testHash(t, "pass123"),
			wantUsers: 2,
		},
		{
			name:      "comments and blank lines",
			input:     "# this is a comment\n\nadmin:" + testHash(t, "secret") + "\n\n# another comment\n",
			wantUsers: 1,
		},
		{
			name:      "empty file",
			input:     "",
			wantUsers: 0,
		},
		{
			name:      "only comments",
			input:     "# comment\n# another",
			wantUsers: 0,
		},
		{
			name:    "missing colon",
			input:   "adminnoseparator",
			wantErr: true,
		},
		{
			name:    "empty username",
			input:   ":" + testHash(t, "secret"),
			wantErr: true,
		},
		{
			name:    "empty hash",
			input:   "admin:",
			wantErr: true,
		},
		{
			name:    "unsupported apr1 hash",
			input:   "admin:$apr1$xyz$abcdefghijklmnop",
			wantErr: true,
		},
		{
			name:    "unsupported SHA hash",
			input:   "admin:{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=",
			wantErr: true,
		},
		{
			name:    "plaintext password",
			input:   "admin:plaintext",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, err := ParseHTPasswd(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if store.Len() != tt.wantUsers {
				t.Errorf("user count = %d, want %d", store.Len(), tt.wantUsers)
			}
		})
	}
}

func TestCredentialStore_Validate(t *testing.T) {
	t.Parallel()

	input := "admin:" + testHash(t, "secret") + "\nviewer:" + testHash(t, "readonly")
	store, err := ParseHTPasswd(strings.NewReader(input))
	if err != nil {
		t.Fatalf("failed to parse htpasswd: %v", err)
	}

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{"valid admin", "admin", "secret", true},
		{"valid viewer", "viewer", "readonly", true},
		{"wrong password", "admin", "wrong", false},
		{"wrong username", "nobody", "secret", false},
		{"empty username", "", "secret", false},
		{"empty password", "admin", "", false},
		{"swapped credentials", "viewer", "secret", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := store.Validate(tt.username, tt.password); got != tt.want {
				t.Errorf("Validate(%q, %q) = %v, want %v", tt.username, tt.password, got, tt.want)
			}
		})
	}
}

func TestIsBcryptHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hash string
		want bool
	}{
		{"$2y$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01", true},
		{"$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01", true},
		{"$2b$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ01", true},
		{"$apr1$xyz$abcdefghijklmnop", false},
		{"{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=", false},
		{"plaintext", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.hash[:min(len(tt.hash), 10)], func(t *testing.T) {
			t.Parallel()
			if got := isBcryptHash(tt.hash); got != tt.want {
				t.Errorf("isBcryptHash(%q) = %v, want %v", tt.hash, got, tt.want)
			}
		})
	}
}
