package analytics

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func testVisit(path string, class visitClass, at time.Time) visit {
	return visit{
		Path:       path,
		DeviceType: "desktop",
		Browser:    "chrome",
		OS:         "linux",
		Class:      class,
		At:         at,
	}
}

func TestAggregator_FoldsRepeatViewsIntoOneKey(t *testing.T) {
	now := time.Now()
	a := newAggregator(1000, 1000, now)

	for i := 0; i < 100; i++ {
		a.record(testVisit("/pricing", visitContinued, now))
	}

	if got := a.pendingLen(); got != 1 {
		t.Fatalf("pendingLen = %d, want 1 (all views share a key)", got)
	}

	drained := a.drain()
	for key, delta := range drained {
		if key.Path != "/pricing" {
			t.Fatalf("key.Path = %q, want /pricing", key.Path)
		}
		if delta.Views != 100 {
			t.Fatalf("Views = %d, want 100", delta.Views)
		}
	}
}

func TestAggregator_CountsSessionClassesSeparately(t *testing.T) {
	now := time.Now()
	a := newAggregator(1000, 1000, now)

	a.record(testVisit("/", visitNew, now))
	a.record(testVisit("/", visitNew, now))
	a.record(testVisit("/", visitReturning, now))
	a.record(testVisit("/", visitContinued, now))
	a.record(testVisit("/", visitContinued, now))

	drained := a.drain()
	if len(drained) != 1 {
		t.Fatalf("drained %d keys, want 1", len(drained))
	}
	for _, delta := range drained {
		if delta.Views != 5 {
			t.Errorf("Views = %d, want 5", delta.Views)
		}
		if delta.NewSessions != 2 {
			t.Errorf("NewSessions = %d, want 2", delta.NewSessions)
		}
		if delta.ReturningSessions != 1 {
			t.Errorf("ReturningSessions = %d, want 1", delta.ReturningSessions)
		}
	}
}

func TestAggregator_BoundsDistinctPathsPerDay(t *testing.T) {
	now := time.Now()
	const maxPaths = 100
	a := newAggregator(100000, maxPaths, now)

	// Every request hits a unique path — a junk-URL flood, or an unbounded
	// route like /order/{id}.
	const flood = 20000
	for i := 0; i < flood; i++ {
		a.record(testVisit(fmt.Sprintf("/junk-%d", i), visitNew, now))
	}

	drained := a.drain()
	// maxPaths real paths plus the single overflow bucket.
	if len(drained) > maxPaths+1 {
		t.Fatalf("%d distinct paths recorded, want <= %d", len(drained), maxPaths+1)
	}

	var overflowViews, total int
	for key, delta := range drained {
		total += delta.Views
		if key.Path == OverflowPath {
			overflowViews = delta.Views
		}
	}
	if total != flood {
		t.Fatalf("total views = %d, want %d (no view may be dropped)", total, flood)
	}
	if overflowViews != flood-maxPaths {
		t.Fatalf("overflow views = %d, want %d", overflowViews, flood-maxPaths)
	}
	t.Logf("%d junk URLs -> %d rows (%d absorbed by %s)", flood, len(drained), overflowViews, OverflowPath)
}

func TestAggregator_PathBudgetResetsOnDateRollover(t *testing.T) {
	day1 := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	a := newAggregator(100000, 2, day1)

	a.record(testVisit("/a", visitNew, day1))
	a.record(testVisit("/b", visitNew, day1))
	a.record(testVisit("/c", visitNew, day1)) // budget exhausted -> overflow

	a.record(testVisit("/c", visitNew, day2)) // fresh budget on the new day

	drained := a.drain()
	found := false
	for key := range drained {
		if key.Path == "/c" && key.Date == day2.Format("2006-01-02") {
			found = true
		}
	}
	if !found {
		t.Fatal("path /c was not recorded on day 2; per-day budget did not reset")
	}
}

func TestAggregator_RecentRingIsNewestFirstAndBounded(t *testing.T) {
	now := time.Now()
	a := newAggregator(10000, 10000, now)

	total := SessionRingSize + 20
	for i := 0; i < total; i++ {
		a.record(testVisit(fmt.Sprintf("/p%d", i), visitContinued, now))
	}

	recent := a.recentVisits(SessionRingSize)
	if len(recent) != SessionRingSize {
		t.Fatalf("recentVisits len = %d, want %d", len(recent), SessionRingSize)
	}
	for i, rv := range recent {
		want := fmt.Sprintf("/p%d", total-1-i)
		if rv.Path != want {
			t.Fatalf("recent[%d].Path = %q, want %q (ring must be newest-first)", i, rv.Path, want)
		}
	}
}

func TestAggregator_RecentRingHandlesPartialFill(t *testing.T) {
	now := time.Now()
	a := newAggregator(10000, 10000, now)

	a.record(testVisit("/first", visitNew, now))
	a.record(testVisit("/second", visitContinued, now))

	recent := a.recentVisits(SessionRingSize)
	if len(recent) != 2 {
		t.Fatalf("len = %d, want 2", len(recent))
	}
	if recent[0].Path != "/second" || recent[1].Path != "/first" {
		t.Fatalf("got %q,%q want /second,/first", recent[0].Path, recent[1].Path)
	}
}

func TestAggregator_ActivityCountsTrailingHour(t *testing.T) {
	base := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := newAggregator(10000, 10000, base)

	// One visit per minute for 90 minutes.
	for i := 0; i < 90; i++ {
		a.record(testVisit("/", visitContinued, base.Add(time.Duration(i)*time.Minute)))
	}

	count, _ := a.activity(base.Add(89 * time.Minute))
	if count != hourlyBuckets {
		t.Fatalf("trailing-hour count = %d, want %d (older minutes must age out)", count, hourlyBuckets)
	}
}

