package negotiate

import (
	"mime"
	"testing"
)

func init() {
	// Register markdown MIME type for tests
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
}

func TestDetectMIME(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{"html file", "test.html", "text/html; charset=utf-8"},
		{"text file", "test.txt", "text/plain; charset=utf-8"},
		{"markdown file", "test.md", "text/markdown; charset=utf-8"},
		{"json file", "test.json", "application/json"},
		{"mjs module", "component.mjs", "text/javascript; charset=utf-8"},
		{"unknown extension", "test.unknown", "application/octet-stream"},
		{"no extension", "test", "application/octet-stream"},
		{"path with dots", "path/to.file/test.txt", "text/plain; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DetectMIME(tt.path); got != tt.want {
				t.Errorf("DetectMIME(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizeMimeType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mimeType string
		want     string
	}{
		{
			name:     "MIME type without parameters",
			mimeType: "text/html",
			want:     "text/html",
		},
		{
			name:     "MIME type with multiple parameters",
			mimeType: "application/json; charset=utf-8; boundary=something",
			want:     "application/json",
		},
		{
			name:     "Malformed MIME type",
			mimeType: "invalid",
			want:     "invalid",
		},
		{
			name:     "uppercase without params lowercased",
			mimeType: "TEXT/HTML",
			want:     "text/html",
		},
		{
			name:     "uppercase with params lowercased",
			mimeType: "TEXT/HTML; charset=utf-8",
			want:     "text/html",
		},
		{
			name:     "Empty string",
			mimeType: "",
			want:     "",
		},
		{
			name:     "Only whitespaces",
			mimeType: "   ",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeMimeType(tt.mimeType)
			if got != tt.want {
				t.Errorf("NormalizeMimeType(%q) = %q, want %q", tt.mimeType, got, tt.want)
			}
		})
	}
}
