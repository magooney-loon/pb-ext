// Package testutil provides shared test helpers for pb-ext packages.
// It wraps PocketBase's test infrastructure with pb-ext schema setup.
package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/analytics"

	// pb-ext schemas are created by migrations registered from these packages'
	// init functions, which tests.NewTestApp applies via RunAllMigrations.
	_ "github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// NewTestApp creates a TestApp with PocketBase's system migrations and pb-ext's
// registered migrations applied, so the _analytics and _alerts tables
// (auxiliary.db) and the _job_logs collection (data.db) all exist.
func NewTestApp(t testing.TB) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Cleanup() })
	return app
}

// NewAnalytics creates a TestApp with the _analytics table plus a running
// Analytics collector. The collector is closed (and its pending counters
// flushed) during test cleanup.
//
// The background flush worker is started, so pass analytics.WithFlushInterval to
// control its cadence; tests that want to observe buffering should set a long
// interval and call Flush explicitly.
func NewAnalytics(t testing.TB, opts ...analytics.Option) (*tests.TestApp, *analytics.Analytics) {
	t.Helper()
	app := NewTestApp(t)

	a, err := analytics.Initialize(app, opts...)
	if err != nil {
		t.Fatalf("analytics.Initialize: %v", err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Logf("analytics.Close: %v", err)
		}
	})

	return app, a
}

// AlertCapture is an alerts.Transport that records what it was asked to
// deliver, so tests can assert on alerts without a network or a bot token.
type AlertCapture struct {
	mu       sync.Mutex
	messages []alerts.Message
	delivery chan alerts.Message
}

// NewAlertCapture creates a capturing transport.
func NewAlertCapture() *AlertCapture {
	return &AlertCapture{delivery: make(chan alerts.Message, 64)}
}

func (c *AlertCapture) Name() string                     { return "capture" }
func (c *AlertCapture) Target() string                   { return "test capture" }
func (c *AlertCapture) Verify(ctx context.Context) error { return nil }

// Send records the message and never fails.
func (c *AlertCapture) Send(ctx context.Context, m alerts.Message, instance string) error {
	c.mu.Lock()
	c.messages = append(c.messages, m)
	c.mu.Unlock()

	select {
	case c.delivery <- m:
	default:
	}
	return nil
}

// Messages returns everything delivered so far.
func (c *AlertCapture) Messages() []alerts.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]alerts.Message(nil), c.messages...)
}

// Wait blocks for the next delivery, failing the test if none arrives.
// Delivery is asynchronous — a background worker owns every send — so tests
// must wait rather than assert immediately after Send returns.
func (c *AlertCapture) Wait(t testing.TB) alerts.Message {
	t.Helper()
	select {
	case m := <-c.delivery:
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an alert")
		return alerts.Message{}
	}
}

// NewAlerts creates a TestApp plus a running Notifier wired to a capturing
// transport. The notifier is closed during cleanup.
//
// Lifecycle notices are off by default so a test only sees the alerts it
// provokes; pass alerts.WithLifecycleAlerts(true) to exercise them.
func NewAlerts(t testing.TB, opts ...alerts.Option) (*tests.TestApp, *alerts.Notifier, *AlertCapture) {
	t.Helper()

	app := NewTestApp(t)
	capture := NewAlertCapture()

	defaults := []alerts.Option{
		alerts.WithTransport(capture),
		alerts.WithEnabled(true),
		alerts.WithMinSendInterval(0),
		alerts.WithLifecycleAlerts(false),
		alerts.WithEvaluateInterval(time.Hour),
	}

	n := alerts.Initialize(app, append(defaults, opts...)...)
	t.Cleanup(func() { _ = n.Close() })

	return app, n, capture
}

// AnalyticsTotals returns the number of persisted _analytics rows along with the
// summed view and session counters. The table lives in auxiliary.db.
func AnalyticsTotals(t testing.TB, app core.App) (rows, views, newSessions, returningSessions int) {
	t.Helper()

	err := app.AuxDB().
		Select(
			"COUNT(*)",
			"COALESCE(SUM(views),0)",
			"COALESCE(SUM(unique_sessions),0)",
			"COALESCE(SUM(returning_sessions),0)",
		).
		From(analytics.TableName).
		Row(&rows, &views, &newSessions, &returningSessions)
	if err != nil {
		t.Fatalf("count analytics rows: %v", err)
	}

	return rows, views, newSessions, returningSessions
}
