package server

import (
	"net/http/httptest"
	"testing"
)

func TestGenerateETag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
	}{
		{name: "simple text", content: []byte("hello world")},
		{name: "empty content", content: []byte{}},
		{name: "binary content", content: []byte{0x89, 0x50, 0x4E, 0x47}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			etag := generateETag(tt.content)

			// Must be consistent
			etag2 := generateETag(tt.content)
			if etag != etag2 {
				t.Errorf("ETag not consistent: %q != %q", etag, etag2)
			}

			// Must be a weak validator with quoted hex string: W/"..."
			if len(etag) < 5 { // W/"x" minimum
				t.Errorf("ETag too short: %q", etag)
			}
			if etag[:2] != `W/` {
				t.Errorf("ETag should start with W/, got %q", etag)
			}
			if etag[2] != '"' || etag[len(etag)-1] != '"' {
				t.Errorf("ETag value should be quoted, got %q", etag)
			}
		})
	}
}

func TestGenerateETag_DifferentContent(t *testing.T) {
	t.Parallel()

	etag1 := generateETag([]byte("content A"))
	etag2 := generateETag([]byte("content B"))

	if etag1 == etag2 {
		t.Errorf("Different content should produce different ETags: %q == %q", etag1, etag2)
	}
}

func TestCheckETag(t *testing.T) {
	t.Parallel()

	etag := `W/"abc123"`

	tests := []struct {
		name        string
		ifNoneMatch string
		want        bool
	}{
		{
			name:        "exact match",
			ifNoneMatch: `W/"abc123"`,
			want:        true,
		},
		{
			name:        "no match",
			ifNoneMatch: `W/"different"`,
			want:        false,
		},
		{
			name:        "empty header",
			ifNoneMatch: "",
			want:        false,
		},
		{
			name:        "wildcard",
			ifNoneMatch: "*",
			want:        true,
		},
		{
			name:        "wildcard with spaces",
			ifNoneMatch: "  *  ",
			want:        true,
		},
		{
			name:        "multiple values with match",
			ifNoneMatch: `W/"other", W/"abc123", W/"another"`,
			want:        true,
		},
		{
			name:        "multiple values without match",
			ifNoneMatch: `W/"one", W/"two", W/"three"`,
			want:        false,
		},
		{
			name:        "match with surrounding spaces",
			ifNoneMatch: `  W/"abc123"  `,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.ifNoneMatch != "" {
				req.Header.Set("If-None-Match", tt.ifNoneMatch)
			}

			got := checkETag(req, etag)
			if got != tt.want {
				t.Errorf("checkETag() = %v, want %v", got, tt.want)
			}
		})
	}
}
