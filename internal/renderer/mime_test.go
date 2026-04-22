package renderer

import "testing"

func TestNormalizeMimeType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		want     string
	}{
		{
			name:     "MIME type with charset",
			mimeType: "text/html; charset=utf-8",
			want:     "text/html",
		},
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
			name:     "Empty string",
			mimeType: "",
			want:     "",
		},
		{
			name:     "Image MIME type",
			mimeType: "image/png",
			want:     "image/png",
		},
		{
			name:     "CSS with charset",
			mimeType: "text/css; charset=utf-8",
			want:     "text/css",
		},
		{
			name:     "JavaScript with charset",
			mimeType: "text/javascript; charset=utf-8",
			want:     "text/javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMimeType(tt.mimeType)
			if got != tt.want {
				t.Errorf("NormalizeMimeType(%q) = %q, want %q", tt.mimeType, got, tt.want)
			}
		})
	}
}
