package server

import (
	"log/slog"
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
