package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	pbext "github.com/magooney-loon/pb-ext/core"
	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/monitoring"
)

// dbSizes is a snapshot of both SQLite files pb-ext writes to, plus their WAL
// files — see the "auxiliary.db split" note in core/analytics/CLAUDE.md.
type dbSizes struct {
	dataDB, dataWAL, auxDB, auxWAL int64
}

func readDBSizes(dataDir string) dbSizes {
	stat := func(name string) int64 {
		fi, err := os.Stat(filepath.Join(dataDir, name))
		if err != nil {
			return 0
		}
		return fi.Size()
	}
	return dbSizes{
		dataDB:  stat("data.db"),
		dataWAL: stat("data.db-wal"),
		auxDB:   stat("auxiliary.db"),
		auxWAL:  stat("auxiliary.db-wal"),
	}
}

// resourceSample is one point in a stage's resource time series.
type resourceSample struct {
	at         time.Time
	cpuPercent float64
	rssBytes   uint64
	diskUsed   float64
	db         dbSizes
}

// kindSummary is the final per-traffic-mix-entry tally for a stage, taken
// after the worker pool has stopped so the atomics can be read plainly.
type kindSummary struct {
	label    string
	requests int64
	errors   int64
}

// stageResult is everything reported for one entry in the -workers ramp.
type stageResult struct {
	workers  int
	duration time.Duration

	requests           int64
	errors             int64
	transport          int64
	p50, p90, p99, max time.Duration

	byKind []kindSummary

	samples []resourceSample

	dbBefore, dbAfter dbSizes
	benchItems        int64
	analyticsRows     int64
	analyticsViews    int64
}

func (s stageResult) reqPerSec() float64 {
	if s.duration <= 0 {
		return 0
	}
	return float64(s.requests) / s.duration.Seconds()
}

func (s stageResult) errRate() float64 {
	if s.requests == 0 {
		return 0
	}
	return float64(s.errors) / float64(s.requests)
}

func (s stageResult) cpuAvgPeak() (avg, peak float64) {
	if len(s.samples) == 0 {
		return 0, 0
	}
	var sum float64
	for _, sm := range s.samples {
		sum += sm.cpuPercent
		if sm.cpuPercent > peak {
			peak = sm.cpuPercent
		}
	}
	return sum / float64(len(s.samples)), peak
}

func (s stageResult) rssAvgPeak() (avg, peak uint64) {
	if len(s.samples) == 0 {
		return 0, 0
	}
	var sum uint64
	for _, sm := range s.samples {
		sum += sm.rssBytes
		if sm.rssBytes > peak {
			peak = sm.rssBytes
		}
	}
	return sum / uint64(len(s.samples)), peak
}

func (s stageResult) diskAvgPeak() (avg, peak float64) {
	if len(s.samples) == 0 {
		return 0, 0
	}
	var sum float64
	for _, sm := range s.samples {
		sum += sm.diskUsed
		if sm.diskUsed > peak {
			peak = sm.diskUsed
		}
	}
	return sum / float64(len(s.samples)), peak
}

