// Package testutil provides shared test helpers for pb-ext packages.
// It wraps PocketBase's test infrastructure with pb-ext collection setup.
package testutil

import (
	"testing"

	"github.com/magooney-loon/pb-ext/core/analytics"

	// pb-ext schemas are created by migrations registered from these packages'
	// init functions, which tests.NewTestApp applies via RunAllMigrations.
	_ "github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// NewTestApp creates a TestApp with PocketBase's system migrations and pb-ext's
// registered migrations applied, so _analytics and _job_logs both exist.
func NewTestApp(t testing.TB) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Cleanup() })
	return app
}

// NewAnalytics creates a TestApp with the _analytics collection plus a running
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

// AnalyticsTotals returns the number of persisted _analytics rows along with the
// summed view and session counters.
func AnalyticsTotals(t testing.TB, app core.App) (rows, views, newSessions, returningSessions int) {
	t.Helper()

	err := app.DB().
		Select(
			"COUNT(*)",
			"COALESCE(SUM(views),0)",
			"COALESCE(SUM(unique_sessions),0)",
			"COALESCE(SUM(returning_sessions),0)",
		).
		From(analytics.CollectionName).
		Row(&rows, &views, &newSessions, &returningSessions)
	if err != nil {
		t.Fatalf("count analytics rows: %v", err)
	}

	return rows, views, newSessions, returningSessions
}
