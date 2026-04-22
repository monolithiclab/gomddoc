package renderer

import "testing"

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
			name:     "Empty string",
			mimeType: "",
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
