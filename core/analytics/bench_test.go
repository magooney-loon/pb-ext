package analytics_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// benchRequest builds a reusable request; Track never mutates it.
func benchRequest(path string) *http.Request {
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("User-Agent", uaChrome)
	return r
}

// seedCounters bulk-inserts rows straight into _analytics so aggregate
// benchmarks can reach table sizes that would take too long to accumulate
// through the collector.
func seedCounters(tb testing.TB, app core.App, rows int) {
	tb.Helper()

	const chunk = 200
	err := app.RunInTransaction(func(txApp core.App) error {
		for start := 0; start < rows; start += chunk {
			end := min(start+chunk, rows)

			var b strings.Builder
			b.WriteString("INSERT INTO " + analytics.CollectionName +
				" (path, date, device_type, browser, views, unique_sessions, returning_sessions, created, updated) VALUES ")
			params := dbx.Params{}

			for i := start; i < end; i++ {
				if i > start {
					b.WriteString(",")
				}
				p := fmt.Sprintf("p%d", i)
				fmt.Fprintf(&b, "({:%[1]sa},{:%[1]sb},'desktop','chrome',10,2,1,'','')", p)
				params[p+"a"] = fmt.Sprintf("/p%d", i)
				params[p+"b"] = time.Now().AddDate(0, 0, -(i % analytics.LookbackDays)).Format("2006-01-02")
			}

			if _, err := txApp.NonconcurrentDB().NewQuery(b.String()).Bind(params).Execute(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		tb.Fatalf("seed %d counter rows: %v", rows, err)
	}
}

// --- request path ---

// BenchmarkTrack measures the per-page-view cost now that the request path is
// purely in-memory.
func BenchmarkTrack(b *testing.B) {
	_, a := testutil.NewAnalytics(b, neverFlush())
	r := benchRequest("/pricing")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Track("1.2.3.4", r)
	}
}

// BenchmarkTrackParallel is the headline scaling check: throughput should now
// improve with cores instead of serializing on SQLite's single writer.
// Run with -cpu 1,8,32 to see the curve.
func BenchmarkTrackParallel(b *testing.B) {
	_, a := testutil.NewAnalytics(b, neverFlush())

	// Pre-build inputs so the benchmark measures Track rather than
	// httptest.NewRequest, which allocates several KB per call.
	paths := make([]*http.Request, 20)
	for i := range paths {
		paths[i] = benchRequest(fmt.Sprintf("/p/%d", i))
	}
	ips := make([]string, 1024)
	for i := range ips {
		ips[i] = fmt.Sprintf("10.0.%d.%d", i%255, (i/255)%255)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			a.Track(ips[i%len(ips)], paths[i%len(paths)])
			i++
		}
	})
}

// BenchmarkTrackParallelUniqueVisitors is the worst case for the visitor
// tracker: every request presents a previously unseen client, forcing a map
// insert and periodic generation rotation.
func BenchmarkTrackParallelUniqueVisitors(b *testing.B) {
	_, a := testutil.NewAnalytics(b, neverFlush())
	var counter atomic.Int64
	r := benchRequest("/")

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			a.Track(fmt.Sprintf("198.51.100.%d", counter.Add(1)), r)
		}
	})
}

// BenchmarkTrackHighCardinalityPaths is the worst case for the path budget:
// every request targets a distinct URL.
func BenchmarkTrackHighCardinalityPaths(b *testing.B) {
	_, a := testutil.NewAnalytics(b, neverFlush())

	// Four times the default path budget, so the run exercises both the
	// "already counted" lookup and the overflow bucket.
	paths := make([]*http.Request, analytics.DefaultMaxDistinctPaths*4)
	for i := range paths {
		paths[i] = benchRequest(fmt.Sprintf("/junk-%d", i))
	}
	var counter atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			a.Track("1.2.3.4", paths[int(counter.Add(1))%len(paths)])
		}
	})
}

// --- flush ---

