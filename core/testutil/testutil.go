// Package testutil provides shared test helpers for pb-ext packages.
// It wraps PocketBase's test infrastructure with pb-ext collection setup.
package testutil

import (
	"testing"

	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/pocketbase/pocketbase/tests"
)

// NewTestApp creates a bare TestApp with only PocketBase system migrations applied.
// Use this for tests that don't need pb-ext collections.
func NewTestApp(t testing.TB) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Cleanup() })
	return app
}

// NewTestAppWithJobs creates a TestApp with the _job_logs collection set up.
func NewTestAppWithJobs(t testing.TB) *tests.TestApp {
	t.Helper()
	app := NewTestApp(t)
	if err := jobs.SetupCollection(app); err != nil {
		t.Fatalf("jobs.SetupCollection: %v", err)
	}
	return app
}

// NewTestAppWithAnalytics creates a TestApp with the _analytics collection set up.
func NewTestAppWithAnalytics(t testing.TB) *tests.TestApp {
	t.Helper()
	app := NewTestApp(t)
	if err := analytics.SetupCollection(app); err != nil {
		t.Fatalf("analytics.SetupCollection: %v", err)
	}
	return app
}

// NewTestAppFull creates a TestApp with all pb-ext collections set up.
func NewTestAppFull(t testing.TB) *tests.TestApp {
	t.Helper()
	app := NewTestApp(t)
	if err := jobs.SetupCollection(app); err != nil {
		t.Fatalf("jobs.SetupCollection: %v", err)
	}
	if err := analytics.SetupCollection(app); err != nil {
		t.Fatalf("analytics.SetupCollection: %v", err)
	}
	return app
}
