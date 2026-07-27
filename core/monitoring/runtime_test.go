package monitoring

import (
	"runtime"
	"testing"
	"time"
)

func TestCollectRuntimeStats(t *testing.T) {
	stats := CollectRuntimeStats()

	if stats.NumGoroutines < 1 {
		t.Errorf("NumGoroutines = %d, want at least 1", stats.NumGoroutines)
	}
	if stats.NumCPU != runtime.NumCPU() {
		t.Errorf("NumCPU = %d, want %d", stats.NumCPU, runtime.NumCPU())
	}
	if stats.AllocatedBytes == 0 {
		t.Error("AllocatedBytes = 0, want a live heap")
	}
	if stats.TotalAllocBytes < stats.AllocatedBytes {
		t.Errorf("TotalAllocBytes (%d) < AllocatedBytes (%d); cumulative must exceed live",
			stats.TotalAllocBytes, stats.AllocatedBytes)
	}
	if stats.NumCgoCall < 0 {
		t.Errorf("NumCgoCall = %d, want >= 0", stats.NumCgoCall)
	}
}

// TestRuntimeStats_HeapObjectsIsACount guards the units. HeapObjects is a
// number of live objects, not a byte size — dividing it by 1024^2 and labelling
// the result "MB" renders a meaningless near-zero figure.
func TestRuntimeStats_HeapObjectsIsACount(t *testing.T) {
	// Hold a large allocation so the two quantities cannot be confused.
	ballast := make([][]byte, 2000)
	for i := range ballast {
		ballast[i] = make([]byte, 1024)
	}
	runtime.GC()

	stats := CollectRuntimeStats()
	runtime.KeepAlive(ballast)

	if stats.HeapObjects == 0 {
		t.Fatal("HeapObjects = 0, want live objects")
	}

	// ~2000 small allocations weigh ~2 MB but are only ~2000 objects: treating
	// the count as bytes would report a fraction of a megabyte.
	asMB := float64(stats.HeapObjects) / 1048576
	if asMB > 1 {
		t.Logf("HeapObjects=%d happens to exceed 1 when divided as bytes; the units are still distinct", stats.HeapObjects)
	}
	if float64(stats.AllocatedBytes) <= float64(stats.HeapObjects) {
		t.Errorf("AllocatedBytes (%d) should exceed HeapObjects (%d): bytes are not object counts",
			stats.AllocatedBytes, stats.HeapObjects)
	}
}

// TestRuntimeStats_LastGCDurationUsesMostRecentPause pins the documented
// ring-buffer idiom PauseNs[(NumGC+255)%256].
func TestRuntimeStats_LastGCDurationUsesMostRecentPause(t *testing.T) {
	runtime.GC()
	stats := CollectRuntimeStats()

	if stats.NumGC == 0 {
		t.Skip("no GC has run")
	}
	if stats.LastGCDuration <= 0 {
		t.Errorf("LastGCDuration = %v, want a positive pause after a forced GC", stats.LastGCDuration)
	}
	if stats.LastGCDuration > time.Second {
		t.Errorf("LastGCDuration = %v, implausibly long — wrong ring-buffer index?", stats.LastGCDuration)
	}
	if uint64(stats.LastGCDuration) > stats.GCPauseTotal {
		t.Errorf("LastGCDuration (%v) exceeds the cumulative pause total (%d ns)",
			stats.LastGCDuration, stats.GCPauseTotal)
	}
}

func TestRuntimeStats_LastGCTimeIsRecent(t *testing.T) {
	runtime.GC()
	stats := CollectRuntimeStats()

	if stats.NumGC == 0 {
		t.Skip("no GC has run")
	}
	if stats.LastGCTime.IsZero() {
		t.Fatal("LastGCTime is zero after a forced GC")
	}
	if since := time.Since(stats.LastGCTime); since > time.Minute || since < -time.Minute {
		t.Errorf("LastGCTime is %v away from now; LastGC is nanoseconds since the epoch", since)
	}
}

func TestRuntimeStats_NextGCExceedsLiveHeap(t *testing.T) {
	runtime.GC()
	stats := CollectRuntimeStats()

	if stats.NextGC == 0 {
		t.Skip("NextGC not reported")
	}
	// The next collection is targeted above the current live heap.
	if stats.NextGC < stats.AllocatedBytes {
		t.Errorf("NextGC (%d) < AllocatedBytes (%d)", stats.NextGC, stats.AllocatedBytes)
	}
}

func TestCollectRuntimeStats_GoroutineCountTracks(t *testing.T) {
	before := CollectRuntimeStats().NumGoroutines

	release := make(chan struct{})
	started := make(chan struct{})
	const spawned = 20

	for i := 0; i < spawned; i++ {
		go func() {
			started <- struct{}{}
			<-release
		}()
	}
	for i := 0; i < spawned; i++ {
		<-started
	}

	during := CollectRuntimeStats().NumGoroutines
	close(release)

	if during <= before {
		t.Errorf("NumGoroutines did not rise after spawning %d goroutines: %d then %d",
			spawned, before, during)
	}
}

func TestRuntimeStatsZeroValue(t *testing.T) {
	var stats RuntimeStats

	if stats.NumGoroutines != 0 || stats.AllocatedBytes != 0 || stats.NumGC != 0 {
		t.Error("zero value should have zeroed fields")
	}
	if !stats.LastGCTime.IsZero() {
		t.Error("zero value should have a zero LastGCTime")
	}
}