// BenchmarkFlush measures the cost of persisting a batch of counters, which is
// paid once per FlushInterval rather than once per request.
func BenchmarkFlush(b *testing.B) {
	for _, keys := range []int{1, 100, 1000, 5000} {
		b.Run(fmt.Sprintf("keys=%d", keys), func(b *testing.B) {
			_, a := testutil.NewAnalytics(b, neverFlush(), analytics.WithMaxDistinctPaths(keys))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				for k := 0; k < keys; k++ {
					a.Track("1.2.3.4", benchRequest(fmt.Sprintf("/p%d", k)))
				}
				b.StartTimer()

				if err := a.Flush(); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportMetric(float64(keys), "rows/flush")
		})
	}
}

// --- dashboard ---

// BenchmarkGetData measures the dashboard aggregate against a populated table.
// The queries are bounded by LookbackDays and served from an index.
func BenchmarkGetData(b *testing.B) {
	for _, rows := range []int{1000, 50000} {
		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			app, a := testutil.NewAnalytics(b, neverFlush(), analytics.WithCacheTTL(0))
			seedCounters(b, app, rows)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := a.GetData(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGetDataCached measures repeat dashboard loads inside the cache TTL.
func BenchmarkGetDataCached(b *testing.B) {
	app, a := testutil.NewAnalytics(b, neverFlush(), analytics.WithCacheTTL(time.Minute))
	seedCounters(b, app, 50000)

	if _, err := a.GetData(); err != nil { // warm the cache
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.GetData(); err != nil {
			b.Fatal(err)
		}
	}
}

// --- sustained load ---

// TestStress_SustainedLoad drives the collector with every worker the machine
// has for a fixed wall-clock window, then verifies that every single page view
// survived the memory buffer, the background flusher and the final Close.
//
// It also reports achieved throughput, which is the number that matters for the
// question this package exists to answer.
func TestStress_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in -short mode")
	}

	app, a := testutil.NewAnalytics(t,
		analytics.WithFlushInterval(50*time.Millisecond),
		analytics.WithMaxDistinctPaths(500),
	)

	workers := runtime.GOMAXPROCS(0)
	const duration = 2 * time.Second

	var tracked atomic.Int64
	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			paths := make([]*http.Request, 50)
			for i := range paths {
				paths[i] = benchRequest(fmt.Sprintf("/p%d", i))
			}

			for i := 0; ; i++ {
				if i%256 == 0 && time.Now().After(deadline) {
					return
				}
				a.Track(fmt.Sprintf("10.%d.%d.%d", w%255, i%255, (i/255)%255), paths[i%len(paths)])
				tracked.Add(1)
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// Close performs the final flush.
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rows, views, _, _ := testutil.AnalyticsTotals(t, app)
	want := int(tracked.Load())

	if views != want {
		t.Fatalf("persisted views = %d, want %d (%d lost)", views, want, want-views)
	}

	perSec := float64(want) / elapsed.Seconds()
	t.Logf("%d workers tracked %d page views in %v (%.0f views/sec) -> %d rows",
		workers, want, elapsed.Round(time.Millisecond), perSec, rows)

	current, max := a.VisitorMemory()
	t.Logf("visitor memory: %d/%d entries", current, max)
}

// TestStress_FlushKeepsUpWithTraffic verifies the background flusher drains the
// pending set while traffic is ongoing, so memory does not grow unbounded
// between flushes.
func TestStress_FlushKeepsUpWithTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped in -short mode")
	}

	_, a := testutil.NewAnalytics(t,
		analytics.WithFlushInterval(20*time.Millisecond),
		analytics.WithMaxDistinctPaths(200),
	)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	var maxPending atomic.Int64

	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				a.Track(fmt.Sprintf("10.%d.0.%d", w, i%255), benchRequest(fmt.Sprintf("/p%d", i%400)))
			}
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if p := int64(a.PendingCounters()); p > maxPending.Load() {
				maxPending.Store(p)
			}
			time.Sleep(time.Millisecond)
		}
	}()

	time.Sleep(time.Second)
	close(stop)
	wg.Wait()

	// 200 distinct paths plus the overflow bucket is the ceiling for a single
	// day, so pending must never exceed that regardless of request volume.
	const ceiling = 201
	if got := maxPending.Load(); got > ceiling {
		t.Fatalf("peak pending counters = %d, want <= %d", got, ceiling)
	}
	t.Logf("peak pending counters under sustained load: %d (ceiling %d)", maxPending.Load(), ceiling)
}
