package server

import (
	"fmt"

	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/pocketbase/pocketbase/core"
)

// ownMigrations returns pb-ext's migrations, in the order they should be applied.
//
// They are also registered in core.AppMigrations from each package's init, so
// an app that embeds pb-ext without using Server.Start still gets them via
// PocketBase's normal RunAllMigrations.
func ownMigrations() []*core.Migration {
	return []*core.Migration{
		jobs.Migration(),
		analytics.Migration(),
	}
}

// applyOwnMigrations applies pb-ext's schema migrations immediately, without
// touching the app's own.
//
// PocketBase runs core.AppMigrations at the start of apis.Serve, which is after
// OnBootstrap — too late for the job manager, which user code expects to be
// available from its own OnServe hooks. Running just pb-ext's migrations here
// records them in the _migrations table under the same file names, so the later
// RunAllMigrations pass skips them.
func applyOwnMigrations(app core.App) error {
	list := core.MigrationsList{}
	for _, m := range ownMigrations() {
		list.Add(m)
	}

	applied, err := core.NewMigrationsRunner(app, list).Up()
	if err != nil {
		return fmt.Errorf("apply pb-ext migrations: %w", err)
	}

	for _, file := range applied {
		app.Logger().Info("Applied pb-ext migration", "file", file)
	}

	return nil
}
