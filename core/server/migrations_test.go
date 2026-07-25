package server

import (
	"testing"

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

// dropOwnCollections reverts pb-ext's migrations, simulating a database that
// has not seen them yet.
func dropOwnCollections(t *testing.T, app core.App) {
	t.Helper()
	for _, m := range ownMigrations() {
		if err := app.RunInTransaction(m.Down); err != nil {
			t.Fatalf("revert %s: %v", m.File, err)
		}
		_, err := app.NonconcurrentDB().NewQuery("DELETE FROM _migrations WHERE file = {:f}").
			Bind(dbx.Params{"f": m.File}).Execute()
		if err != nil {
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
	dropOwnCollections(t, app)

	// Precondition: the schema really is absent.
	for _, name := range []string{jobs.Collection, analytics.CollectionName} {
		if _, err := app.FindCollectionByNameOrId(name); err == nil {
			t.Fatalf("precondition failed: %s still exists", name)
		}
	}

	if err := applyOwnMigrations(app); err != nil {
		t.Fatalf("applyOwnMigrations: %v", err)
	}

	for _, name := range []string{jobs.Collection, analytics.CollectionName} {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Errorf("%s missing after applyOwnMigrations: %v", name, err)
		}
	}
}

// TestApplyOwnMigrations_Idempotent covers the double-run that happens on every
// real startup: once from OnBootstrap, once from apis.Serve's RunAllMigrations.
func TestApplyOwnMigrations_Idempotent(t *testing.T) {
	app := newApp(t)
	dropOwnCollections(t, app)

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
	dropOwnCollections(t, app)

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
