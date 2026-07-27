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
//
// The timestamp was bumped past the original 1780000001 when _analytics moved
// from a data.db collection to an auxiliary.db table, so installs carrying the
// old history entry still run this one.
const MigrationFile = "1780000002_pbext_analytics.go"

// legacyCollectionName is the data.db collection _analytics used to be, kept
// only so migrateUp can clean it up. Nothing reads it.
const legacyCollectionName = "_analytics"

var migration = &core.Migration{
	File: MigrationFile,
	Up:   migrateUp,
	Down: migrateDown,

	// The _migrations table lives in data.db (PocketBase's runner creates it
	// with app.DB()), but this migration's table lives in auxiliary.db. Deleting
	// auxiliary.db — which is a reasonable thing to do, it holds only logs and
	// counters — would otherwise leave the history saying "applied" with no
	// table to show for it. PocketBase's own _logs migration does the same.
	ReapplyCondition: func(txApp core.App, runner *core.MigrationsRunner, fileName string) (bool, error) {
		return !txApp.AuxHasTable(TableName), nil
	},
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
	if _, err := txApp.AuxDB().NewQuery(createTableSQL).Execute(); err != nil {
		return fmt.Errorf("create %s table in auxiliary.db: %w", TableName, err)
	}
	return dropLegacyCollection(txApp)
}

func migrateDown(txApp core.App) error {
	if _, err := txApp.AuxDB().NewQuery(dropTableSQL).Execute(); err != nil {
		return fmt.Errorf("drop %s table from auxiliary.db: %w", TableName, err)
	}
	return nil
}

// dropLegacyCollection removes the data.db _analytics collection left behind by
// pb-ext versions that stored counters there. Historical counters are not
// carried over — they are aggregate page views with a 90-day retention, not
// something worth a cross-database copy — so this only reclaims the space and
// keeps a dead system collection out of the Admin UI.
func dropLegacyCollection(txApp core.App) error {
	col, err := txApp.FindCollectionByNameOrId(legacyCollectionName)
	if err != nil {
		return nil // fresh install, or already cleaned up
	}

	// System collections are protected from deletion by a built-in hook.
	col.System = false
	if err := txApp.SaveNoValidate(col); err != nil {
		return fmt.Errorf("unmark legacy %s collection as system: %w", legacyCollectionName, err)
	}
	if err := txApp.Delete(col); err != nil {
		return fmt.Errorf("delete legacy %s collection: %w", legacyCollectionName, err)
	}
	return nil
}
