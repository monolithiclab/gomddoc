package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// discardWriter is a ResponseWriter that throws the body away. httptest's
// recorder grows a bytes.Buffer per request, which costs more than the whole
// middleware and hides what this benchmark is meant to measure.
type discardWriter struct{ h http.Header }

func (d discardWriter) Header() http.Header         { return d.h }
func (d discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d discardWriter) WriteHeader(int)             {}

func BenchmarkCompression(b *testing.B) {
	// Prepare payloads once, outside the benchmark loop. They are []byte because
	// a string conversion inside the handler would allocate a copy per request
	// and swamp the numbers.
	smallHTML := []byte(strings.Repeat("<p>Hello, world!</p>\n", 25))  // ~500 bytes
	largeHTML := []byte(strings.Repeat("<p>Hello, world!</p>\n", 250)) // ~5 KB
	largePNG := []byte(strings.Repeat("\x89PNG\r\n\x1a\n IDAT", 500))  // ~5 KB binary-ish
	prefix := []byte("<!doctype html>\n")

	tests := []struct {
		name        string
		method      string
		contentType string
		payloads    [][]byte // written in order, one Write each
	}{
		{
			name:        "BelowThreshold_TextHTML",
			method:      http.MethodGet,
			contentType: "text/html; charset=utf-8",
			payloads:    [][]byte{smallHTML},
		},
		{
			name:        "AboveThreshold_TextHTML_Gzip",
			method:      http.MethodGet,
			contentType: "text/html; charset=utf-8",
			payloads:    [][]byte{largeHTML},
		},
		{
			// The buffered case: a short prefix settles nothing, then the body
			// arrives. It must not be copied into the pooled buffer.
			name:        "AboveThreshold_PrefixThenBody_Gzip",
			method:      http.MethodGet,
			contentType: "text/html; charset=utf-8",
			payloads:    [][]byte{prefix, largeHTML},
		},
		{
			name:        "AboveThreshold_ImagePNG_Skipped",
			method:      http.MethodGet,
			contentType: "image/png",
			payloads:    [][]byte{largePNG},
		},
		{
			name:        "HEAD_LargeTextHTML",
			method:      http.MethodHead,
			contentType: "text/html; charset=utf-8",
			payloads:    [][]byte{largeHTML},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				for _, p := range tt.payloads {
					_, _ = w.Write(p)
				}
			}))

			req := httptest.NewRequest(tt.method, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := discardWriter{h: make(http.Header, 4)}

			for b.Loop() {
				clear(w.h) // Compression appends to Vary on every request
				handler.ServeHTTP(w, req)
			}
		})
	}
}
