// Package audit records access to the administrative surfaces — PocketBase's
// admin UI at /_/, pb-ext's dashboard at /_/_, superuser API calls, and every
// superuser authentication attempt — and raises alerts on the patterns that
// look like someone trying to get in.
//
// # Why this exists alongside PocketBase's own request log
//
// PocketBase already logs every request into _logs: url, method, status,
// execution time, referer, user agent, the auth collection, and the client IP
// when Settings.Logs.LogIP is on (it is, by default). Duplicating that would be
// waste. What this package adds is what that log cannot answer:
//
//   - Which account a failed login targeted. The attempted identity arrives in
//     the request body; PocketBase records a 400 and nothing else. For "someone
//     is trying to get into the admin panel", that is the single most useful
//     field, and it exists nowhere else.
//   - Which superuser did something. Settings.Logs.LogAuthId defaults to false,
//     so the built-in log says "a superuser" and not which one.
//   - A retention window suited to an investigation. Logs.MaxDays defaults to
//     5; an intrusion is usually noticed later than that.
//   - A queryable shape. Admin access in _logs is a JSON blob among every other
//     request, with no per-source rollup and nothing to alert on.
//
// # Cost
//
// The request path does no database work: an event is folded into a bounded
// in-memory map under one short mutex, and a background worker writes batches.
// All detection — the brute-force count, the never-seen-before address — runs
// on that worker, never on the request. This matters more here than elsewhere:
// the code runs when the server is being scanned, and must not become the thing
// that falls over.
package audit

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/pocketbase/pocketbase/core"
)

// flushChunkRows is how many rows go into one INSERT. At 17 bound parameters
// per row this stays well below SQLite's variable limit.
const flushChunkRows = 50

// eventKey is the aggregation key: two events matching on all of it, inside one
// flush window, become one row with a count.
//
// It is a comparable struct rather than a joined string so no separator can be
// forged — a user agent containing the separator must not be able to collide
// with a different event's key.
type eventKey struct {
	Kind      string
	Method    string
	Path      string
	Query     string
	Status    int
	Outcome   string
	AuthState string
	Identity  string
	IP        string
	UserAgent string
	Referer   string
}

// eventAgg accumulates the repeats of one key.
type eventAgg struct {
	Count      int
	First      time.Time
	Last       time.Time
	DurationMs float64
	TraceID    string
	Error      string
}

// Auditor buffers admin access events and writes them in batches.
type Auditor struct {
	app      core.App
	cfg      Config
	notifier *alerts.Notifier

	// mu guards everything below. It is taken on the request path, so it is
	// held only for map updates — never across I/O.
	mu      sync.Mutex
	pending map[eventKey]*eventAgg
	dropped uint64
	written uint64
	// droppedReported is the drop count already alerted on, so a persistent
	// flood produces one alert rather than one per flush.
	droppedReported uint64

	// degraded records that the schema is missing, so the dashboard can say so
	// rather than showing an empty log that looks like "no attacks".
	degraded bool

	flushNow chan struct{}
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
}

