package alerts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTransport records what it was asked to deliver and can be made to fail or
// to block, which is how the queue and retry behaviour are exercised without a
// network.
type fakeTransport struct {
	mu       sync.Mutex
	sent     []Message
	errs     []error // consumed in order; nil once exhausted
	block    chan struct{}
	delivery chan Message
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{delivery: make(chan Message, 64)}
}

func (f *fakeTransport) Name() string   { return "fake" }
func (f *fakeTransport) Target() string { return "fake-target" }

func (f *fakeTransport) Verify(ctx context.Context) error { return nil }

func (f *fakeTransport) Send(ctx context.Context, m Message, instance string) error {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	f.mu.Lock()
	var err error
	if len(f.errs) > 0 {
		err, f.errs = f.errs[0], f.errs[1:]
	}
	if err == nil {
		f.sent = append(f.sent, m)
	}
	f.mu.Unlock()

	if err == nil {
		select {
		case f.delivery <- m:
		default:
		}
	}
	return err
}

func (f *fakeTransport) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

// newTestNotifier builds a notifier with no app (so nothing touches a database)
// and instant pacing.
func newTestNotifier(t testing.TB, extra ...Option) (*Notifier, *fakeTransport) {
	t.Helper()

	transport := newFakeTransport()
	opts := append([]Option{
		WithTransport(transport),
		WithEnabled(true),
		WithMinSendInterval(0),
		WithPersistence(false),
		WithLifecycleAlerts(false),
		WithEvaluateInterval(time.Hour),
		// The real schedule spends 10s on a retry cycle, which is right in
		// production and useless in a test.
		func(c *Config) { c.backoff = []time.Duration{time.Millisecond} },
	}, extra...)

	n := Initialize(nil, opts...)
	t.Cleanup(func() { _ = n.Close() })

	return n, transport
}

func waitForDelivery(t testing.TB, f *fakeTransport) Message {
	t.Helper()
	select {
	case m := <-f.delivery:
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a delivery")
		return Message{}
	}
}

func TestSend_DeliversThroughTheTransport(t *testing.T) {
	n, transport := newTestNotifier(t)

	n.Send(Message{Level: LevelError, Title: "boom"})

	got := waitForDelivery(t, transport)
	if got.Title != "boom" {
		t.Fatalf("delivered title = %q, want %q", got.Title, "boom")
	}
	// The counter is bumped after the transport returns, so poll rather than
	// assume the worker has caught up.
	waitFor(t, func() bool { return n.Stats().Sent == 1 })
}

// A full queue must never block the caller: Send runs on request paths, where
// blocking on an unreachable Telegram would stall the request itself.
func TestSend_FullQueueDropsWithoutBlocking(t *testing.T) {
	n, transport := newTestNotifier(t, WithQueueSize(1))

	// Wedge the transport so nothing drains.
	transport.block = make(chan struct{})
	defer close(transport.block)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 100 {
			n.Send(Message{Title: "flood", Key: string(rune('a' + i%26))})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Send blocked when the queue was full")
	}

	if got := n.Stats().Dropped; got == 0 {
		t.Fatal("Dropped = 0, want the overflow to be counted")
	}
	if got := len(n.queue); got > 1 {
		t.Fatalf("queue length = %d, want it bounded at 1", got)
	}
}

func TestSend_CooldownSuppressesTheSameKey(t *testing.T) {
	n, transport := newTestNotifier(t, WithCooldown(time.Hour))

	n.Send(Message{Title: "first", Key: "cpu"})
	waitForDelivery(t, transport)

	n.Send(Message{Title: "second", Key: "cpu"})
	n.Send(Message{Title: "third", Key: "disk"})

	got := waitForDelivery(t, transport)
	if got.Title != "third" {
		t.Fatalf("delivered %q, want the differently keyed message", got.Title)
	}

	// Suppression happens synchronously inside Send; delivery does not.
	if got := n.Stats().Suppressed; got != 1 {
		t.Fatalf("Suppressed = %d, want 1", got)
	}
	waitFor(t, func() bool { return n.Stats().Sent == 2 })
}

