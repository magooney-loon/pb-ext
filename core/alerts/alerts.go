// Package alerts delivers operational notifications — crashes, failed cron
// jobs, traffic and error spikes, and anything the embedding app wants to
// report — to a chat transport, Telegram being the built-in one.
//
// Three properties shape everything here:
//
//   - Emitting an alert never does I/O. Send folds the message into a bounded
//     in-memory queue under a short mutex and returns; a single background
//     worker owns every network call and every database write. Alerts are
//     emitted from request handlers and from panic recovery, so an unreachable
//     Telegram must cost a request nothing at all.
//   - Nothing here can break the server. A missing token, a revoked bot, a
//     network partition and a full queue are all ordinary states with defined
//     behaviour; none of them returns an error to the caller, and none of them
//     delays startup or shutdown.
//   - Get never returns nil. An unconfigured notifier is a working no-op, so
//     user code writes alerts.Get().Send(...) with no guard.
package alerts

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// backoffSchedule is the delay before each retry of a transient failure.
// A 429 overrides it with Telegram's own retry_after.
var backoffSchedule = []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second}

// maxCooldownKeys bounds the cooldown map. User code is free to invent a fresh
// Key per message; without a ceiling that map is an unbounded leak, so the
// oldest entries are pruned once it grows past this.
const maxCooldownKeys = 1000

// digestKey is the bucket suppressions are counted under when the hourly cap,
// rather than a per-key cooldown, is what stopped a message.
const digestKey = "hourly cap"

// Notifier owns the queue, the worker and the rule evaluator.
//
// The zero value is a valid disabled notifier: every method checks for it, so
// Get can hand one out rather than nil.
type Notifier struct {
	app       core.App
	cfg       Config
	transport Transport

	queue    chan Message
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once

	// mu guards everything below it. It is held only for map and counter
	// updates — never across I/O — because Send takes it on a request path.
	mu         sync.Mutex
	lastSent   map[string]time.Time
	suppressed map[string]int
	hourStart  time.Time
	hourCount  int
	counters   counters

	rules   ruleSet
	metrics MetricsFunc

	// eval is the derived metrics view. Only the evaluator goroutine touches
	// it, which is why it needs no lock — see sample.
	eval evaluation

	marker markerState
}

// counters are the delivery tallies reported by Stats.
type counters struct {
	sent          uint64
	failed        uint64
	dropped       uint64
	suppressed    uint64
	lastSent      time.Time
	lastError     string
	lastErrorTime time.Time
	misconfigured bool
	reason        string
}

// Initialize resolves the configuration, starts the worker and the rule
// evaluator, and returns a ready notifier. It also installs the result as the
// package singleton returned by Get.
//
// It deliberately returns no error. Alerting is diagnostic machinery: a server
// that refuses to boot because its notification channel is misconfigured has
// turned a convenience into an outage. Every failure is reported through the
// log, Stats().Misconfigured and the dashboard instead.
func Initialize(app core.App, opts ...Option) *Notifier {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.applyEnv()
	cfg.normalize()

	n := &Notifier{
		app:        app,
		cfg:        cfg,
		lastSent:   make(map[string]time.Time),
		suppressed: make(map[string]int),
		hourStart:  time.Now(),
		metrics:    cfg.metrics,
	}

	transport, enabled, misconfigured, reason := resolveTransport(app, &n.cfg)
	n.transport = transport
	n.counters.reason = reason
	n.counters.misconfigured = misconfigured

	if app != nil {
		n.marker.path = markerPath(app.DataDir())
		// Read before anything overwrites it: this is the only evidence that the
		// previous process died without running its shutdown hook.
		n.marker.previous, n.marker.hasPrevious = readMarker(n.marker.path)
	}

	// Persistence is best-effort: a missing table is a broken install, not a
	// reason to lose the alerts themselves. Checked before the enabled gate so
	// that SendTest — which works while alerts are disabled — still logs.
	if n.cfg.Persist && app != nil && !app.AuxHasTable(TableName) {
		n.cfg.Persist = false
		app.Logger().Error(
			"Alert history disabled: table is missing from auxiliary.db",
			"table", TableName,
			"migration", MigrationFile,
		)
	}

	if !enabled {
		n.cfg.Enabled = false
		if app != nil && reason != "" {
			app.Logger().Debug("Alerts disabled", "reason", reason)
		}
		setGlobal(n)
		return n
	}

	n.cfg.Enabled = true

	n.queue = make(chan Message, n.cfg.QueueSize)
	n.stop = make(chan struct{})
	n.stopped = make(chan struct{})

	n.registerBuiltinRules()

	go n.worker()
	go n.evaluator()
	go n.verify()

	if app != nil {
		app.Logger().Info("✅ Alerts initialized",
			"transport", n.transport.Name(),
			"target", n.transport.Target(),
			"rules", n.rules.len(),
		)
	}

	setGlobal(n)
	return n
}

