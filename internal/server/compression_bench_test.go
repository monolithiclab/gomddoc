package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkCompression(b *testing.B) {
	// Prepare payloads once, outside the benchmark loop.
	smallHTML := strings.Repeat("<p>Hello, world!</p>\n", 25)  // ~500 bytes
	largeHTML := strings.Repeat("<p>Hello, world!</p>\n", 250) // ~5 KB
	largePNG := strings.Repeat("\x89PNG\r\n\x1a\n IDAT", 500)  // ~5 KB binary-ish

	tests := []struct {
		name        string
		method      string
		contentType string
		accept      string
		payload     string
	}{
		{
			name:        "BelowThreshold_TextHTML",
			method:      http.MethodGet,
			contentType: "text/html; charset=utf-8",
			accept:      "gzip",
			payload:     smallHTML,
		},
		{
			name:        "AboveThreshold_TextHTML_Gzip",
			method:      http.MethodGet,
			contentType: "text/html; charset=utf-8",
			accept:      "gzip",
			payload:     largeHTML,
		},
		{
			name:        "AboveThreshold_ImagePNG_Skipped",
			method:      http.MethodGet,
			contentType: "image/png",
			accept:      "gzip",
			payload:     largePNG,
		},
		{
			name:        "HEAD_LargeTextHTML",
			method:      http.MethodHead,
			contentType: "text/html; charset=utf-8",
			accept:      "gzip",
			payload:     largeHTML,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte(tt.payload))
			}))

			for b.Loop() {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(tt.method, "/", nil)
				req.Header.Set("Accept-Encoding", tt.accept)
				handler.ServeHTTP(rec, req)
			}
		})
	}
}