// An empty Key means "always deliver" — that is what makes an explicit Send
// from application code reliable.
func TestSend_UnkeyedMessagesAreNeverSuppressed(t *testing.T) {
	n, transport := newTestNotifier(t, WithCooldown(time.Hour))

	for range 3 {
		n.Send(Message{Title: "deploy finished"})
	}
	for range 3 {
		waitForDelivery(t, transport)
	}

	if got := n.Stats().Suppressed; got != 0 {
		t.Fatalf("Suppressed = %d, want 0", got)
	}
}

func TestSend_HourlyCapHoldsBackTheOverflow(t *testing.T) {
	n, transport := newTestNotifier(t, WithMaxAlertsPerHour(2))

	for i := range 5 {
		n.Send(Message{Title: "msg", Key: string(rune('a' + i))})
	}
	waitForDelivery(t, transport)
	waitForDelivery(t, transport)

	stats := n.Stats()
	if stats.Suppressed != 3 {
		t.Fatalf("Suppressed = %d, want 3", stats.Suppressed)
	}
}

func TestTakeDigest_SummarisesSuppressedAlerts(t *testing.T) {
	n, _ := newTestNotifier(t, WithMaxAlertsPerHour(1))

	for range 4 {
		n.Send(Message{Title: "noise", Key: "k"})
	}

	msg, ok := n.takeDigest(true)
	if !ok {
		t.Fatal("takeDigest returned nothing, want a summary")
	}
	if !strings.Contains(msg.Title, "suppressed") {
		t.Fatalf("digest title = %q, want it to mention suppression", msg.Title)
	}
	if len(msg.Fields) == 0 {
		t.Fatal("digest carried no per-key counts")
	}

	if _, ok := n.takeDigest(true); ok {
		t.Fatal("takeDigest returned a second summary; the tally should reset")
	}
}

// The cooldown map is keyed by caller-supplied strings, so it needs a ceiling.
func TestCooldownMap_StaysBounded(t *testing.T) {
	n, _ := newTestNotifier(t, WithCooldown(time.Hour), WithMaxAlertsPerHour(1_000_000))

	for i := range maxCooldownKeys * 2 {
		n.admit(Message{Title: "x", Key: "key-" + string(rune(i))}, time.Now())
	}

	n.mu.Lock()
	size := len(n.lastSent)
	n.mu.Unlock()

	if size > maxCooldownKeys {
		t.Fatalf("cooldown map holds %d keys, want at most %d", size, maxCooldownKeys)
	}
}

func TestDeliver_RetriesTransientFailuresThenGivesUp(t *testing.T) {
	n, transport := newTestNotifier(t, WithMaxRetries(2))

	transport.mu.Lock()
	transport.errs = []error{
		&SendError{Err: errTest},
		&SendError{Err: errTest},
		&SendError{Err: errTest},
	}
	transport.mu.Unlock()

	n.Send(Message{Title: "will fail"})

	waitFor(t, func() bool { return n.Stats().Failed == 1 })

	if transport.count() != 0 {
		t.Fatalf("transport recorded %d deliveries, want 0", transport.count())
	}
	if got := n.Stats().LastError; got == "" {
		t.Fatal("LastError is empty after a failed delivery")
	}
}

func TestDeliver_PermanentFailureStopsRetryingAndFlagsTheConfig(t *testing.T) {
	n, transport := newTestNotifier(t, WithMaxRetries(3))

	transport.mu.Lock()
	transport.errs = []error{&SendError{Permanent: true, Err: errTest}}
	transport.mu.Unlock()

	n.Send(Message{Title: "bad token"})

	waitFor(t, func() bool { return n.Stats().Failed == 1 })

	stats := n.Stats()
	if !stats.Misconfigured {
		t.Fatal("Misconfigured = false after a permanent failure")
	}
	// One attempt, not four: a bad token fails identically forever.
	if transport.count() != 0 {
		t.Fatalf("transport recorded %d deliveries, want 0", transport.count())
	}
}

