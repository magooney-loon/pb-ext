package jobs

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// MigrationFile is the identity recorded in PocketBase's _migrations table for
// the _job_logs schema. See analytics.MigrationFile for why the name is passed
// explicitly rather than derived from runtime.Caller.
const MigrationFile = "1780000000_pbext_jobs.go"

var migration = &core.Migration{
	File: MigrationFile,
	Up:   migrateUp,
	Down: migrateDown,
}

func init() {
	core.AppMigrations.Add(migration)
}

// Migration returns the registered _job_logs migration.
// See analytics.Migration for why pb-ext applies these during OnBootstrap.
func Migration() *core.Migration {
	return migration
}

func migrateUp(txApp core.App) error {
	if err := txApp.SaveNoValidate(newLogCollection()); err != nil {
		return fmt.Errorf("create %s collection: %w", Collection, err)
	}
	return nil
}

func migrateDown(txApp core.App) error {
	col, err := txApp.FindCollectionByNameOrId(Collection)
	if err != nil {
		return nil // already absent
	}

	// System collections are protected from deletion by a built-in hook.
	col.System = false
	if err := txApp.SaveNoValidate(col); err != nil {
		return fmt.Errorf("unmark %s as system: %w", Collection, err)
	}
	if err := txApp.Delete(col); err != nil {
		return fmt.Errorf("delete %s collection: %w", Collection, err)
	}
	return nil
}
