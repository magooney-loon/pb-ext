package analytics

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// flushChunkRows is how many counter rows go into a single INSERT statement.
// At 7 bound parameters per row this stays far below SQLite's variable limit.
const flushChunkRows = 100

// Analytics tracks page views as aggregated daily counters.
//
// The request path is entirely in-memory: a view is folded into a counter map,
// a recent-visit ring and a minute bucket, all under one short mutex. A
// background goroutine persists accumulated deltas every FlushInterval in a
// single transaction, so request throughput no longer depends on SQLite's
// single writer connection.
//
// No personal data (IP, user agent, visitor id) is ever persisted. The visitor
// map is memory-only, keyed by a non-reversible hash, and bounded.
type Analytics struct {
	app core.App
	cfg Config

	agg      *aggregator
	visitors *visitorTracker

	flushNow chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
	stopped  chan struct{}

	// cacheMu guards the memoized dashboard aggregate.
	cacheMu     sync.Mutex
	cached      *dbAggregates
	cachedUntil time.Time
}

// New creates an Analytics instance without starting its background worker.
// Use Initialize for normal startup.
func New(app core.App, opts ...Option) *Analytics {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.normalize()

	now := time.Now()

	return &Analytics{
		app:      app,
		cfg:      cfg,
		agg:      newAggregator(cfg.MaxPendingCounters, cfg.MaxDistinctPaths, now),
		visitors: newVisitorTracker(cfg.VisitorGenerations, cfg.MaxVisitorsPerGeneration, cfg.SessionWindow, now),
		flushNow: make(chan struct{}, 1),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// Initialize verifies the schema, starts the flush worker and returns an
// Analytics ready to serve requests.
func Initialize(app core.App, opts ...Option) (*Analytics, error) {
	app.Logger().Info("Initializing analytics system")

	// The schema is created by the registered migration, which PocketBase runs
	// before serving. Fail loudly rather than silently collecting into nothing.
	if !app.AuxHasTable(TableName) {
		return nil, fmt.Errorf(
			"%s table is missing from auxiliary.db — pb-ext migration %s has not been applied",
			TableName, MigrationFile,
		)
	}

	a := New(app, opts...)
	go a.flushWorker()

	return a, nil
}

// Config returns the resolved configuration.
func (a *Analytics) Config() Config {
	return a.cfg
}

// Close stops the flush worker and persists whatever is still pending.
// It is safe to call more than once.
func (a *Analytics) Close() error {
	a.stopOnce.Do(func() {
		close(a.stop)
		<-a.stopped
	})
	return a.Flush()
}

// flushWorker persists pending counters on a timer, on demand when the pending
// set grows past its ceiling, and one last time on shutdown.
func (a *Analytics) flushWorker() {
	defer close(a.stopped)

	ticker := time.NewTicker(a.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
		case <-a.flushNow:
		}

		if err := a.Flush(); err != nil {
			a.app.Logger().Error("analytics flush failed", "error", err)
		}
	}
}

// requestFlush asks the worker to flush early without blocking the caller.
func (a *Analytics) requestFlush() {
	select {
	case a.flushNow <- struct{}{}:
	default: // a flush is already queued
	}
}

// Flush writes all accumulated counters in a single transaction.
// It is a no-op when nothing is pending.
func (a *Analytics) Flush() error {
	deltas := a.agg.drain()
	if len(deltas) == 0 {
		return nil
	}

	if err := a.writeDeltas(deltas); err != nil {
		// Keep the counts so a transient failure doesn't lose data; restore is
		// itself bounded, so a permanently broken database can't grow memory.
		a.agg.restore(deltas)
		return err
	}

	a.invalidateCache()
	return nil
}

// writeDeltas upserts every accumulated counter inside one transaction, so a
// flush costs a single commit regardless of how many rows changed.
//
// The transaction is on auxiliary.db, which has its own single writer
// connection and its own WAL. That is the point of storing counters there: a
// flush can never block — or be blocked by — an application write to data.db,
// however many rows it carries.
func (a *Analytics) writeDeltas(deltas map[counterKey]*counterDelta) error {
	keys := make([]counterKey, 0, len(deltas))
	for key := range deltas {
		keys = append(keys, key)
	}

	stamp := types.NowDateTime().String()

	return a.app.AuxRunInTransaction(func(txApp core.App) error {
		for start := 0; start < len(keys); start += flushChunkRows {
			end := min(start+flushChunkRows, len(keys))

			chunk := keys[start:end]
			sql, params := buildUpsert(chunk, deltas, stamp)

			if _, err := txApp.AuxNonconcurrentDB().NewQuery(sql).Bind(params).Execute(); err != nil {
				return fmt.Errorf("upsert %d analytics counters: %w", len(chunk), err)
			}
		}
		return nil
	})
}

// buildUpsert renders a multi-row INSERT ... ON CONFLICT DO UPDATE that adds the
// pending deltas onto whatever is already stored.
func buildUpsert(keys []counterKey, deltas map[counterKey]*counterDelta, stamp string) (string, dbx.Params) {
	var b strings.Builder
	params := dbx.Params{}

	b.WriteString("INSERT INTO ")
	b.WriteString(TableName)
	b.WriteString(" (path, date, device_type, browser, views, unique_sessions, returning_sessions, created, updated) VALUES ")

	for i, key := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		p := fmt.Sprintf("p%d", i)
		fmt.Fprintf(&b, "({:%[1]sa},{:%[1]sb},{:%[1]sc},{:%[1]sd},{:%[1]se},{:%[1]sf},{:%[1]sg},{:st},{:st})", p)

		delta := deltas[key]
		params[p+"a"] = key.Path
		params[p+"b"] = key.Date
		params[p+"c"] = key.DeviceType
		params[p+"d"] = key.Browser
		params[p+"e"] = delta.Views
		params[p+"f"] = delta.NewSessions
		params[p+"g"] = delta.ReturningSessions
	}
	params["st"] = stamp

	b.WriteString(` ON CONFLICT (path, date, device_type, browser) DO UPDATE SET
		views = views + excluded.views,
		unique_sessions = unique_sessions + excluded.unique_sessions,
		returning_sessions = returning_sessions + excluded.returning_sessions,
		updated = excluded.updated`)

	return b.String(), params
}