// Close must deliver what is still queued. The shutdown notice is queued by
// NotifyStopped and then immediately followed by Close, so if the drain drops
// it the last thing a server ever says is lost.
func TestClose_DrainsTheQueue(t *testing.T) {
	transport := newFakeTransport()
	n := Initialize(nil,
		WithTransport(transport),
		WithEnabled(true),
		WithMinSendInterval(0),
		WithPersistence(false),
		WithLifecycleAlerts(false),
		WithEvaluateInterval(time.Hour),
	)

	for i := range 5 {
		n.Send(Message{Title: "queued", Key: string(rune('a' + i))})
	}

	if err := n.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := transport.count(); got != 5 {
		t.Fatalf("delivered %d of 5 queued messages during the drain", got)
	}
}

// A wedged transport must not hold the process open past DrainTimeout.
func TestClose_IsBoundedByTheDrainTimeout(t *testing.T) {
	transport := newFakeTransport()
	transport.block = make(chan struct{})
	defer close(transport.block)

	n := Initialize(nil,
		WithTransport(transport),
		WithEnabled(true),
		WithMinSendInterval(0),
		WithPersistence(false),
		WithLifecycleAlerts(false),
		WithEvaluateInterval(time.Hour),
		WithSendTimeout(50*time.Millisecond),
		WithDrainTimeout(200*time.Millisecond),
	)

	for i := range 10 {
		n.Send(Message{Title: "stuck", Key: string(rune('a' + i))})
	}

	start := time.Now()
	_ = n.Close()

	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("Close took %v against a wedged transport", elapsed)
	}
}

func TestDisabledNotifier_IsAWorkingNoOp(t *testing.T) {
	// The zero value stands in for "not configured yet", which is what Get
	// hands out before Initialize runs.
	var n Notifier

	n.Send(Message{Title: "ignored"})
	n.Sendf(LevelError, "also %s", "ignored")
	if err := n.Close(); err != nil {
		t.Fatalf("Close on a disabled notifier: %v", err)
	}
	if n.Enabled() {
		t.Fatal("Enabled = true on a zero-value notifier")
	}
	if got := n.Data(); got == nil {
		t.Fatal("Data returned nil")
	}
	if got := n.Stats(); got.Enabled {
		t.Fatal("Stats reports enabled on a zero-value notifier")
	}
}

func TestGet_NeverReturnsNil(t *testing.T) {
	if Get() == nil {
		t.Fatal("Get returned nil")
	}
}

func TestInitialize_WithoutCredentialsStaysDisabled(t *testing.T) {
	t.Setenv("PBEXT_TELEGRAM_BOT_TOKEN", "")
	t.Setenv("PBEXT_TELEGRAM_CHAT_ID", "")

	n := Initialize(nil)
	t.Cleanup(func() { _ = n.Close() })

	if n.Enabled() {
		t.Fatal("alerts enabled without any configuration")
	}
	if n.queue != nil {
		t.Fatal("a disabled notifier allocated a queue")
	}
	// Must still be usable.
	n.Send(Message{Title: "dropped on the floor"})
}

func TestInitialize_KillSwitchOutranksExplicitEnable(t *testing.T) {
	t.Setenv("PBEXT_ALERTS_ENABLED", "false")

	transport := newFakeTransport()
	n := Initialize(nil, WithTransport(transport), WithEnabled(true))
	t.Cleanup(func() { _ = n.Close() })

	if n.Enabled() {
		t.Fatal("PBEXT_ALERTS_ENABLED=false did not disable alerting")
	}
}

func TestConfig_RedactsTheToken(t *testing.T) {
	n := Initialize(nil, WithTelegram("123456789:AAHverySecretValue", "-100123"))
	t.Cleanup(func() { _ = n.Close() })

	if got := n.Config().BotToken; strings.Contains(got, "verySecret") {
		t.Fatalf("Config exposed the token: %q", got)
	}
}

