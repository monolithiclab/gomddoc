package server

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/monolithiclab/gomddoc/internal/testutil/fanout"
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

// TestLazyBytes_ConcurrentFirstGet hits a cold lazyBytes from many goroutines,
// which is the only state it is ever in when a crawler and a feed reader arrive
// together on a freshly started server.
//
// The generator sleeps so that a missing lock is a lost race rather than a coin
// flip. It costs 1ms once, not 50: get holds l.mu across generate, so exactly
// one goroutine ever reaches the sleep.
func TestLazyBytes_ConcurrentFirstGet(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	lb := newLazyBytes("thing", func() ([]byte, error) {
		calls.Add(1)
		time.Sleep(time.Millisecond)
		return []byte("generated once"), nil
	})

	const goroutines = 50
	results := make([][]byte, goroutines)
	fanout.Run(goroutines, func(i int) {
		out, err := lb.get()
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
			return
		}
		results[i] = out
	})

	if got := calls.Load(); got != 1 {
		t.Errorf("generate called %d times for %d concurrent cold callers, want 1", got, goroutines)
	}
	// Backing arrays, not bytes.Equal: the mutex publishes l.cached, so two
	// callers holding different arrays means one read the field before the write
	// that filled it — which an equality check on a deterministic value cannot see.
	for i := range goroutines {
		if len(results[i]) == 0 || &results[i][0] != &results[0][0] {
			t.Fatalf("goroutine %d got a different backing array than goroutine 0; the cached value is not published under the lock", i)
		}
	}
}
