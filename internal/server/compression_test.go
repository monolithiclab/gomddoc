package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper to decompress gzip body from a recorder.
func decompressGzip(t *testing.T, body []byte) string {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Failed to decompress gzip: %v", err)
	}
	return string(out)
}

// largeBody returns a string larger than minCompressionSize.
func largeBody() string {
	return strings.Repeat("Hello, World! This is compressible content. ", 100)
}

func TestCompression_GzipWhenAccepted(t *testing.T) {
	body := largeBody()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("Content-Encoding = %q, want %q", ce, "gzip")
	}

	decompressed := decompressGzip(t, w.Body.Bytes())
	if decompressed != body {
		t.Errorf("Decompressed body length = %d, want %d", len(decompressed), len(body))
	}
}

func TestCompression_NoGzipWithoutAcceptEncoding(t *testing.T) {
	body := largeBody()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Accept-Encoding header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ce := w.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty", ce)
	}

	if w.Body.String() != body {
		t.Error("Body should not be compressed")
	}
}

func TestCompression_SkipSmallResponses(t *testing.T) {
	smallBody := "tiny"
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(smallBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ce := w.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty for small response", ce)
	}

	if w.Body.String() != smallBody {
		t.Errorf("Body = %q, want %q", w.Body.String(), smallBody)
	}
}

func TestCompression_VaryHeaderAlwaysSet(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
	}{
		{"with gzip", "gzip"},
		{"without gzip", "deflate"},
		{"no header", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("body"))
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if vary := w.Header().Get("Vary"); vary != "Accept-Encoding" {
				t.Errorf("Vary = %q, want %q", vary, "Accept-Encoding")
			}
		})
	}
}

func TestCompression_ContentLengthRemovedWhenCompressing(t *testing.T) {
	body := largeBody()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Length", "99999") // Explicitly set, should be removed
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if cl := w.Header().Get("Content-Length"); cl != "" {
		t.Errorf("Content-Length = %q, want empty when compressing", cl)
	}
}

func TestCompression_ContentEncodingSetWhenCompressing(t *testing.T) {
	body := largeBody()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("Content-Encoding = %q, want %q", ce, "gzip")
	}
}

func TestCompression_SkipAlreadyCompressedContentTypes(t *testing.T) {
	body := largeBody()
	tests := []struct {
		name        string
		contentType string
	}{
		{"png image", "image/png"},
		{"jpeg image", "image/jpeg"},
		{"webp image", "image/webp"},
		{"mp4 video", "video/mp4"},
		{"mp3 audio", "audio/mpeg"},
		{"zip archive", "application/zip"},
		{"gzip archive", "application/gzip"},
		{"wasm binary", "application/wasm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte(body))
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if ce := w.Header().Get("Content-Encoding"); ce == "gzip" {
				t.Errorf("Should not compress %s content", tt.contentType)
			}
		})
	}
}

func TestCompression_Flusher(t *testing.T) {
	body := largeBody()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should still produce valid gzip output after flush
	if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", ce)
	}

	decompressed := decompressGzip(t, w.Body.Bytes())
	if decompressed != body {
		t.Errorf("Decompressed body mismatch after flush")
	}
}

func TestCompression_PreservesStatusCode(t *testing.T) {
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		want     bool
	}{
		{"plain gzip", "gzip", true},
		{"gzip with deflate", "gzip, deflate", true},
		{"gzip with q-value", "gzip;q=0.8, deflate", true},
		{"only deflate", "deflate", false},
		{"empty", "", false},
		{"br only", "br", false},
		{"mixed with gzip last", "deflate, br, gzip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.encoding != "" {
				r.Header.Set("Accept-Encoding", tt.encoding)
			}
			if got := acceptsGzip(r); got != tt.want {
				t.Errorf("acceptsGzip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompressionWriter_Unwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := &compressionWriter{ResponseWriter: rec}
	if cw.Unwrap() != rec {
		t.Error("Unwrap should return underlying ResponseWriter")
	}
}

func TestCompressionWriter_WriteDecided_Compress(t *testing.T) {
	// Handler writes a large body in two chunks to exercise writeDecided with gzip active.
	firstChunk := strings.Repeat("A", minCompressionSize+1)
	secondChunk := strings.Repeat("B", 500)
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(firstChunk))
		// After the first write exceeds threshold, decided=true and compress=true.
		// This second write goes through writeDecided with gzip.
		_, _ = w.Write([]byte(secondChunk))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ce := w.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", ce)
	}

	decompressed := decompressGzip(t, w.Body.Bytes())
	want := firstChunk + secondChunk
	if decompressed != want {
		t.Errorf("decompressed body length = %d, want %d", len(decompressed), len(want))
	}
}

func TestCompressionWriter_WriteDecided_Passthrough(t *testing.T) {
	// Handler writes a large body with image/png content type in two chunks.
	// The first write triggers the decision (passthrough because image/png is skipped).
	// The second write goes through writeDecided without gzip.
	firstChunk := strings.Repeat("\x89", minCompressionSize+1)
	secondChunk := strings.Repeat("\x00", 500)
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(firstChunk))
		// After the first write exceeds threshold, decided=true and compress=false.
		// This second write goes through writeDecided raw (passthrough).
		_, _ = w.Write([]byte(secondChunk))
	}))

	req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ce := w.Header().Get("Content-Encoding"); ce == "gzip" {
		t.Error("image/png should not be gzip-compressed")
	}

	want := firstChunk + secondChunk
	if w.Body.String() != want {
		t.Errorf("body length = %d, want %d", w.Body.Len(), len(want))
	}
}

func TestCompression_HeadRequest(t *testing.T) {
	body := largeBody()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodHead, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if ce := w.Header().Get("Content-Encoding"); ce == "gzip" {
		t.Error("HEAD requests should not be compressed")
	}

	if vary := w.Header().Get("Vary"); vary != "Accept-Encoding" {
		t.Errorf("Vary = %q, want %q", vary, "Accept-Encoding")
	}
}

func TestShouldSkipContentType(t *testing.T) {
	tests := []struct {
		contentType string
		skip        bool
	}{
		{"text/html", false},
		{"text/css", false},
		{"application/json", false},
		{"application/javascript", false},
		{"image/png", true},
		{"image/jpeg", true},
		{"video/mp4", true},
		{"audio/mpeg", true},
		{"application/zip", true},
		{"application/gzip", true},
		{"application/wasm", true},
		{"image/svg+xml", false}, // SVG is text-based XML and compresses well
		{"text/html; charset=utf-8", false},
		{"IMAGE/PNG", true}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			if got := shouldSkipContentType(tt.contentType); got != tt.skip {
				t.Errorf("shouldSkipContentType(%q) = %v, want %v", tt.contentType, got, tt.skip)
			}
		})
	}
}