// --- crash marker ---

func TestNotifyStarted_ReportsAnUncleanPreviousRun(t *testing.T) {
	n, transport := newTestNotifier(t, WithLifecycleAlerts(true))

	dir := t.TempDir()
	n.marker.path = filepath.Join(dir, markerFile)
	n.marker.previous = lastRun{
		PID:     4711,
		Started: time.Now().Add(-2 * time.Hour),
		Beat:    time.Now().Add(-30 * time.Minute),
		State:   stateRunning,
	}
	n.marker.hasPrevious = true

	n.NotifyStarted()

	got := waitForDelivery(t, transport)
	if got.Level != LevelCritical {
		t.Fatalf("level = %v, want critical", got.Level)
	}
	if !strings.Contains(got.Title, "unexpected exit") {
		t.Fatalf("title = %q, want it to report an unexpected exit", got.Title)
	}
	if got.Fields["previous pid"] != "4711" {
		t.Fatalf("fields = %v, want the previous pid", got.Fields)
	}
	if _, ok := got.Fields["ran for"]; !ok {
		t.Fatalf("fields = %v, want the previous run's duration", got.Fields)
	}

	// The marker now belongs to this run.
	current, ok := readMarker(n.marker.path)
	if !ok || current.State != stateRunning || current.PID != os.Getpid() {
		t.Fatalf("marker after start = %+v, want this process marked running", current)
	}
}

func TestNotifyStarted_CleanPreviousRunIsJustAStartup(t *testing.T) {
	n, transport := newTestNotifier(t, WithLifecycleAlerts(true))

	n.marker.path = filepath.Join(t.TempDir(), markerFile)
	n.marker.previous = lastRun{PID: 1, State: stateStopped}
	n.marker.hasPrevious = true

	n.NotifyStarted()

	got := waitForDelivery(t, transport)
	if got.Title != "Server started" {
		t.Fatalf("title = %q, want a plain startup notice", got.Title)
	}
}

// No marker means no information — never a crash report. A read-only data
// directory would otherwise claim a crash at every single boot.
func TestNotifyStarted_MissingMarkerReportsNoCrash(t *testing.T) {
	n, transport := newTestNotifier(t, WithLifecycleAlerts(true))
	n.marker.path = filepath.Join(t.TempDir(), markerFile)

	n.NotifyStarted()

	got := waitForDelivery(t, transport)
	if strings.Contains(got.Title, "unexpected") {
		t.Fatalf("title = %q, want no crash claim without evidence", got.Title)
	}
}

func TestNotifyStarted_CountsRestartsIntoACrashLoop(t *testing.T) {
	n, transport := newTestNotifier(t, WithLifecycleAlerts(true))

	n.marker.path = filepath.Join(t.TempDir(), markerFile)
	n.marker.previous = lastRun{
		PID:      99,
		Started:  time.Now().Add(-time.Minute),
		Beat:     time.Now().Add(-time.Second),
		State:    stateRunning,
		Restarts: restartLoopThreshold - 1,
	}
	n.marker.hasPrevious = true

	n.NotifyStarted()

	got := waitForDelivery(t, transport)
	if !strings.Contains(got.Title, "crash looping") {
		t.Fatalf("title = %q, want a crash-loop report", got.Title)
	}
}