// resolveTransport decides whether alerting runs, and with what.
//
// The order matters: an explicit WithEnabled beats credential detection, and
// the PBEXT_ALERTS_ENABLED kill switch beats everything, so an operator can
// silence a noisy deployment without a rebuild.
func resolveTransport(app core.App, cfg *Config) (t Transport, enabled, misconfigured bool, reason string) {
	switch {
	case cfg.transport != nil:
		t = cfg.transport
	case cfg.BotToken != "" && cfg.ChatID != "":
		t = newTelegramTransport(*cfg)
	}

	if cfg.disabledByEnv {
		return t, false, false, "disabled by PBEXT_ALERTS_ENABLED"
	}
	if cfg.enabledSet && !cfg.Enabled {
		return t, false, false, "disabled by configuration"
	}
	if t == nil {
		// Asking for alerts without giving anywhere to send them is a
		// misconfiguration worth surfacing on the dashboard; not asking at all
		// is just the default state and says nothing.
		if cfg.enabledSet && cfg.Enabled {
			return nil, false, true, "no transport configured: set a bot token and chat id"
		}
		if cfg.BotToken != "" || cfg.ChatID != "" {
			return nil, false, true, "incomplete Telegram configuration: both a bot token and a chat id are required"
		}
		return nil, false, false, "not configured"
	}
	if app != nil && app.IsDev() && !cfg.EnabledInDev {
		// pb-cli restarts the server on every file save; without this, each save
		// fires a shutdown notice and a startup notice.
		return t, false, false, "disabled in developer mode (use WithEnabledInDev to override)"
	}

	return t, true, false, ""
}

// verify confirms the credentials in the background. It never blocks startup:
// the point is only that a revoked token is discovered at boot rather than
// during the first incident.
func (n *Notifier) verify() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := n.transport.Verify(ctx); err != nil {
		var se *SendError
		if errors.As(err, &se) && se.Permanent {
			n.setMisconfigured(err.Error())
			if n.app != nil {
				n.app.Logger().Error("Alert transport rejected the credentials", "error", err)
			}
			return
		}
		// A transient failure here says nothing about the configuration.
		if n.app != nil {
			n.app.Logger().Warn("Could not verify alert transport", "error", err)
		}
		return
	}

	if n.app != nil {
		n.app.Logger().Info("Alert transport verified", "target", n.transport.Target())
	}
}

// Enabled reports whether alerts are actually being delivered.
func (n *Notifier) Enabled() bool {
	return n != nil && n.queue != nil
}

// Config returns the resolved configuration. The bot token is redacted.
func (n *Notifier) Config() Config {
	if n == nil {
		return Config{}
	}
	cfg := n.cfg
	cfg.BotToken = redactToken(cfg.BotToken)
	return cfg
}

// Send queues a message for delivery.
//
// It performs no I/O and never blocks: the cooldown check is a map lookup under
// a short mutex, and the queue send is non-blocking. It is safe to call from a
// request handler, from panic recovery, and on a disabled or zero-value
// notifier.
//
// When the queue is full the message is dropped and counted rather than
// blocking the caller. Dropping is the right failure here: messages are already
// deduplicated, so a full queue means the transport is wedged, and 256 queued
// copies of "CPU high" carry no more information than one. The count surfaces
// in the next digest and on the dashboard.
func (n *Notifier) Send(m Message) {
	if n == nil || n.queue == nil {
		return
	}
	if m.Title == "" {
		m.Title = "Alert"
	}

	if !n.admit(m, time.Now()) {
		return
	}

	select {
	case n.queue <- m:
	default:
		n.mu.Lock()
		n.counters.dropped++
		n.suppressed["queue full"]++
		n.mu.Unlock()
	}
}

// Sendf queues a message built from a format string, for callers that just want
// a line of text.
func (n *Notifier) Sendf(level Level, format string, args ...any) {
	n.Send(Message{Level: level, Title: fmt.Sprintf(format, args...)})
}

// admit applies the per-key cooldown and the hourly cap.
//
// The cooldown clock starts at enqueue rather than at delivery, so a storm that
// cannot be delivered is throttled just as firmly as one that can.
func (n *Notifier) admit(m Message, now time.Time) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if now.Sub(n.hourStart) >= time.Hour {
		n.hourStart = now
		n.hourCount = 0
	}

	if m.Key != "" && n.cfg.Cooldown > 0 {
		if last, seen := n.lastSent[m.Key]; seen && now.Sub(last) < n.cfg.Cooldown {
			n.suppressed[m.Key]++
			n.counters.suppressed++
			return false
		}
	}

	if n.hourCount >= n.cfg.MaxAlertsPerHour {
		n.suppressed[digestKey]++
		n.counters.suppressed++
		return false
	}

	if m.Key != "" {
		n.pruneCooldownsLocked(now)
		n.lastSent[m.Key] = now
	}
	n.hourCount++
	return true
}

