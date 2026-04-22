package server

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func BenchmarkGenerateETag(b *testing.B) {
	small := bytes.Repeat([]byte("hello"), 20)        // 100 bytes
	medium := bytes.Repeat([]byte("content "), 128)   // 1 KB
	large := bytes.Repeat([]byte("paragraph "), 1024) // 10 KB

	tests := []struct {
		name    string
		content []byte
	}{
		{"Small_100B", small},
		{"Medium_1KB", medium},
		{"Large_10KB", large},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tt.content)))
			for b.Loop() {
				generateETag(tt.content)
			}
		})
	}
}

func BenchmarkCheckETag(b *testing.B) {
	etag := `W/"a1b2c3d4e5f6"`

	tests := []struct {
		name        string
		ifNoneMatch string
	}{
		{"Match", `W/"a1b2c3d4e5f6"`},
		{"MultipleValues", `W/"aaa", W/"bbb", W/"a1b2c3d4e5f6", W/"ccc", W/"ddd"`},
		{"NoMatch", `W/"aaa", W/"bbb", W/"ccc", W/"ddd", W/"eee"`},
		{"Wildcard", "*"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("If-None-Match", tt.ifNoneMatch)
			for b.Loop() {
				checkETag(req, etag)
			}
		})
	}
}