func TestNotifyStopped_RestartIsSilentButStillMarksTheRunClean(t *testing.T) {
	n, transport := newTestNotifier(t, WithLifecycleAlerts(true))
	n.marker.path = filepath.Join(t.TempDir(), markerFile)

	n.NotifyStopped(true)

	marker, ok := readMarker(n.marker.path)
	if !ok || marker.State != stateStopped {
		t.Fatalf("marker = %+v, want a clean shutdown recorded", marker)
	}

	select {
	case m := <-transport.delivery:
		t.Fatalf("a restart sent %q; it should be silent", m.Title)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestReadMarker_TreatsCorruptionAsNoInformation(t *testing.T) {
	path := filepath.Join(t.TempDir(), markerFile)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok := readMarker(path); ok {
		t.Fatal("a corrupt marker was accepted; it must read as no information")
	}
}

func TestWriteMarker_IsAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), markerFile)
	want := lastRun{PID: 7, State: stateRunning, Started: time.Now(), Beat: time.Now()}

	if err := writeMarker(path, want); err != nil {
		t.Fatal(err)
	}

	// No temp file left behind.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("the temporary marker file was not renamed away")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got lastRun
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("marker is not valid JSON: %v", err)
	}
	if got.PID != want.PID || got.State != want.State {
		t.Fatalf("marker = %+v, want pid %d state %q", got, want.PID, want.State)
	}
}

// --- rules ---

