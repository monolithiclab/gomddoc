package common

import (
	"mime"
	"testing"
)

func init() {
	// Register markdown MIME type for tests
	_ = mime.AddExtensionType(".md", "text/markdown; charset=utf-8")
}

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"html file", "test.html", "text/html; charset=utf-8"},
		{"text file", "test.txt", "text/plain; charset=utf-8"},
		{"markdown file", "test.md", "text/markdown; charset=utf-8"},
		{"json file", "test.json", "application/json"},
		{"unknown extension", "test.unknown", "application/octet-stream"},
		{"no extension", "test", "application/octet-stream"},
		{"path with dots", "path/to.file/test.txt", "text/plain; charset=utf-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectMIME(tt.path); got != tt.want {
				t.Errorf("DetectMIME(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
