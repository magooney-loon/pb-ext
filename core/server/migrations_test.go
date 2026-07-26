package server

import (
	"testing"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// This file is in package server (not server_test) so it can call
// applyOwnMigrations directly. It cannot use core/testutil, which imports
// core/analytics and core/jobs and would create an import cycle.

func newApp(t *testing.T) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Cleanup() })
	return app
}

// pbExtSchemaExists reports whether each of pb-ext's schema objects is present.
// They no longer live in the same database: _job_logs is a data.db collection,
// while _analytics and _alerts are plain tables in auxiliary.db.
func pbExtSchemaExists(app core.App) map[string]bool {
	_, jobsErr := app.FindCollectionByNameOrId(jobs.Collection)

	return map[string]bool{
		jobs.Collection:     jobsErr == nil,
		analytics.TableName: app.AuxHasTable(analytics.TableName),
		alerts.TableName:    app.AuxHasTable(alerts.TableName),
	}
}

// dropOwnSchema reverts pb-ext's migrations, simulating a database that has not
// seen them yet. The nesting mirrors MigrationsRunner, whose Up/Down wrap an
// auxiliary transaction around the data one so a migration can touch both.
func dropOwnSchema(t *testing.T, app core.App) {
	t.Helper()
	for _, m := range ownMigrations() {
		err := app.AuxRunInTransaction(func(txApp core.App) error {
			return txApp.RunInTransaction(m.Down)
		})
		if err != nil {
			t.Fatalf("revert %s: %v", m.File, err)
		}
		if _, err := app.NonconcurrentDB().NewQuery("DELETE FROM _migrations WHERE file = {:f}").
			Bind(dbx.Params{"f": m.File}).Execute(); err != nil {
			t.Fatalf("clear migration history for %s: %v", m.File, err)
		}
	}
}

// TestApplyOwnMigrations_CreatesSchemaAtBootstrap is the regression test for
// the ordering trap: PocketBase runs core.AppMigrations at the start of
// apis.Serve, which is *after* OnBootstrap. The job manager is initialized
// during OnBootstrap because user code expects GetManager() to work from its
// own OnServe hooks, so pb-ext must apply its own migrations early.
func TestApplyOwnMigrations_CreatesSchemaAtBootstrap(t *testing.T) {
	app := newApp(t)
	dropOwnSchema(t, app)

	// Precondition: the schema really is absent.
	for name, exists := range pbExtSchemaExists(app) {
		if exists {
			t.Fatalf("precondition failed: %s still exists", name)
		}
	}

	if err := applyOwnMigrations(app); err != nil {
		t.Fatalf("applyOwnMigrations: %v", err)
	}

	for name, exists := range pbExtSchemaExists(app) {
		if !exists {
			t.Errorf("%s missing after applyOwnMigrations", name)
		}
	}
}

// TestApplyOwnMigrations_RecreatesDeletedAuxTable covers the split-database
// hazard: _migrations lives in data.db but _analytics lives in auxiliary.db, so
// deleting auxiliary.db leaves history claiming the migration is applied. The
// migration's ReapplyCondition is what brings the table back.
func TestApplyOwnMigrations_RecreatesDeletedAuxTable(t *testing.T) {
	app := newApp(t)

	if err := applyOwnMigrations(app); err != nil {
		t.Fatalf("applyOwnMigrations: %v", err)
	}

	// Drop the tables only — the _migrations rows stay behind, exactly as they
	// would after someone removed auxiliary.db.
	for _, table := range []string{analytics.TableName, alerts.TableName} {
		if _, err := app.AuxDB().NewQuery("DROP TABLE " + table).Execute(); err != nil {
			t.Fatalf("drop aux table %s: %v", table, err)
		}
	}

	if err := applyOwnMigrations(app); err != nil {
		t.Fatalf("applyOwnMigrations after aux table loss: %v", err)
	}
	for _, table := range []string{analytics.TableName, alerts.TableName} {
		if !app.AuxHasTable(table) {
			t.Errorf("%s was not recreated; ReapplyCondition did not fire", table)
		}
	}
}

// TestApplyOwnMigrations_Idempotent covers the double-run that happens on every
// real startup: once from OnBootstrap, once from apis.Serve's RunAllMigrations.
func TestApplyOwnMigrations_Idempotent(t *testing.T) {
	app := newApp(t)
	dropOwnSchema(t, app)

	for i := 0; i < 3; i++ {
		if err := applyOwnMigrations(app); err != nil {
			t.Fatalf("applyOwnMigrations run %d: %v", i+1, err)
		}
	}

	// A second application would have created duplicate history rows.
	for _, m := range ownMigrations() {
		var count int
		err := app.DB().NewQuery("SELECT COUNT(*) FROM _migrations WHERE file = {:f}").
			Bind(dbx.Params{"f": m.File}).Row(&count)
		if err != nil {
			t.Fatalf("query history for %s: %v", m.File, err)
		}
		if count != 1 {
			t.Errorf("migration %s recorded %d times, want 1", m.File, count)
		}
	}
}

// TestApplyOwnMigrations_SatisfiesRunAllMigrations verifies the early run makes
// PocketBase's own pass a no-op rather than a duplicate-create failure.
func TestApplyOwnMigrations_SatisfiesRunAllMigrations(t *testing.T) {
	app := newApp(t)
	dropOwnSchema(t, app)

	if err := applyOwnMigrations(app); err != nil {
		t.Fatalf("applyOwnMigrations: %v", err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatalf("RunAllMigrations after early apply: %v", err)
	}
}

// TestOwnMigrations_AreRegisteredGlobally ensures an app embedding pb-ext
// without Server.Start still gets the schema through PocketBase's normal run.
func TestOwnMigrations_AreRegisteredGlobally(t *testing.T) {
	registered := map[string]bool{}
	for _, m := range core.AppMigrations.Items() {
		registered[m.File] = true
	}

	for _, m := range ownMigrations() {
		if !registered[m.File] {
			t.Errorf("migration %s is not registered in core.AppMigrations", m.File)
		}
	}
}

// TestOwnMigrations_HaveDistinctNames catches a copy-paste collision between
// pb-ext's own migration file names, which would silently skip one of them.
func TestOwnMigrations_HaveDistinctNames(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range ownMigrations() {
		if seen[m.File] {
			t.Errorf("duplicate migration file name %q", m.File)
		}
		seen[m.File] = true

		if m.Up == nil || m.Down == nil {
			t.Errorf("migration %q must define both Up and Down", m.File)
		}
	}
}