func TestEvaluate_FiresOnTheRisingEdgeAndRecoversOnTheFalling(t *testing.T) {
	n, transport := newTestNotifier(t)

	firing := false
	if err := n.AddRule(Rule{
		Key:      "test_rule",
		Level:    LevelWarn,
		Recovery: true,
		Check: func() (bool, Message) {
			return firing, Message{Title: "condition holds"}
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Quiet while false.
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("a quiet rule sent %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	firing = true
	n.evaluate(time.Now())
	if got := waitForDelivery(t, transport); got.Title != "condition holds" {
		t.Fatalf("title = %q, want the rule's message", got.Title)
	}

	// Still true: no repeat. This is the difference between a useful channel
	// and one that sends the same line every 30 seconds.
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("a still-firing rule repeated itself: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	firing = false
	n.evaluate(time.Now())
	got := waitForDelivery(t, transport)
	if !strings.HasPrefix(got.Title, "Recovered") {
		t.Fatalf("title = %q, want a recovery notice", got.Title)
	}
}

func TestEvaluate_SustainDelaysFiring(t *testing.T) {
	n, transport := newTestNotifier(t)

	_ = n.AddRule(Rule{
		Key:     "sustained",
		Sustain: 3,
		Check:   func() (bool, Message) { return true, Message{Title: "held"} },
	})

	n.evaluate(time.Now())
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("rule fired after 2 of 3 required checks: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	n.evaluate(time.Now())
	waitForDelivery(t, transport)
}

func TestEvaluate_PanickingRuleIsDisabledNotFatal(t *testing.T) {
	n, _ := newTestNotifier(t)

	calls := 0
	_ = n.AddRule(Rule{
		Key: "broken",
		Check: func() (bool, Message) {
			calls++
			panic("rule is broken")
		},
	})

	for range maxRulePanics + 2 {
		n.evaluate(time.Now())
	}

	if calls > maxRulePanics {
		t.Fatalf("a panicking rule ran %d times, want it disabled after %d", calls, maxRulePanics)
	}
}

func TestSample_DerivesRatesFromCounterDeltas(t *testing.T) {
	metrics := Metrics{Requests: 100, ServerErrors: 10}
	n, _ := newTestNotifier(t, WithMetrics(func() Metrics { return metrics }))

	start := time.Now()
	n.sample(start)
	if n.eval.valid {
		t.Fatal("the first sample claimed to be valid; there is no interval yet")
	}

	metrics = Metrics{Requests: 200, ServerErrors: 30}
	n.sample(start.Add(10 * time.Second))

	if !n.eval.valid {
		t.Fatal("the second sample is not valid")
	}
	if got := n.eval.requestRate; got != 10 {
		t.Fatalf("requestRate = %v, want 10/s", got)
	}
	if got := n.eval.errorRate; got != 20 {
		t.Fatalf("errorRate = %v, want 20%%", got)
	}
}

// Counters only ever climb within a process; going backwards means the source
// was swapped out, and a negative delta must not become a negative rate.
func TestSample_IgnoresCountersGoingBackwards(t *testing.T) {
	metrics := Metrics{Requests: 500}
	n, _ := newTestNotifier(t, WithMetrics(func() Metrics { return metrics }))

	start := time.Now()
	n.sample(start)

	metrics = Metrics{Requests: 5}
	n.sample(start.Add(time.Second))

	if n.eval.valid {
		t.Fatal("a backwards counter produced a valid sample")
	}
}

func TestErrorRateRule_NeedsEnoughRequestsToJudge(t *testing.T) {
	metrics := Metrics{}
	n, transport := newTestNotifier(t,
		WithMetrics(func() Metrics { return metrics }),
		WithErrorRateAlert(10, 20),
	)

	start := time.Now()
	n.sample(start)

	// 2 of 3 requests failed — a 66% error rate, but far too small a sample.
	metrics = Metrics{Requests: 3, ServerErrors: 2}
	n.evaluate(start.Add(30 * time.Second))

	select {
	case m := <-transport.delivery:
		t.Fatalf("fired on a 3-request window: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	// 30 of 100 — same rate, enough traffic to mean something.
	metrics = Metrics{Requests: 103, ServerErrors: 32}
	n.evaluate(start.Add(60 * time.Second))

	if got := waitForDelivery(t, transport); !strings.Contains(got.Title, "Error rate") {
		t.Fatalf("title = %q, want an error-rate alert", got.Title)
	}
}

// Resource saturation is watched out of the box. A server that looks monitored
// and says nothing while it fills its disk is the failure this guards against.
func TestDefaultConfig_WatchesResourceSaturation(t *testing.T) {
	cfg := DefaultConfig()

	for name, got := range map[string]float64{
		"CPUPercent":       cfg.Thresholds.CPUPercent,
		"MemoryPercent":    cfg.Thresholds.MemoryPercent,
		"DiskPercent":      cfg.Thresholds.DiskPercent,
		"SwapPercent":      cfg.Thresholds.SwapPercent,
		"OpenFilesPercent": cfg.Thresholds.OpenFilesPercent,
	} {
		if got <= 0 {
			t.Errorf("%s = %v by default, want a ceiling", name, got)
		}
		if got >= 100 {
			t.Errorf("%s = %v, want a warning below saturation, not at it", name, got)
		}
	}

	// Traffic thresholds have no universal danger zone and stay opt-in.
	if cfg.Thresholds.ErrorRatePercent != 0 {
		t.Errorf("ErrorRatePercent = %v by default, want it opt-in", cfg.Thresholds.ErrorRatePercent)
	}
	if cfg.Thresholds.SurgeFactor != 0 {
		t.Errorf("SurgeFactor = %v by default, want it opt-in", cfg.Thresholds.SurgeFactor)
	}
	// A busy server legitimately runs thousands of goroutines.
	if cfg.Thresholds.Goroutines != 0 {
		t.Errorf("Goroutines = %v by default, want it opt-in", cfg.Thresholds.Goroutines)
	}
}

func TestRegisterBuiltinRules_RegistersTheResourceRulesByDefault(t *testing.T) {
	n, _ := newTestNotifier(t, WithMetrics(func() Metrics { return Metrics{} }))

	if got := n.rules.len(); got < 5 {
		t.Fatalf("registered %d rules, want at least the five resource ceilings", got)
	}
}

func TestWithoutResourceAlerts_SilencesThemAll(t *testing.T) {
	n, _ := newTestNotifier(t,
		WithMetrics(func() Metrics { return Metrics{} }),
		WithoutResourceAlerts(),
	)

	if got := n.rules.len(); got != 0 {
		t.Fatalf("registered %d rules, want none", got)
	}
}

func TestResourceRules_FireOnSaturation(t *testing.T) {
	metrics := Metrics{}
	n, transport := newTestNotifier(t,
		WithMetrics(func() Metrics { return metrics }),
		WithSustainTicks(1),
		// Isolate the disk rule so the assertion cannot pass on another alert.
		WithoutResourceAlerts(),
		WithDiskAlert(90),
	)

	metrics = Metrics{DiskPercent: 50}
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("fired at 50%% disk: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	metrics = Metrics{DiskPercent: 96}
	n.evaluate(time.Now())

	if got := waitForDelivery(t, transport); !strings.Contains(got.Title, "Disk usage") {
		t.Fatalf("title = %q, want a disk saturation alert", got.Title)
	}
}

// A host with no swap reports 0%, which is not a measurement. Alerting on it
// either way would be wrong; the rule has to know the difference.
func TestSwapRule_SkipsHostsWithNoSwap(t *testing.T) {
	metrics := Metrics{}
	n, transport := newTestNotifier(t,
		WithMetrics(func() Metrics { return metrics }),
		WithSustainTicks(1),
		WithoutResourceAlerts(),
		WithSwapAlert(80),
	)

	// Swap absent: a bogus 99% must be ignored because SwapTotal is zero.
	metrics = Metrics{SwapPercent: 99, SwapTotal: 0}
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("fired on a host with no swap: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	// Swap present and deep.
	metrics = Metrics{SwapPercent: 91, SwapTotal: 8 << 30}
	n.evaluate(time.Now())

	if got := waitForDelivery(t, transport); !strings.Contains(got.Title, "Swap") {
		t.Fatalf("title = %q, want a swap alert", got.Title)
	}
}

// The raw descriptor count means nothing without the ceiling: 512 open files is
// either half a step from an outage or unremarkable depending on the host.
func TestOpenFilesRule_SkipsAnUnknownLimit(t *testing.T) {
	metrics := Metrics{}
	n, transport := newTestNotifier(t,
		WithMetrics(func() Metrics { return metrics }),
		WithSustainTicks(1),
		WithoutResourceAlerts(),
		WithFileDescriptorAlert(80),
	)

	// Limit unknown (Windows, or a failed lookup): no ratio to judge.
	metrics = Metrics{OpenFilesPercent: 99, OpenFiles: 990, OpenFilesLimit: 0}
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("fired with an unknown descriptor limit: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	metrics = Metrics{OpenFilesPercent: 92, OpenFiles: 942, OpenFilesLimit: 1024}
	n.evaluate(time.Now())

	if got := waitForDelivery(t, transport); !strings.Contains(got.Title, "Open file descriptors") {
		t.Fatalf("title = %q, want a descriptor alert", got.Title)
	}
}

// Sustain is what keeps a batch job pinning the cores for one tick from paging
// anyone.
func TestResourceRules_RequireTheThresholdToHold(t *testing.T) {
	metrics := Metrics{CPUPercent: 99}
	n, transport := newTestNotifier(t,
		WithMetrics(func() Metrics { return metrics }),
		WithoutResourceAlerts(),
		WithCPUAlert(90),
		WithSustainTicks(3),
	)

	n.evaluate(time.Now())
	n.evaluate(time.Now())
	select {
	case m := <-transport.delivery:
		t.Fatalf("fired after 2 of 3 required checks: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}

	n.evaluate(time.Now())
	waitForDelivery(t, transport)
}

func TestTrafficSurgeRule_RespectsTheFloor(t *testing.T) {
	metrics := Metrics{}
	n, transport := newTestNotifier(t,
		WithMetrics(func() Metrics { return metrics }),
		WithTrafficSurgeAlert(2, 50),
	)

	start := time.Now()
	n.sample(start)

	// Traffic doubles, from 1/s to 2/s. Nobody should be woken for this.
	metrics = Metrics{Requests: 10}
	n.evaluate(start.Add(10 * time.Second))
	metrics = Metrics{Requests: 30}
	n.evaluate(start.Add(20 * time.Second))

	select {
	case m := <-transport.delivery:
		t.Fatalf("fired below the floor: %q", m.Title)
	case <-time.After(100 * time.Millisecond):
	}
}

// --- helpers ---

var errTest = &testError{"transport unavailable"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func waitFor(t testing.TB, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not met within the timeout")
}
