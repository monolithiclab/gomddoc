package server

import (
	"errors"
	"testing"
)

func TestLazyBytes_CachesAndRetries(t *testing.T) {
	t.Parallel()

	calls := 0
	failFirst := true
	lb := newLazyBytes("thing", func() ([]byte, error) {
		calls++
		if failFirst {
			return nil, errors.New("boom")
		}
		return []byte("ok"), nil
	})

	// First call fails and is NOT cached (retry-on-error, unlike sync.Once).
	if _, err := lb.get(); err == nil {
		t.Fatal("expected error on first call")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}

	// Recover and succeed: result is cached.
	failFirst = false
	out, err := lb.get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "ok" {
		t.Fatalf("got %q, want %q", out, "ok")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (retry after failure)", calls)
	}

	// Subsequent calls return the cached value without re-invoking generate.
	if _, err := lb.get(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("generate re-invoked after success: calls = %d, want 2", calls)
	}
}
