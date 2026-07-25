package analytics

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// vkey turns a readable name into the uint64 key the tracker uses.
func vkey(name string) uint64 { return sessionHash(name, "") }

func TestVisitorTracker_ClassifiesFirstVisitAsNew(t *testing.T) {
	now := time.Now()
	v := newVisitorTracker(4, 100, 30*time.Minute, now)

	if got := v.classify(vkey("visitor-a"), now); got != visitNew {
		t.Fatalf("first visit = %v, want visitNew", got)
	}
}

func TestVisitorTracker_ClassifiesSameSessionAsContinued(t *testing.T) {
	now := time.Now()
	v := newVisitorTracker(4, 100, 30*time.Minute, now)

	v.classify(vkey("visitor-a"), now)

	for i := 1; i <= 5; i++ {
		at := now.Add(time.Duration(i) * time.Minute)
		if got := v.classify(vkey("visitor-a"), at); got != visitContinued {
			t.Fatalf("visit at +%dm = %v, want visitContinued", i, got)
		}
	}
}

func TestVisitorTracker_ClassifiesLapsedSessionAsReturning(t *testing.T) {
	now := time.Now()
	window := 30 * time.Minute
	v := newVisitorTracker(4, 100, window, now)

	v.classify(vkey("visitor-a"), now)

	// Past the session window but still inside the tracker's memory
	// (generations * window = 2h).
	later := now.Add(window + time.Minute)
	if got := v.classify(vkey("visitor-a"), later); got != visitReturning {
		t.Fatalf("visit after session window = %v, want visitReturning", got)
	}
}

func TestVisitorTracker_ForgetsBeyondMemoryHorizon(t *testing.T) {
	now := time.Now()
	window := 30 * time.Minute
	generations := 4
	v := newVisitorTracker(generations, 100, window, now)

	v.classify(vkey("visitor-a"), now)

	// Each classify call rotates at most once, so step forward one window at a
	// time until the entry's generation has been evicted.
	at := now
	for i := 0; i < generations; i++ {
		at = at.Add(window)
		v.classify(vkey("someone-else"), at)
	}

	if got := v.classify(vkey("visitor-a"), at); got != visitNew {
		t.Fatalf("visit beyond memory horizon = %v, want visitNew (forgotten)", got)
	}
}

func TestVisitorTracker_ActiveVisitorSurvivesRotation(t *testing.T) {
	now := time.Now()
	window := 30 * time.Minute
	v := newVisitorTracker(3, 100, window, now)

	at := now
	v.classify(vkey("regular"), at)

	// A visitor active in every window should never be forgotten, even though
	// generations rotate underneath them.
	for i := 0; i < 20; i++ {
		at = at.Add(window - time.Minute)
		if got := v.classify(vkey("regular"), at); got != visitContinued {
			t.Fatalf("iteration %d: got %v, want visitContinued", i, got)
		}
	}
}

func TestVisitorTracker_MemoryIsBounded(t *testing.T) {
	now := time.Now()
	generations, perGen := 4, 500
	v := newVisitorTracker(generations, perGen, 30*time.Minute, now)

	// Simulate a flood of forged visitor keys — the exact shape of an attempt to
	// exhaust memory through the visitor map.
	const flood = 200000
	for i := 0; i < flood; i++ {
		v.classify(vkey(fmt.Sprintf("forged-%d", i)), now)
	}

	capacity := generations * perGen
	if got := v.size(); got > capacity {
		t.Fatalf("after %d distinct keys size = %d, want <= %d", flood, got, capacity)
	}
	if v.capacity() != capacity {
		t.Fatalf("capacity() = %d, want %d", v.capacity(), capacity)
	}
	t.Logf("%d forged keys -> %d entries retained (cap %d)", flood, v.size(), capacity)
}

func TestVisitorTracker_SizeNeverExceedsCapacityDuringGrowth(t *testing.T) {
	now := time.Now()
	v := newVisitorTracker(2, 50, 30*time.Minute, now)

	for i := 0; i < 5000; i++ {
		v.classify(vkey(fmt.Sprintf("k-%d", i)), now)
		if got := v.size(); got > v.capacity() {
			t.Fatalf("size %d exceeded capacity %d at iteration %d", got, v.capacity(), i)
		}
	}
}

func TestVisitorTracker_ConcurrentSameVisitorCountsOneNewSession(t *testing.T) {
	now := time.Now()
	v := newVisitorTracker(4, 10000, 30*time.Minute, now)

	const goroutines = 500
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[visitClass]int{}

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			class := v.classify(vkey("one-visitor"), time.Now())
			mu.Lock()
			counts[class]++
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()

	if counts[visitNew] != 1 {
		t.Fatalf("visitNew = %d, want exactly 1 (session start double counted)", counts[visitNew])
	}
	if counts[visitContinued] != goroutines-1 {
		t.Fatalf("visitContinued = %d, want %d", counts[visitContinued], goroutines-1)
	}
}

func TestVisitorTracker_ConcurrentDistinctVisitors(t *testing.T) {
	now := time.Now()
	v := newVisitorTracker(4, 100000, 30*time.Minute, now)

	const goroutines, perGoroutine = 32, 500
	var wg sync.WaitGroup
	var mu sync.Mutex
	newCount := 0

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			local := 0
			for i := 0; i < perGoroutine; i++ {
				if v.classify(vkey(fmt.Sprintf("g%d-v%d", g, i)), time.Now()) == visitNew {
					local++
				}
			}
			mu.Lock()
			newCount += local
			mu.Unlock()
		}(g)
	}
	wg.Wait()

	want := goroutines * perGoroutine
	if newCount != want {
		t.Fatalf("visitNew = %d, want %d (every key is distinct)", newCount, want)
	}
}

func TestVisitorTracker_DegenerateConfigIsSafe(t *testing.T) {
	now := time.Now()
	v := newVisitorTracker(0, 0, time.Minute, now)

	if v.capacity() < 1 {
		t.Fatalf("capacity = %d, want >= 1", v.capacity())
	}
	if got := v.classify(vkey("a"), now); got != visitNew {
		t.Fatalf("classify = %v, want visitNew", got)
	}
}