// New creates an Auditor without starting its worker. Use Initialize.
func New(app core.App, opts ...Option) *Auditor {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.normalize()

	return &Auditor{
		app:      app,
		cfg:      cfg,
		notifier: cfg.notifier,
		pending:  make(map[eventKey]*eventAgg),
		flushNow: make(chan struct{}, 1),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// Initialize starts the flush worker and returns an Auditor ready to record.
//
// It never returns an error. A missing table disables capture and is reported
// loudly — through the log and through a critical alert — because a security
// log that has quietly stopped recording is worse than one that was never
// enabled: the empty table reads as "nothing happened".
func Initialize(app core.App, opts ...Option) *Auditor {
	a := New(app, opts...)

	setGlobal(a)

	if !a.cfg.Enabled {
		app.Logger().Info("Admin access auditing is disabled")
		return a
	}

	if !app.AuxHasTable(TableName) {
		a.degraded = true
		app.Logger().Error(
			"Admin access auditing is NOT recording: table is missing from auxiliary.db",
			"table", TableName,
			"migration", MigrationFile,
		)
		a.alert(alerts.Message{
			Level: alerts.LevelCritical,
			Key:   "audit_degraded",
			Title: "Admin access auditing is not recording",
			Text:  "The " + TableName + " table is missing from auxiliary.db, so administrative access is going unrecorded.",
			Fields: map[string]string{
				"migration": MigrationFile,
			},
		})
		return a
	}

	go a.flushWorker()

	app.Logger().Info("✅ Admin access auditing initialized",
		"retention_days", a.cfg.RetentionDays,
		"records_ip", a.cfg.RecordIP,
		"records_identity", a.cfg.RecordIdentity,
	)

	return a
}

// Config returns the resolved configuration.
func (a *Auditor) Config() Config {
	if a == nil {
		return Config{}
	}
	return a.cfg
}

// Recording reports whether events are actually being captured.
func (a *Auditor) Recording() bool {
	return a != nil && a.cfg.Enabled && !a.degraded
}

// Track records one event. It does no I/O and is safe for concurrent use and on
// a nil Auditor.
func (a *Auditor) Track(ev Event) {
	if !a.Recording() {
		return
	}

	key := a.keyFor(ev)

	a.mu.Lock()
	agg, exists := a.pending[key]
	switch {
	case exists:
		agg.Count++
		agg.Last = ev.At
		// Keep the worst duration seen for this key; an average would hide the
		// slow outlier that made it interesting.
		if ev.DurationMs > agg.DurationMs {
			agg.DurationMs = ev.DurationMs
		}
		if agg.Error == "" {
			agg.Error = truncate(ev.Error, a.cfg.MaxFieldLength)
		}

	case len(a.pending) >= a.cfg.MaxPendingEvents:
		// At the ceiling, only *new* distinct keys are refused. A flood from one
		// address against one path is a single key, so it keeps being counted
		// exactly — which is the case that matters, since that flood is the
		// reason the ceiling was reached.
		a.dropped++
		a.mu.Unlock()
		a.requestFlush()
		return

	default:
		a.pending[key] = &eventAgg{
			Count:      1,
			First:      ev.At,
			Last:       ev.At,
			DurationMs: ev.DurationMs,
			TraceID:    truncate(ev.TraceID, 64),
			Error:      truncate(ev.Error, a.cfg.MaxFieldLength),
		}
	}
	full := len(a.pending) >= a.cfg.MaxPendingEvents
	a.mu.Unlock()

	if full {
		a.requestFlush()
	}
}

// keyFor applies the field policy and the length caps.
//
// Every string here except Kind, Method and Outcome is attacker-controlled, and
// each one is both a storage-size question and a key-cardinality question — an
// unbounded user agent would let a client mint unlimited distinct keys.
func (a *Auditor) keyFor(ev Event) eventKey {
	key := eventKey{
		Kind:      ev.Kind,
		Method:    truncate(ev.Method, 16),
		Path:      truncate(ev.Path, a.cfg.MaxFieldLength),
		Query:     truncate(ev.Query, a.cfg.MaxFieldLength),
		Status:    ev.Status,
		Outcome:   ev.Outcome,
		AuthState: ev.AuthState,
	}

	if a.cfg.RecordIdentity {
		key.Identity = truncate(ev.Identity, 255)
	}
	if a.cfg.RecordIP {
		key.IP = truncate(ev.IP, 64)
	}
	if a.cfg.RecordUserAgent {
		key.UserAgent = truncate(ev.UserAgent, a.cfg.MaxFieldLength)
	}
	if a.cfg.RecordUserAgent {
		key.Referer = truncate(ev.Referer, a.cfg.MaxFieldLength)
	}

	return key
}

// Pending reports how many distinct events are waiting to be written.
func (a *Auditor) Pending() int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

// Close stops the worker and writes whatever is still buffered.
// It is safe to call more than once.
func (a *Auditor) Close() error {
	if a == nil {
		return nil
	}
	a.stopOnce.Do(func() {
		close(a.stop)
		// Only wait for a worker that was actually started.
		if a.Recording() {
			<-a.stopped
		}
	})
	return a.Flush()
}

// flushWorker writes buffered events on a timer, on demand when the buffer
// fills, and one last time on shutdown.
func (a *Auditor) flushWorker() {
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
			a.app.Logger().Error("admin access flush failed", "error", err)
		}
	}
}

// requestFlush asks the worker to flush early without blocking the caller.
func (a *Auditor) requestFlush() {
	select {
	case a.flushNow <- struct{}{}:
	default: // a flush is already queued
	}
}

// Flush writes buffered events and runs detection over them.
func (a *Auditor) Flush() error {
	if !a.Recording() {
		return nil
	}

	a.mu.Lock()
	batch := a.pending
	a.pending = make(map[eventKey]*eventAgg)
	a.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	// Detection reads the table, so it runs before the insert: "has this address
	// ever signed in successfully before" must not be answered by the row we are
	// about to write.
	a.detect(batch)

	if err := a.write(batch); err != nil {
		// Restore what we could not write, subject to the same ceiling, so a
		// transient database failure does not silently erase the audit trail —
		// and cannot grow memory without bound either.
		a.restore(batch)
		return err
	}

	a.mu.Lock()
	for _, agg := range batch {
		a.written += uint64(agg.Count)
	}
	a.mu.Unlock()

	return nil
}

// restore folds an unwritten batch back into the buffer for the next attempt.
func (a *Auditor) restore(batch map[eventKey]*eventAgg) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for key, agg := range batch {
		if existing, ok := a.pending[key]; ok {
			existing.Count += agg.Count
			if agg.First.Before(existing.First) {
				existing.First = agg.First
			}
			if agg.Last.After(existing.Last) {
				existing.Last = agg.Last
			}
			continue
		}
		if len(a.pending) >= a.cfg.MaxPendingEvents {
			a.dropped += uint64(agg.Count)
			continue
		}
		a.pending[key] = agg
	}
}

// alert sends through the configured notifier, falling back to the package
// singleton. Both are safe when alerting is not configured.
func (a *Auditor) alert(m alerts.Message) {
	if a.notifier != nil {
		a.notifier.Send(m)
		return
	}
	alerts.Get().Send(m)
}

// truncate shortens attacker-controlled strings, marking where it cut.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// --- global singleton ---

var global atomic.Pointer[Auditor]

func setGlobal(a *Auditor) { global.Store(a) }

// Get returns the auditor installed by Initialize.
//
// It never returns nil: before Initialize runs, it hands back a disabled
// auditor whose Track is a no-op, so callers need no guard.
func Get() *Auditor {
	if a := global.Load(); a != nil {
		return a
	}
	return disabled
}

var disabled = &Auditor{}
