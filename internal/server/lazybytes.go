package server

import (
	"log/slog"
	"net/http"
	"sync"
)

// lazyBytes caches the result of a byte-producing generator after the first
// successful call. Unlike sync.Once it retries when generation fails, so a
// transient error does not permanently break the endpoint. Shared by the
// sitemap and feed handlers, whose source (the metadata index) is immutable
// after construction.
type lazyBytes struct {
	what     string // label for error logging, e.g. "sitemap"
	generate func() ([]byte, error)

	mu     sync.Mutex
	cached []byte
}

// newLazyBytes returns a lazyBytes that produces its value via generate.
func newLazyBytes(what string, generate func() ([]byte, error)) *lazyBytes {
	return &lazyBytes{what: what, generate: generate}
}

// get returns the cached bytes, generating them on first successful call.
func (l *lazyBytes) get() ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cached != nil {
		return l.cached, nil
	}

	out, err := l.generate()
	if err != nil {
		slog.Error("Failed to generate "+l.what, slog.Any("error", err))
		return nil, err
	}
	l.cached = out
	return l.cached, nil
}

// serve writes the cached bytes as a conditional response. It exists so the
// rule — a body cached for the process's lifetime must carry an ETag, or every
// crawler hit re-downloads a document that cannot have changed — is a property
// of lazyBytes rather than of however many handlers happen to wrap one today.
//
// The failure is plain text, not the scope's themed ErrorPage: both callers
// serve XML to feed readers and crawlers, and an HTML error body on an XML
// endpoint is worse than a bare one.
func (l *lazyBytes) serve(w http.ResponseWriter, r *http.Request, contentType string) {
	data, err := l.get()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	serveWithETag(w, r, data, contentType, cacheDynamic)
}
