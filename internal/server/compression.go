package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// minCompressionSize is the minimum response size in bytes before compression kicks in.
const minCompressionSize = 1024

// gzipWriterPool reuses gzip writers to reduce allocations.
var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// compressedContentTypes lists MIME type prefixes that are already compressed
// and should not be compressed again.
var skipCompressionTypes = []string{
	"image/",
	"video/",
	"audio/",
	"application/zip",
	"application/gzip",
	"application/x-gzip",
	"application/x-compress",
	"application/x-bzip2",
	"application/x-xz",
	"application/x-7z-compressed",
	"application/x-rar-compressed",
	"application/zstd",
	"application/wasm",
}

// Compression is HTTP middleware that applies gzip compression to responses
// when the client supports it. Responses smaller than minCompressionSize bytes
// are sent uncompressed. Already-compressed content types (images, video, etc.)
// are never re-compressed.
func Compression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always set Vary so caches know the response depends on Accept-Encoding
		w.Header().Set("Vary", "Accept-Encoding")

		// Skip compression for HEAD requests (body is discarded anyway)
		// and for clients that don't accept gzip
		if r.Method == http.MethodHead || !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}

		cw := &compressionWriter{
			ResponseWriter: w,
		}
		defer cw.Close()

		next.ServeHTTP(cw, r)
	})
}

// acceptsGzip checks whether the request Accept-Encoding header includes gzip.
func acceptsGzip(r *http.Request) bool {
	for encoding := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		// Trim whitespace and ignore q-values for simplicity
		enc := strings.TrimSpace(encoding)
		if idx := strings.Index(enc, ";"); idx >= 0 {
			enc = strings.TrimSpace(enc[:idx])
		}
		if strings.EqualFold(enc, "gzip") {
			return true
		}
	}
	return false
}

// shouldSkipContentType returns true if the content type is already compressed
// and should not be gzip-encoded again.
func shouldSkipContentType(ct string) bool {
	ct = strings.ToLower(ct)
	// Strip parameters (charset, boundary, etc.)
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	// SVG is text-based XML and compresses well despite being under image/
	if strings.HasPrefix(ct, "image/svg") {
		return false
	}
	for _, skip := range skipCompressionTypes {
		if strings.HasPrefix(ct, skip) {
			return true
		}
	}
	return false
}

// compressionWriter buffers writes until it can decide whether to compress.
// If the buffered data exceeds minCompressionSize and the content type is
// compressible, it lazily initializes a gzip writer and flushes the buffer
// through it. Otherwise, the buffer is written directly.
type compressionWriter struct {
	http.ResponseWriter
	buf        []byte
	gzw        *gzip.Writer
	statusCode int
	decided    bool // whether we have committed to compress or not
	compress   bool // the decision: true = gzip, false = passthrough
}

// WriteHeader captures the status code but defers writing it until we know
// whether we will compress.
func (cw *compressionWriter) WriteHeader(statusCode int) {
	cw.statusCode = statusCode
}

// Write buffers data until we can decide whether to compress.
func (cw *compressionWriter) Write(b []byte) (int, error) {
	if cw.statusCode == 0 {
		cw.statusCode = http.StatusOK
	}

	// If we already decided, write directly
	if cw.decided {
		return cw.writeDecided(b)
	}

	// Buffer data
	cw.buf = append(cw.buf, b...)

	// If we have enough data, make the decision
	if len(cw.buf) >= minCompressionSize {
		cw.decide()
		return len(b), cw.flushBuffer()
	}

	return len(b), nil
}

// decide determines whether compression should be used based on content type.
func (cw *compressionWriter) decide() {
	cw.decided = true
	ct := cw.ResponseWriter.Header().Get("Content-Type")
	cw.compress = !shouldSkipContentType(ct)
}

// flushBuffer writes the buffered data either compressed or raw.
func (cw *compressionWriter) flushBuffer() error {
	if cw.compress {
		cw.initGzip()
	}
	cw.ResponseWriter.WriteHeader(cw.statusCode)
	if cw.gzw != nil {
		_, err := cw.gzw.Write(cw.buf)
		cw.buf = nil
		return err
	}
	_, err := cw.ResponseWriter.Write(cw.buf) // #nosec G705 -- buffered content forwarded with original Content-Type and nosniff header
	cw.buf = nil
	return err
}

// initGzip sets up compression headers and acquires a gzip writer from the pool.
func (cw *compressionWriter) initGzip() {
	cw.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	cw.ResponseWriter.Header().Del("Content-Length")

	gzw := gzipWriterPool.Get().(*gzip.Writer)
	gzw.Reset(cw.ResponseWriter)
	cw.gzw = gzw
}

// writeDecided writes data after the compress/passthrough decision has been made.
func (cw *compressionWriter) writeDecided(b []byte) (int, error) {
	if cw.gzw != nil {
		return cw.gzw.Write(b)
	}
	return cw.ResponseWriter.Write(b)
}

// Close finalizes the response. If data was buffered but never reached the
// threshold, it is written uncompressed. If gzip was active, the gzip stream
// is closed and the writer returned to the pool.
func (cw *compressionWriter) Close() {
	// If we never decided (small response), flush raw
	if !cw.decided && len(cw.buf) > 0 {
		if cw.statusCode == 0 {
			cw.statusCode = http.StatusOK
		}
		cw.ResponseWriter.WriteHeader(cw.statusCode)
		_, _ = cw.ResponseWriter.Write(cw.buf)
		cw.buf = nil
		return
	}

	// If we decided but nothing was written yet (empty body after decision), write header
	if !cw.decided && cw.statusCode != 0 {
		cw.ResponseWriter.WriteHeader(cw.statusCode)
		return
	}

	// Close gzip writer and return to pool
	if cw.gzw != nil {
		_ = cw.gzw.Close()
		gzipWriterPool.Put(cw.gzw)
		cw.gzw = nil
	}
}

// Flush implements http.Flusher. If the underlying ResponseWriter supports
// flushing, we flush any pending gzip data and then flush the transport.
func (cw *compressionWriter) Flush() {
	// If we have not decided yet, force a decision so data gets written
	if !cw.decided && len(cw.buf) > 0 {
		cw.decide()
		_ = cw.flushBuffer()
	}

	if cw.gzw != nil {
		_ = cw.gzw.Flush()
	}

	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for middleware that needs
// to inspect it (e.g., http.ResponseController).
func (cw *compressionWriter) Unwrap() http.ResponseWriter {
	return cw.ResponseWriter
}
