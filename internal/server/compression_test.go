package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
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

// assertBody checks whether the response was gzipped and that the body the client
// ends up with is the one the handler wrote.
func assertBody(t *testing.T, rec *httptest.ResponseRecorder, wantGzip bool, want string) {
	t.Helper()
	ce := rec.Header().Get("Content-Encoding")
	if gz := ce == "gzip"; gz != wantGzip {
		t.Fatalf("gzipped = %v (Content-Encoding %q), want %v", gz, ce, wantGzip)
	}
	got := rec.Body.String()
	if wantGzip {
		got = decompressGzip(t, rec.Body.Bytes())
	}
	if got != want {
		t.Errorf("body = %.40q… (%d bytes), want %.40q… (%d bytes)", got, len(got), want, len(want))
	}
}

func TestCompression_GzipWhenAccepted(t *testing.T) {
	t.Parallel()
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

	assertBody(t, w, true, body)
}

func TestCompression_NoGzipWithoutAcceptEncoding(t *testing.T) {
	t.Parallel()
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

	assertBody(t, w, false, body)
}

func TestCompression_SkipSmallResponses(t *testing.T) {
	t.Parallel()
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

	assertBody(t, w, false, smallBody)
}

func TestCompression_VaryHeaderAlwaysSet(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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

			// Vary header should contain Accept-Encoding (may also contain other values)
			varyValues := w.Header().Values("Vary")
			found := slices.Contains(varyValues, "Accept-Encoding")
			if !found {
				t.Errorf("Vary header values %v should contain %q", varyValues, "Accept-Encoding")
			}
		})
	}
}

func TestCompression_VaryHeaderPreservesExisting(t *testing.T) {
	t.Parallel()
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("body"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	varyValues := w.Header().Values("Vary")
	hasAccept := false
	hasAcceptEncoding := false
	for _, v := range varyValues {
		if v == "Accept" {
			hasAccept = true
		}
		if v == "Accept-Encoding" {
			hasAcceptEncoding = true
		}
	}
	if !hasAccept {
		t.Errorf("Vary header should preserve existing Accept value, got %v", varyValues)
	}
	if !hasAcceptEncoding {
		t.Errorf("Vary header should include Accept-Encoding, got %v", varyValues)
	}
}

func TestCompression_ContentLengthRemovedWhenCompressing(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
	assertBody(t, w, true, body)
}

func TestCompression_PreservesStatusCode(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	tests := []struct {
		name     string
		encoding string
		want     bool
	}{
		{"plain gzip", "gzip", true},
		{"gzip with deflate", "gzip, deflate", true},
		{"gzip with q-value", "gzip;q=0.8, deflate", true},
		{"gzip with q=1", "gzip;q=1", true},
		{"gzip q=0 rejected", "gzip;q=0", false},
		{"gzip q=0.0 rejected", "gzip;q=0.0", false},
		{"gzip q=0.00 rejected", "gzip;q=0.00", false},
		{"gzip q=0.000 rejected", "gzip;q=0.000", false},
		{"gzip q=0. rejected", "gzip;q=0.", false},
		{"gzip q=0 with extra params", "gzip;level=5;q=0", false},
		{"only deflate", "deflate", false},
		{"empty", "", false},
		{"br only", "br", false},
		{"mixed with gzip last", "deflate, br, gzip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
	rec := httptest.NewRecorder()
	cw := &compressionWriter{ResponseWriter: rec}
	if cw.Unwrap() != rec {
		t.Error("Unwrap should return underlying ResponseWriter")
	}
}

func TestCompressionWriter_WriteDecided_Compress(t *testing.T) {
	t.Parallel()
	// Handler writes a large body in two chunks to exercise writeDecided with gzip active.
	firstChunk := strings.Repeat("A", minCompressionSize+1)
	secondChunk := strings.Repeat("B", 500)
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(firstChunk))
		// The first write already exceeds the threshold, so it commits without
		// being buffered; this one goes through the decided path with gzip.
		_, _ = w.Write([]byte(secondChunk))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assertBody(t, w, true, firstChunk+secondChunk)
}

func TestCompressionWriter_WriteDecided_Passthrough(t *testing.T) {
	t.Parallel()
	// Handler writes a large body with image/png content type in two chunks.
	// The first write triggers the decision (passthrough because image/png is skipped).
	// The second write goes through writeDecided without gzip.
	firstChunk := strings.Repeat("\x89", minCompressionSize+1)
	secondChunk := strings.Repeat("\x00", 500)
	handler := Compression(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(firstChunk))
		// The first write already exceeds the threshold, so it commits without
		// being buffered; this one goes through the decided path raw.
		_, _ = w.Write([]byte(secondChunk))
	}))

	req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assertBody(t, w, false, firstChunk+secondChunk)
}