// pruneCooldownsLocked keeps the cooldown map bounded. Callers are free to use
// a unique Key per message, which would otherwise grow this map forever.
func (n *Notifier) pruneCooldownsLocked(now time.Time) {
	if len(n.lastSent) < maxCooldownKeys {
		return
	}
	for k, t := range n.lastSent {
		if now.Sub(t) >= n.cfg.Cooldown {
			delete(n.lastSent, k)
		}
	}
	// Still full of live entries: drop the lot rather than grow without bound.
	// Losing cooldown state costs at most one duplicate alert per key.
	if len(n.lastSent) >= maxCooldownKeys {
		clear(n.lastSent)
	}
}

// worker owns every delivery. One goroutine means ordering is preserved and the
// per-chat rate limit is enforced by construction rather than by a lock.
func (n *Notifier) worker() {
	defer close(n.stopped)

	digestTicker := time.NewTicker(time.Minute)
	defer digestTicker.Stop()

	var lastSend time.Time

	for {
		select {
		case <-n.stop:
			n.drain()
			return

		case <-digestTicker.C:
			if m, ok := n.takeDigest(false); ok {
				n.deliver(context.Background(), m, &lastSend)
			}

		case m := <-n.queue:
			n.deliver(context.Background(), m, &lastSend)
		}
	}
}

// drain makes a bounded attempt to deliver what is still queued at shutdown.
// A wedged transport must not hold the process open, so the whole drain shares
// one deadline.
func (n *Notifier) drain() {
	ctx, cancel := context.WithTimeout(context.Background(), n.cfg.DrainTimeout)
	defer cancel()

	// Pacing is dropped here: the queue is short, the deadline is the real
	// bound, and a shutdown notice is worth more than a rate-limit courtesy.
	var noPacing time.Time

	for {
		select {
		case m := <-n.queue:
			if ctx.Err() != nil {
				n.mu.Lock()
				n.counters.dropped++
				n.mu.Unlock()
				continue
			}
			n.deliver(ctx, m, &noPacing)
		default:
			return
		}
	}
}

// deliver sends one message, retrying transient failures.
func (n *Notifier) deliver(ctx context.Context, m Message, lastSend *time.Time) {
	if !n.pace(ctx, lastSend) {
		n.record(m, StatusDropped, 0, errors.New("shutting down"))
		return
	}

	var (
		lastErr  error
		attempts int
	)

	for attempt := 0; attempt <= n.cfg.MaxRetries; attempt++ {
		attempts++

		attemptCtx, cancel := context.WithTimeout(ctx, n.cfg.SendTimeout)
		err := n.transport.Send(attemptCtx, m, n.cfg.Instance)
		cancel()

		*lastSend = time.Now()

		if err == nil {
			n.record(m, StatusSent, attempts, nil)
			return
		}
		lastErr = err

		var se *SendError
		if errors.As(err, &se) {
			if se.Permanent {
				// A bad token or a wrong chat id will fail identically forever.
				// Retrying it is a hot loop against someone else's API.
				n.setMisconfigured(err.Error())
				break
			}
			if se.RetryAfter > 0 {
				if !n.sleep(ctx, se.RetryAfter) {
					break
				}
				continue
			}
		}

		if attempt < n.cfg.MaxRetries {
			if !n.sleep(ctx, n.backoffFor(attempt)) {
				break
			}
		}
	}

	n.record(m, StatusFailed, attempts, lastErr)
}

// pace enforces the minimum gap between sends, staying inside Telegram's
// per-chat rate limit without needing to see a 429 first.
func (n *Notifier) pace(ctx context.Context, lastSend *time.Time) bool {
	if n.cfg.MinSendInterval <= 0 || lastSend.IsZero() {
		return true
	}
	wait := n.cfg.MinSendInterval - time.Since(*lastSend)
	if wait <= 0 {
		return true
	}
	return n.sleep(ctx, wait)
}

// sleep waits, returning false if the context expired or the notifier is
// shutting down.
//
// Watching n.stop directly, rather than a context cancelled alongside it, is
// what makes shutdown behave: the worker's select can pick a queued message in
// the same instant Close fires, and that message must still get its one
// best-effort attempt. Cancelling the delivery context instead would fail it
// before it was ever tried. All sleeping stops at shutdown — a 30-second retry
// backoff must not delay process exit — but sending does not.
func (n *Notifier) sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	case <-n.stop:
		return false
	}
}