// runStage drives one ramp step: launches the monitor sampler, runs the load
// generator to completion (blocking for -duration), then snapshots database
// state and assembles the result.
func runStage(srv *pbext.Server, client *http.Client, addr, authToken, dataDir string, workers int, duration, sampleInterval time.Duration, stageNum, stageTotal int) stageResult {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	dbBefore := readDBSizes(dataDir)
	startedAt := time.Now()
	result := newLoadResult()

	monitorDone := make(chan []resourceSample, 1)
	go func() {
		monitorDone <- sampleStage(ctx, srv, dataDir, sampleInterval, startedAt, duration, result, stageNum, stageTotal, workers, dbBefore)
	}()

	runLoad(ctx, client, addr, authToken, workers, result)
	samples := <-monitorDone

	elapsed := time.Since(startedAt)
	dbAfter := readDBSizes(dataDir)

	byKind := make([]kindSummary, len(trafficMix))
	for i, k := range trafficMix {
		byKind[i] = kindSummary{
			label:    k.label,
			requests: result.byKind[i].requests.Load(),
			errors:   result.byKind[i].errors.Load(),
		}
	}

	benchItems, analyticsRows, analyticsViews := readRowCounts(srv)

	return stageResult{
		workers:        workers,
		duration:       elapsed,
		requests:       result.total.Load(),
		errors:         result.errors.Load(),
		transport:      result.transport.Load(),
		p50:            result.hist.percentile(0.50),
		p90:            result.hist.percentile(0.90),
		p99:            result.hist.percentile(0.99),
		max:            result.hist.max(),
		byKind:         byKind,
		samples:        samples,
		dbBefore:       dbBefore,
		dbAfter:        dbAfter,
		benchItems:     benchItems,
		analyticsRows:  analyticsRows,
		analyticsViews: analyticsViews,
	}
}

// sampleStage samples system + DB state on a ticker until ctx is done,
// printing one progress line per sample so a long stage isn't silent.
func sampleStage(ctx context.Context, srv *pbext.Server, dataDir string, interval time.Duration, startedAt time.Time, duration time.Duration, result *loadResult, stageNum, stageTotal, workers int, dbBefore dbSizes) []resourceSample {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var samples []resourceSample
	for {
		select {
		case <-ctx.Done():
			return samples
		case <-ticker.C:
			sample := takeSample(ctx, dataDir)
			samples = append(samples, sample)
			printProgress(stageNum, stageTotal, workers, time.Since(startedAt), duration, result, sample, dbBefore)
		}
	}
}

func takeSample(ctx context.Context, dataDir string) resourceSample {
	stats, _ := monitoring.CollectSystemStats(ctx, time.Now(), dataDir)
	sample := resourceSample{at: time.Now(), db: readDBSizes(dataDir)}
	if stats != nil {
		sample.cpuPercent = stats.ProcessStats.CPUPercent
		sample.rssBytes = stats.ProcessStats.RSS
		sample.diskUsed = stats.DiskUsagePercent
	}
	return sample
}

func printProgress(stageNum, stageTotal, workers int, elapsed, duration time.Duration, result *loadResult, sample resourceSample, dbBefore dbSizes) {
	total := result.total.Load()
	errs := result.errors.Load()
	errPct := 0.0
	if total > 0 {
		errPct = float64(errs) / float64(total) * 100
	}
	fmt.Printf("[stage %d/%d workers=%-4d] %5s/%s  %8d req  %8.1f rps  p99=%-8s err=%5.2f%%  cpu=%5.1f%%  rss=%-8s  data.db=%+8s  aux.db=%+8s\n",
		stageNum, stageTotal, workers,
		elapsed.Round(time.Second), duration,
		total, float64(total)/max(elapsed.Seconds(), 0.001),
		result.hist.percentile(0.99),
		errPct,
		sample.cpuPercent,
		formatBytes(int64(sample.rssBytes)),
		formatDelta(sample.db.dataDB-dbBefore.dataDB),
		formatDelta(sample.db.auxDB-dbBefore.auxDB),
	)
}

// readRowCounts reads bench_items (data.db) and _analytics (auxiliary.db)
// state directly — the same query shape as testutil.AnalyticsTotals, inlined
// here since that helper takes a testing.TB.
func readRowCounts(srv *pbext.Server) (benchItems, analyticsRows, analyticsViews int64) {
	_ = srv.App().DB().Select("COUNT(*)").From(benchCollectionName).Row(&benchItems)
	_ = srv.App().AuxDB().
		Select("COUNT(*)", "COALESCE(SUM(views),0)").
		From(analytics.TableName).
		Row(&analyticsRows, &analyticsViews)
	return benchItems, analyticsRows, analyticsViews
}
