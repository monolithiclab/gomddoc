package server

import (
	"testing"
)

func TestParseAccept(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantCount int
		wantFirst MediaType
		wantOrder []string // Full MIME types in expected order
	}{
		{
			name:      "empty header defaults to */*",
			header:    "",
			wantCount: 1,
			wantFirst: MediaType{Type: "*", Subtype: "*", Q: 1.0, Full: "*/*"},
			wantOrder: []string{"*/*"},
		},
		{
			name:      "single type",
			header:    "text/html",
			wantCount: 1,
			wantFirst: MediaType{Type: "text", Subtype: "html", Q: 1.0, Full: "text/html"},
			wantOrder: []string{"text/html"},
		},
		{
			name:      "multiple types preserve order",
			header:    "text/html, application/json",
			wantCount: 2,
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "quality factors sorted descending",
			header:    "text/html;q=0.8, application/json;q=0.9",
			wantCount: 2,
			wantFirst: MediaType{Type: "application", Subtype: "json", Q: 0.9, Full: "application/json"},
			wantOrder: []string{"application/json", "text/html"},
		},
		{
			name:      "quality factors with equal values preserve order",
			header:    "text/html;q=0.8, application/json;q=0.8, text/plain;q=0.8",
			wantCount: 3,
			wantOrder: []string{"text/html", "application/json", "text/plain"},
		},
		{
			name:      "wildcard */*",
			header:    "*/*",
			wantCount: 1,
			wantFirst: MediaType{Type: "*", Subtype: "*", Q: 1.0, Full: "*/*"},
			wantOrder: []string{"*/*"},
		},
		{
			name:      "type wildcard",
			header:    "text/*",
			wantCount: 1,
			wantFirst: MediaType{Type: "text", Subtype: "*", Q: 1.0, Full: "text/*"},
			wantOrder: []string{"text/*"},
		},
		{
			name:      "complex accept header",
			header:    "text/html, application/xhtml+xml, application/xml;q=0.9, image/webp, */*;q=0.8",
			wantCount: 5,
			wantOrder: []string{"text/html", "application/xhtml+xml", "image/webp", "application/xml", "*/*"},
		},
		{
			name:      "quality factor zero",
			header:    "text/html;q=0, application/json",
			wantCount: 2,
			wantOrder: []string{"application/json", "text/html"},
		},
		{
			name:      "quality factor with decimal places",
			header:    "text/html;q=0.95, application/json;q=0.85",
			wantCount: 2,
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "whitespace handling",
			header:    "  text/html  ,  application/json  ",
			wantCount: 2,
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "invalid entries skipped",
			header:    "text/html, invalid, application/json",
			wantCount: 2,
			wantOrder: []string{"text/html", "application/json"},
		},
		{
			name:      "other parameters ignored",
			header:    "text/html; charset=utf-8; q=0.9",
			wantCount: 1,
			wantFirst: MediaType{Type: "text", Subtype: "html", Q: 0.9, Full: "text/html"},
		},
		{
			name:      "browser-like accept header",
			header:    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			wantCount: 4,
			wantOrder: []string{"text/html", "application/xhtml+xml", "application/xml", "*/*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAccept(tt.header)

			if len(result) != tt.wantCount {
				t.Errorf("ParseAccept() returned %d types, want %d", len(result), tt.wantCount)
			}

			if tt.wantFirst.Full != "" {
				if len(result) == 0 {
					t.Fatal("ParseAccept() returned empty result")
				}
				first := result[0]
				if first.Type != tt.wantFirst.Type || first.Subtype != tt.wantFirst.Subtype ||
					first.Q != tt.wantFirst.Q || first.Full != tt.wantFirst.Full {
					t.Errorf("ParseAccept() first = %+v, want %+v", first, tt.wantFirst)
				}
			}

			if len(tt.wantOrder) > 0 {
				if len(result) != len(tt.wantOrder) {
					t.Fatalf("ParseAccept() returned %d types, want %d for order check", len(result), len(tt.wantOrder))
				}
				for i, want := range tt.wantOrder {
					if result[i].Full != want {
						t.Errorf("ParseAccept() result[%d].Full = %q, want %q", i, result[i].Full, want)
					}
				}
			}
		})
	}
}