func (n *Notifier) backoffFor(attempt int) time.Duration {
	schedule := n.cfg.backoff
	if len(schedule) == 0 {
		schedule = backoffSchedule
	}
	if attempt < len(schedule) {
		return schedule[attempt]
	}
	return schedule[len(schedule)-1]
}

// record updates the counters and appends to the delivery log.
func (n *Notifier) record(m Message, status string, attempts int, err error) {
	n.mu.Lock()
	switch status {
	case StatusSent:
		n.counters.sent++
		n.counters.lastSent = time.Now()
	case StatusDropped:
		n.counters.dropped++
	default:
		n.counters.failed++
	}
	if err != nil {
		n.counters.lastError = err.Error()
		n.counters.lastErrorTime = time.Now()
	}
	n.mu.Unlock()

	rec := Record{
		Level:     m.Level.String(),
		Key:       m.Key,
		Title:     m.Title,
		Text:      m.Text,
		Transport: n.transport.Name(),
		Status:    status,
		Attempts:  attempts,
	}
	if err != nil {
		rec.Error = err.Error()
	}
	n.persist(rec)

	if err != nil && n.app != nil {
		n.app.Logger().Warn("Alert delivery failed",
			"title", m.Title,
			"status", status,
			"attempts", attempts,
			"error", err,
		)
	}
}

// takeDigest builds the summary of everything suppressed in the past window and
// resets the tally. force ignores the window, for shutdown.
func (n *Notifier) takeDigest(force bool) (Message, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.suppressed) == 0 {
		return Message{}, false
	}
	if !force && time.Since(n.hourStart) < time.Hour {
		return Message{}, false
	}

	total := 0
	fields := make(map[string]string, len(n.suppressed))
	for k, count := range n.suppressed {
		total += count
		fields[k] = fmt.Sprintf("%d", count)
	}
	clear(n.suppressed)

	return Message{
		Level:  LevelWarn,
		Title:  fmt.Sprintf("%d alerts suppressed", total),
		Text:   "Repeated alerts held back by the cooldown, the hourly cap or a full queue.",
		Fields: fields,
	}, true
}

// Stats returns a point-in-time view for the dashboard and status endpoint.
// It never contains the bot token.
func (n *Notifier) Stats() Stats {
	if n == nil {
		return Stats{}
	}

	n.mu.Lock()
	c := n.counters
	n.mu.Unlock()

	s := Stats{
		Enabled:       n.queue != nil,
		Misconfigured: c.misconfigured,
		Reason:        c.reason,
		Instance:      n.cfg.Instance,
		QueueSize:     n.cfg.QueueSize,
		Sent:          c.sent,
		Failed:        c.failed,
		Dropped:       c.dropped,
		Suppressed:    c.suppressed,
		LastSent:      c.lastSent,
		LastError:     c.lastError,
		LastErrorTime: c.lastErrorTime,
	}
	if n.queue != nil {
		s.Queued = len(n.queue)
	}
	if n.transport != nil {
		s.Transport = n.transport.Name()
		s.Target = n.transport.Target()
	}
	s.Rules, s.Firing = n.rules.counts()

	return s
}

// setMisconfigured records a permanent transport failure. Deliveries keep being
// attempted — the operator may fix the token without a restart — but the state
// is visible on the dashboard rather than buried in a log.
func (n *Notifier) setMisconfigured(reason string) {
	n.mu.Lock()
	n.counters.misconfigured = true
	n.counters.reason = reason
	n.mu.Unlock()
}

// Close stops the worker and the evaluator, flushing what it can within
// DrainTimeout. It is safe to call more than once, and on a disabled notifier.
func (n *Notifier) Close() error {
	if n == nil || n.queue == nil {
		return nil
	}

	n.stopOnce.Do(func() {
		// Anything held back is worth one last line before the process exits.
		if m, ok := n.takeDigest(true); ok {
			select {
			case n.queue <- m:
			default:
			}
		}
		close(n.stop)
		<-n.stopped
	})
	return nil
}

// --- global singleton ---

var (
	global   atomic.Pointer[Notifier]
	disabled = &Notifier{}
)

func setGlobal(n *Notifier) { global.Store(n) }

// Get returns the notifier installed by Initialize.
//
// It never returns nil: before Initialize runs, or when alerts are not
// configured, it hands back a disabled notifier whose Send is a no-op. That is
// what lets user code call alerts.Get().Send(...) unguarded.
func Get() *Notifier {
	if n := global.Load(); n != nil {
		return n
	}
	return disabled
}