func TestAggregator_ActivityDecaysWithoutTraffic(t *testing.T) {
	base := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := newAggregator(10000, 10000, base)

	for i := 0; i < 10; i++ {
		a.record(testVisit("/", visitContinued, base))
	}

	if count, _ := a.activity(base); count != 10 {
		t.Fatalf("immediate count = %d, want 10", count)
	}
	if count, _ := a.activity(base.Add(30 * time.Minute)); count != 10 {
		t.Fatalf("count at +30m = %d, want 10 (still inside the hour)", count)
	}
	if count, _ := a.activity(base.Add(61 * time.Minute)); count != 0 {
		t.Fatalf("count at +61m = %d, want 0 (window fully expired)", count)
	}
}

func TestAggregator_ActivityScaleTracksPeak(t *testing.T) {
	base := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := newAggregator(1000000, 1000000, base)

	// Below the floor, the fixed denominator applies.
	if _, scale := a.activity(base); scale != MaxExpectedHourlyVisits {
		t.Fatalf("idle scale = %d, want %d", scale, MaxExpectedHourlyVisits)
	}

	const burst = MaxExpectedHourlyVisits * 40
	for i := 0; i < burst; i++ {
		a.record(testVisit("/", visitContinued, base))
	}

	count, scale := a.activity(base)
	if count != burst {
		t.Fatalf("count = %d, want %d", count, burst)
	}
	if scale != burst {
		t.Fatalf("scale = %d, want %d (denominator must follow the peak)", scale, burst)
	}

	// After the window drains, the peak is retained so the bar stays comparable.
	if _, scale := a.activity(base.Add(2 * time.Hour)); scale != burst {
		t.Fatalf("scale after decay = %d, want %d", scale, burst)
	}
}

func TestAggregator_DrainEmptiesPending(t *testing.T) {
	now := time.Now()
	a := newAggregator(1000, 1000, now)

	a.record(testVisit("/", visitNew, now))
	if got := len(a.drain()); got != 1 {
		t.Fatalf("first drain = %d keys, want 1", got)
	}
	if got := a.pendingLen(); got != 0 {
		t.Fatalf("pendingLen after drain = %d, want 0", got)
	}
	if got := a.drain(); got != nil {
		t.Fatalf("second drain = %v, want nil", got)
	}
}

func TestAggregator_RestoreFoldsCountsBack(t *testing.T) {
	now := time.Now()
	a := newAggregator(1000, 1000, now)

	a.record(testVisit("/", visitNew, now))
	drained := a.drain()

	// New activity arrives while the failed flush is being restored.
	a.record(testVisit("/", visitContinued, now))
	a.restore(drained)

	redrained := a.drain()
	if len(redrained) != 1 {
		t.Fatalf("keys = %d, want 1", len(redrained))
	}
	for _, delta := range redrained {
		if delta.Views != 2 {
			t.Fatalf("Views = %d, want 2 (restored + new)", delta.Views)
		}
		if delta.NewSessions != 1 {
			t.Fatalf("NewSessions = %d, want 1", delta.NewSessions)
		}
	}
}

func TestAggregator_RestoreRespectsCeiling(t *testing.T) {
	now := time.Now()
	a := newAggregator(10, 100000, now)

	// A large failed flush must not be able to grow pending without bound.
	failed := map[counterKey]*counterDelta{}
	for i := 0; i < 500; i++ {
		failed[counterKey{Path: fmt.Sprintf("/p%d", i), Date: "2026-07-25"}] = &counterDelta{Views: 1}
	}
	a.restore(failed)

	if got := a.pendingLen(); got > 10 {
		t.Fatalf("pendingLen after oversized restore = %d, want <= 10", got)
	}
}

func TestAggregator_SignalsFlushAtCeiling(t *testing.T) {
	now := time.Now()
	a := newAggregator(5, 100000, now)

	for i := 0; i < 4; i++ {
		if needsFlush := a.record(testVisit(fmt.Sprintf("/p%d", i), visitNew, now)); needsFlush {
			t.Fatalf("needsFlush at %d keys, want false", i+1)
		}
	}
	if needsFlush := a.record(testVisit("/p4", visitNew, now)); !needsFlush {
		t.Fatal("needsFlush at ceiling = false, want true")
	}
}

func TestAggregator_NeverDropsViewsUnderConcurrency(t *testing.T) {
	now := time.Now()
	a := newAggregator(50, 100000, now) // deliberately tiny ceiling

	const goroutines, perGoroutine = 32, 1000
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				a.record(testVisit(fmt.Sprintf("/p%d", i%200), visitContinued, now))
			}
		}(g)
	}
	wg.Wait()

	total := 0
	for _, delta := range a.drain() {
		total += delta.Views
	}

	want := goroutines * perGoroutine
	if total != want {
		t.Fatalf("total views = %d, want %d", total, want)
	}
}

func TestAggregator_ConcurrentReadsAndWrites(t *testing.T) {
	now := time.Now()
	a := newAggregator(1000, 1000, now)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				a.record(testVisit(fmt.Sprintf("/p%d", i%50), visitContinued, time.Now()))
			}
		}()
	}

	// Readers mirroring what the dashboard does.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				a.recentVisits(SessionRingSize)
				a.activity(time.Now())
				a.pendingLen()
			}
		}()
	}

	// Drainer mirroring the flush worker.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			a.drain()
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
