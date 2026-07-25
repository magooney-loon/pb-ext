package analytics

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// MigrationFile is the identity recorded in PocketBase's _migrations table for
// the _analytics schema.
//
// It is passed to Register explicitly instead of being derived from
// runtime.Caller, because PocketBase keys applied migrations on the base file
// name alone — an app migration that happened to be called "migrations.go"
// would otherwise be silently treated as already applied. The timestamp prefix
// keeps pb-ext's migrations sorted ahead of anything an app generates later,
// and the pbext_ segment keeps the name from colliding with one.
const MigrationFile = "1780000001_pbext_analytics.go"

var migration = &core.Migration{
	File: MigrationFile,
	Up:   migrateUp,
	Down: migrateDown,
}

// Registering from init means importing this package is enough for the schema
// to exist: apis.Serve runs core.AppMigrations before the router is built, and
// tests.NewTestApp runs them too. pb-ext's migrations share core.AppMigrations
// with the app's own — that is the supported extension point (PocketBase's jsvm
// plugin does the same) and ordering is by file name, so the timestamp above
// guarantees this runs first.
func init() {
	core.AppMigrations.Add(migration)
}

// Migration returns the registered _analytics migration.
//
// pb-ext applies its own migrations during OnBootstrap, because parts of the
// server (the job manager) must be usable before apis.Serve reaches
// RunAllMigrations. Applying them early records them in _migrations, so the
// later run is a no-op.
func Migration() *core.Migration {
	return migration
}

func migrateUp(txApp core.App) error {
	if err := txApp.SaveNoValidate(newCounterCollection()); err != nil {
		return fmt.Errorf("create %s collection: %w", CollectionName, err)
	}
	return nil
}

func migrateDown(txApp core.App) error {
	col, err := txApp.FindCollectionByNameOrId(CollectionName)
	if err != nil {
		return nil // already absent
	}

	// System collections are protected from deletion by a built-in hook.
	col.System = false
	if err := txApp.SaveNoValidate(col); err != nil {
		return fmt.Errorf("unmark %s as system: %w", CollectionName, err)
	}
	if err := txApp.Delete(col); err != nil {
		return fmt.Errorf("delete %s collection: %w", CollectionName, err)
	}
	return nil
}
