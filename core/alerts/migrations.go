package alerts

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// MigrationFile is the identity recorded in PocketBase's _migrations table for
// the _alerts schema.
//
// It is passed to the migration explicitly instead of being derived from
// runtime.Caller, because PocketBase keys applied migrations on the base file
// name alone — an app migration that happened to be called "migrations.go"
// would otherwise be silently treated as already applied. The timestamp prefix
// keeps pb-ext's migrations sorted ahead of anything an app generates later,
// and the pbext_ segment keeps the name from colliding with one.
const MigrationFile = "1780000003_pbext_alerts.go"

var migration = &core.Migration{
	File: MigrationFile,
	Up:   migrateUp,
	Down: migrateDown,

	// The _migrations table lives in data.db (PocketBase's runner creates it
	// with app.DB()), but this migration's table lives in auxiliary.db. Deleting
	// auxiliary.db — a reasonable thing to do, it holds only logs and counters —
	// would otherwise leave the history saying "applied" with no table to show
	// for it. PocketBase's own _logs migration does the same.
	ReapplyCondition: func(txApp core.App, runner *core.MigrationsRunner, fileName string) (bool, error) {
		return !txApp.AuxHasTable(TableName), nil
	},
}

// Registering from init means importing this package is enough for the schema
// to exist: apis.Serve runs core.AppMigrations before the router is built, and
// tests.NewTestApp runs them too.
func init() {
	core.AppMigrations.Add(migration)
}

// Migration returns the registered _alerts migration.
//
// pb-ext applies its own migrations during OnBootstrap, because parts of the
// server must be usable before apis.Serve reaches RunAllMigrations. Applying
// them early records them in _migrations, so the later run is a no-op.
func Migration() *core.Migration {
	return migration
}

func migrateUp(txApp core.App) error {
	if _, err := txApp.AuxDB().NewQuery(createTableSQL).Execute(); err != nil {
		return fmt.Errorf("create %s table in auxiliary.db: %w", TableName, err)
	}
	return nil
}

func migrateDown(txApp core.App) error {
	if _, err := txApp.AuxDB().NewQuery(dropTableSQL).Execute(); err != nil {
		return fmt.Errorf("drop %s table from auxiliary.db: %w", TableName, err)
	}
	return nil
}