func TestMediaType_Matches(t *testing.T) {
	tests := []struct {
		name      string
		mediaType MediaType
		mimeType  string
		want      bool
	}{
		{
			name:      "exact match",
			mediaType: MediaType{Type: "text", Subtype: "html", Full: "text/html"},
			mimeType:  "text/html",
			want:      true,
		},
		{
			name:      "exact mismatch - different subtype",
			mediaType: MediaType{Type: "text", Subtype: "html", Full: "text/html"},
			mimeType:  "text/plain",
			want:      false,
		},
		{
			name:      "exact mismatch - different type",
			mediaType: MediaType{Type: "text", Subtype: "html", Full: "text/html"},
			mimeType:  "application/json",
			want:      false,
		},
		{
			name:      "wildcard */* matches everything",
			mediaType: MediaType{Type: "*", Subtype: "*", Full: "*/*"},
			mimeType:  "text/html",
			want:      true,
		},
		{
			name:      "wildcard */* matches application/json",
			mediaType: MediaType{Type: "*", Subtype: "*", Full: "*/*"},
			mimeType:  "application/json",
			want:      true,
		},
		{
			name:      "wildcard */* matches image/png",
			mediaType: MediaType{Type: "*", Subtype: "*", Full: "*/*"},
			mimeType:  "image/png",
			want:      true,
		},
		{
			name:      "type wildcard text/* matches text/html",
			mediaType: MediaType{Type: "text", Subtype: "*", Full: "text/*"},
			mimeType:  "text/html",
			want:      true,
		},
		{
			name:      "type wildcard text/* matches text/plain",
			mediaType: MediaType{Type: "text", Subtype: "*", Full: "text/*"},
			mimeType:  "text/plain",
			want:      true,
		},
		{
			name:      "type wildcard text/* matches text/markdown",
			mediaType: MediaType{Type: "text", Subtype: "*", Full: "text/*"},
			mimeType:  "text/markdown",
			want:      true,
		},
		{
			name:      "type wildcard text/* does not match application/json",
			mediaType: MediaType{Type: "text", Subtype: "*", Full: "text/*"},
			mimeType:  "application/json",
			want:      false,
		},
		{
			name:      "type wildcard image/* matches image/png",
			mediaType: MediaType{Type: "image", Subtype: "*", Full: "image/*"},
			mimeType:  "image/png",
			want:      true,
		},
		{
			name:      "type wildcard image/* does not match text/html",
			mediaType: MediaType{Type: "image", Subtype: "*", Full: "image/*"},
			mimeType:  "text/html",
			want:      false,
		},
		{
			name:      "invalid mime type - no slash",
			mediaType: MediaType{Type: "text", Subtype: "html", Full: "text/html"},
			mimeType:  "texthtml",
			want:      false,
		},
		{
			name:      "invalid mime type - too many slashes",
			mediaType: MediaType{Type: "text", Subtype: "html", Full: "text/html"},
			mimeType:  "text/html/extra",
			want:      false,
		},
		{
			name:      "empty mime type",
			mediaType: MediaType{Type: "text", Subtype: "html", Full: "text/html"},
			mimeType:  "",
			want:      false,
		},
		{
			name:      "application/xhtml+xml exact match",
			mediaType: MediaType{Type: "application", Subtype: "xhtml+xml", Full: "application/xhtml+xml"},
			mimeType:  "application/xhtml+xml",
			want:      true,
		},
		{
			name:      "application/xhtml+xml mismatch",
			mediaType: MediaType{Type: "application", Subtype: "xhtml+xml", Full: "application/xhtml+xml"},
			mimeType:  "application/xml",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mediaType.Matches(tt.mimeType)
			if got != tt.want {
				t.Errorf("MediaType.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAccept_StableSort(t *testing.T) {
	// Test that stable sort preserves original order for equal q-values
	header := "a/a;q=0.5, b/b;q=0.5, c/c;q=0.5, d/d;q=0.5"
	result := ParseAccept(header)

	expected := []string{"a/a", "b/b", "c/c", "d/d"}
	for i, want := range expected {
		if result[i].Full != want {
			t.Errorf("StableSort failed: result[%d].Full = %q, want %q", i, result[i].Full, want)
		}
	}
}

func TestParseAccept_MixedQValues(t *testing.T) {
	header := "text/html;q=0.9, application/json;q=1.0, text/plain;q=0.8, image/png"
	result := ParseAccept(header)

	expectedOrder := []string{
		"application/json", // q=1.0
		"image/png",        // q=1.0 (default)
		"text/html",        // q=0.9
		"text/plain",       // q=0.8
	}

	if len(result) != len(expectedOrder) {
		t.Fatalf("ParseAccept() returned %d types, want %d", len(result), len(expectedOrder))
	}

	for i, want := range expectedOrder {
		if result[i].Full != want {
			t.Errorf("ParseAccept() result[%d].Full = %q, want %q", i, result[i].Full, want)
		}
	}
}

func TestMediaType_Matches_Comprehensive(t *testing.T) {
	// Test comprehensive wildcard matching scenarios
	tests := []struct {
		acceptHeader string
		mimeType     string
		shouldMatch  bool
	}{
		{"text/html", "text/html", true},
		{"text/html", "text/plain", false},
		{"*/*", "text/html", true},
		{"*/*", "application/json", true},
		{"*/*", "image/png", true},
		{"text/*", "text/html", true},
		{"text/*", "text/plain", true},
		{"text/*", "application/json", false},
		{"application/*", "application/json", true},
		{"application/*", "text/html", false},
		{"text/html, application/json", "text/html", true},
		{"text/html, application/json", "application/json", true},
		{"text/html, application/json", "image/png", false},
	}

	for _, tt := range tests {
		t.Run(tt.acceptHeader+" vs "+tt.mimeType, func(t *testing.T) {
			types := ParseAccept(tt.acceptHeader)
			matched := false
			for _, mt := range types {
				if mt.Matches(tt.mimeType) {
					matched = true
					break
				}
			}
			if matched != tt.shouldMatch {
				t.Errorf("Accept %q matching %q = %v, want %v", tt.acceptHeader, tt.mimeType, matched, tt.shouldMatch)
			}
		})
	}
}
