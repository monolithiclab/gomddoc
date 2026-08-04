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

// bufPool reuses byte buffers for compression buffering, avoiding
// per-request allocation of the compressionWriter.buf slice. Write commits
// before it appends, so a buffer never holds minCompressionSize bytes: one
// allocated at that size is never re-grown, and every buffer in the pool has
// exactly that capacity.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, minCompressionSize)
		return &b
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
		// Always add Vary so caches know the response depends on Accept-Encoding.
		// Use Add (not Set) to preserve any existing Vary values from other middleware.
		w.Header().Add("Vary", "Accept-Encoding")

		// Skip compression for HEAD requests (body is discarded anyway)
		// and for clients that don't accept gzip
		if r.Method == http.MethodHead || !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}

		bp := bufPool.Get().(*[]byte)
		cw := &compressionWriter{
			ResponseWriter: w,
			buf:            (*bp)[:0],
			bufPtr:         bp,
		}
		defer cw.Close()

		next.ServeHTTP(cw, r)
	})
}

// acceptsGzip checks whether the request Accept-Encoding header includes gzip
// with a non-zero quality value. Per RFC 9110, "gzip;q=0" explicitly disables gzip.
func acceptsGzip(r *http.Request) bool {
	for encoding := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		enc := strings.TrimSpace(encoding)
		params := ""
		if idx := strings.Index(enc, ";"); idx >= 0 {
			params = enc[idx+1:]
			enc = strings.TrimSpace(enc[:idx])
		}
		if strings.EqualFold(enc, "gzip") {
			return !isQualityZero(params)
		}
	}
	return false
}

// isQualityZero returns true if the parameters contain a q-value of zero.
// Handles multiple semicolon-separated parameters (e.g., "level=5;q=0").
// Per RFC 9110 §12.4.2, quality values have up to 3 decimal places.
func isQualityZero(params string) bool {
	for param := range strings.SplitSeq(params, ";") {
		p := strings.TrimSpace(param)
		if len(p) >= 3 && (p[0] == 'q' || p[0] == 'Q') && p[1] == '=' {
			v := p[2:]
			return v == "0" || v == "0." || v == "0.0" || v == "0.00" || v == "0.000"
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
	bufPtr     *[]byte // pool pointer for returning the buffer
	gzw        *gzip.Writer
	statusCode int
	decided    bool // whether the compress/passthrough decision has been committed
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

	// Still short of the threshold: keep buffering.
	if len(cw.buf)+len(b) < minCompressionSize {
		cw.buf = append(cw.buf, b...)
		return len(b), nil
	}

	// This write settles the question, so it goes straight to the encoder — only
	// what was buffered before it is copied. Handlers deliver a whole document in
	// one Write, so appending here would copy the entire body just to flush it on
	// the next line, and would grow the pooled buffer to match it.
	cw.commit()
	if len(cw.buf) > 0 {
		if err := cw.flushBuffer(); err != nil {
			return 0, err
		}
	}
	return cw.writeDecided(b)
}

// commit settles the compress/passthrough decision from the content type,
// acquires the gzip writer if compressing, and writes the status line.
// Everything written afterwards goes through writeDecided.
func (cw *compressionWriter) commit() {
	cw.decided = true
	if !shouldSkipContentType(cw.ResponseWriter.Header().Get("Content-Type")) {
		cw.initGzip()
	}
	cw.ResponseWriter.WriteHeader(cw.statusCode)
}

// flushBuffer writes the buffered data through the committed writer and empties
// the buffer by length — never by niling it, which would drop the pooled array.
func (cw *compressionWriter) flushBuffer() error {
	_, err := cw.writeDecided(cw.buf)
	cw.buf = cw.buf[:0]
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
	return cw.ResponseWriter.Write(b) // #nosec G705 -- handler content forwarded with its original Content-Type and nosniff header
}

// Close finalizes the response. If data was buffered but never reached the
// threshold, it is written uncompressed. If gzip was active, the gzip stream
// is closed and the writer returned to the pool. The buffer is always
// returned to the pool.
func (cw *compressionWriter) Close() {
	defer cw.returnBuf()

	// Never decided: the response stayed under the threshold, so it goes out
	// raw — no gzip writer was ever acquired, so flushBuffer passes it through.
	// statusCode is zero only when the handler wrote nothing at all, in which
	// case there is no response to commit.
	if !cw.decided {
		if cw.statusCode == 0 {
			return
		}
		cw.ResponseWriter.WriteHeader(cw.statusCode)
		_ = cw.flushBuffer()
		return
	}

	// Close gzip writer and return to pool
	if cw.gzw != nil {
		_ = cw.gzw.Close()
		gzipWriterPool.Put(cw.gzw)
		cw.gzw = nil
	}
}

// returnBuf hands the pooled buffer back. Only the pointer travels: cw.buf is a
// slice of the pooled array and never outgrows it (see bufPool), so the pool's
// own slice header stays untouched and keeps its capacity for the next request.
// Writing cw.buf back over it is what let a niled buf refill the pool with
// zero-capacity slices; should the buffer ever be reallocated after all, the
// oversized copy is simply dropped here instead of parked in the pool.
func (cw *compressionWriter) returnBuf() {
	if cw.bufPtr != nil {
		bufPool.Put(cw.bufPtr)
		cw.bufPtr = nil
		cw.buf = nil
	}
}

// Flush implements http.Flusher. If the underlying ResponseWriter supports
// flushing, we flush any pending gzip data and then flush the transport.
func (cw *compressionWriter) Flush() {
	// If we have not decided yet, force a decision so data gets written
	if !cw.decided && len(cw.buf) > 0 {
		cw.commit()
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
