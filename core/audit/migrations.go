package audit

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// MigrationFile is the identity recorded in PocketBase's _migrations table for
// the _admin_access schema.
//
// It is passed to the migration explicitly instead of being derived from
// runtime.Caller, because PocketBase keys applied migrations on the base file
// name alone — an app migration that happened to be called "migrations.go"
// would otherwise be silently treated as already applied. The timestamp prefix
// keeps pb-ext's migrations sorted ahead of anything an app generates later,
// and the pbext_ segment keeps the name from colliding with one.
const MigrationFile = "1780000004_pbext_audit.go"

var migration = &core.Migration{
	File: MigrationFile,
	Up:   migrateUp,
	Down: migrateDown,

	// The _migrations table lives in data.db but this table lives in
	// auxiliary.db, so deleting auxiliary.db would otherwise leave history
	// claiming the migration is applied with no table to show for it. Same
	// condition as _analytics and _alerts, and as PocketBase's own _logs.
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

// Migration returns the registered _admin_access migration.
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