func TestCompression_HeadRequest(t *testing.T) {
	t.Parallel()
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

	varyValues := w.Header().Values("Vary")
	found := slices.Contains(varyValues, "Accept-Encoding")
	if !found {
		t.Errorf("Vary header values %v should contain %q", varyValues, "Accept-Encoding")
	}
}

// newPooledWriter returns a compressionWriter over a buffer exactly as bufPool
// supplies one. bufPtr is deliberately left nil so the writer never touches the
// global pool: returnBuf then leaves cw.buf alone and the test can inspect the
// very slice returnBuf would have handed back.
func newPooledWriter(rec *httptest.ResponseRecorder, contentType string) *compressionWriter {
	cw := &compressionWriter{ResponseWriter: rec, buf: make([]byte, 0, minCompressionSize)}
	cw.Header().Set("Content-Type", contentType)
	return cw
}

// TestCompressionWriter_BufferHandback covers the state of the buffer when it
// goes back to the pool, which the response body cannot show: it is identical
// whether the buffer survives or not. Both defects it guards against show up as
// a changed capacity — niling the buffer drops it to zero, and appending a body
// instead of committing first reallocates it away from the pooled array.
func TestCompressionWriter_BufferHandback(t *testing.T) {
	t.Parallel()
	small := strings.Repeat("s", 64)
	half := strings.Repeat("h", minCompressionSize/2+1)
	large := largeBody() // one write, already past the threshold and the buffer

	tests := []struct {
		name        string
		contentType string
		writes      []string
		wantBody    string
		wantGzip    bool
	}{
		{
			name:        "below threshold, flushed raw by Close",
			contentType: "text/plain",
			writes:      []string{small},
			wantBody:    small,
		},
		{
			name:        "threshold crossed by a second write, gzipped",
			contentType: "text/plain",
			writes:      []string{half, half},
			wantBody:    half + half,
			wantGzip:    true,
		},
		{
			name:        "threshold crossed by a second write, passthrough",
			contentType: "image/png",
			writes:      []string{half, half},
			wantBody:    half + half,
		},
		{
			name:        "single write past the threshold, gzipped",
			contentType: "text/html",
			writes:      []string{large},
			wantBody:    large,
			wantGzip:    true,
		},
		{
			name:        "single write past the threshold, passthrough",
			contentType: "image/png",
			writes:      []string{large},
			wantBody:    large,
		},
		{
			// Close returns early without committing a status line.
			name:        "nothing written at all",
			contentType: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			cw := newPooledWriter(rec, tt.contentType)
			for _, w := range tt.writes {
				n, err := cw.Write([]byte(w))
				if err != nil || n != len(w) {
					t.Fatalf("Write = (%d, %v), want (%d, nil)", n, err, len(w))
				}
			}
			cw.Close()

			if got := len(cw.buf); got != 0 {
				t.Errorf("len(buf) after Close = %d, want 0", got)
			}
			if got := cap(cw.buf); got != minCompressionSize {
				t.Errorf("cap(buf) after Close = %d, want %d", got, minCompressionSize)
			}
			assertBody(t, rec, tt.wantGzip, tt.wantBody)
		})
	}
}

func TestShouldSkipContentType(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			if got := shouldSkipContentType(tt.contentType); got != tt.skip {
				t.Errorf("shouldSkipContentType(%q) = %v, want %v", tt.contentType, got, tt.skip)
			}
		})
	}
}
