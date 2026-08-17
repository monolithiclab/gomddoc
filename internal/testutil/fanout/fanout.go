// Package fanout releases N goroutines at once against a shared object, which
// is what a test needs to reach a lazy cache in the only state it is ever used
// in: cold and concurrent.
package fanout

import "sync"

// Run calls fn(0)..fn(n-1) in n goroutines released together and waits for all
// of them.
//
// The release barrier is the point. A bare WaitGroup lets the first goroutine
// finish before the last is scheduled, so a cold cache is warm by the time the
// contention was supposed to happen and the test passes against an unsynchronized
// implementation. Every caller here goes through the barrier; the hand-rolled
// copies that predate this helper did not.
//
// fn runs off the test goroutine, so it may call t.Errorf but not t.Fatalf.
// Assertions needing a cross-goroutine comparison — pointer identity, a shared
// backing array — write into a caller-owned slice indexed by i and run after Run
// returns.
func Run(n int, fn func(i int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			fn(i)
		})
	}
	close(start)
	wg.Wait()
}
