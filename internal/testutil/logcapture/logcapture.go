// Package logcapture points the process-wide slog default at an in-memory
// handler for the duration of one test, which is how a test asserts on a
// degradation path: a skip that logs and a skip that does not are
// indistinguishable to the caller, so the log record is part of the contract.
package logcapture

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Log holds the records captured while it is installed.
type Log struct {
	mu      sync.Mutex
	records []slog.Record
}

// Install redirects slog.Default to a Log for the rest of the test and restores
// the previous default on cleanup.
//
// The default logger is process-wide, so a test calling this must not call
// t.Parallel(): a sibling running concurrently would have its output captured
// here and its own capture would swallow the records this one is waiting for.
func Install(t *testing.T, level slog.Leveler) *Log {
	t.Helper()
	l := &Log{}
	prev := slog.Default()
	slog.SetDefault(slog.New(&handler{log: l, level: level}))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return l
}

// Has reports whether some captured record has exactly this message and carries
// every one of attrs.
//
// Message and attributes are matched on the *same* record on purpose. Scanning
// rendered output for the message and the attribute separately passes when two
// unrelated records happen to supply one each — and the records that matter here
// are emitted in a loop, one per language, so that is the likely case rather
// than the contrived one.
func (l *Log) Has(msg string, attrs ...slog.Attr) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, rec := range l.records {
		if rec.Message == msg && hasAttrs(rec, attrs) {
			return true
		}
	}
	return false
}

// String renders every captured record, for use in failure messages.
func (l *Log) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var b strings.Builder
	for _, rec := range l.records {
		fmt.Fprintf(&b, "%s %s", rec.Level, rec.Message)
		for attr := range rec.Attrs {
			fmt.Fprintf(&b, " %s=%s", attr.Key, attr.Value)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func hasAttrs(rec slog.Record, want []slog.Attr) bool {
	found := 0
	for attr := range rec.Attrs {
		if slices.ContainsFunc(want, attr.Equal) {
			found++
		}
	}
	return found == len(want)
}

// handler collects records instead of formatting them.
//
// WithGroup is a no-op: grouped keys would have to be qualified for Has to find
// them, and nothing in this repo logs with groups. Add the qualification here
// rather than at the assertion if that changes.
type handler struct {
	log   *Log
	level slog.Leveler
	attrs []slog.Attr
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *handler) Handle(_ context.Context, rec slog.Record) error {
	// Clone before adding: Record's attrs may share a backing array with the
	// caller's, and slog reuses the Record after Handle returns.
	stored := rec.Clone()
	stored.AddAttrs(h.attrs...)
	h.log.mu.Lock()
	defer h.log.mu.Unlock()
	h.log.records = append(h.log.records, stored)
	return nil
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{log: h.log, level: h.level, attrs: slices.Concat(h.attrs, attrs)}
}

func (h *handler) WithGroup(string) slog.Handler { return h }
