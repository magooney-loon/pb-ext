package analytics

import (
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// SetupCollections creates the _analytics system collection and removes
// superseded schemas left by older pb-ext versions.
func SetupCollections(app core.App) error {
	if err := setupCounterCollection(app); err != nil {
		return err
	}
	return dropLegacySessionsCollection(app)
}

// setupCounterCollection creates _analytics — one row per
// (path, date, device_type, browser) — and migrates older schemas onto it.
func setupCounterCollection(app core.App) error {
	if col, err := app.FindCollectionByNameOrId(CollectionName); err == nil {
		if col.Fields.GetByName("ip") != nil {
			// Original raw-events schema — incompatible with the aggregated
			// design, and it holds PII that should not be retained anyway.
			app.Logger().Warn("Detected old _analytics raw-events schema — migrating to aggregated schema (old data dropped)")
			if err := deleteSystemCollection(app, col); err != nil {
				return fmt.Errorf("migration: drop old _analytics: %w", err)
			}
			// Fall through to create the new schema below.
		} else {
			changed := false

			if col.Fields.GetByName("returning_sessions") == nil {
				// Aggregated schema predating returning-visitor tracking —
				// additive, so existing counts are preserved.
				app.Logger().Warn("Migrating _analytics: adding returning_sessions field")
				col.Fields.Add(&core.NumberField{Name: "returning_sessions", Required: false})
				changed = true
			}
			if ensureIndexes(col) {
				app.Logger().Warn("Migrating _analytics: adding dashboard covering indexes")
				changed = true
			}

			if changed {
				if err := app.SaveNoValidate(col); err != nil {
					return fmt.Errorf("migration: update _analytics schema: %w", err)
				}
				app.Logger().Info("Migrated _analytics collection")
			} else {
				app.Logger().Debug("_analytics collection already exists (current schema)")
			}
			return nil
		}
	}

	app.Logger().Debug("Creating _analytics collection")

	col := core.NewBaseCollection(CollectionName)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "path", Required: true})
	col.Fields.Add(&core.TextField{Name: "date", Required: true})
	col.Fields.Add(&core.TextField{Name: "device_type", Required: false})
	col.Fields.Add(&core.TextField{Name: "browser", Required: false})
	col.Fields.Add(&core.NumberField{Name: "views", Required: true})
	col.Fields.Add(&core.NumberField{Name: "unique_sessions", Required: true})
	col.Fields.Add(&core.NumberField{Name: "returning_sessions", Required: false})
	col.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	ensureIndexes(col)

	if err := app.SaveNoValidate(col); err != nil {
		app.Logger().Error("Failed to create _analytics collection", "error", err)
		return err
	}

	app.Logger().Info("Created _analytics collection")
	return nil
}

// counterIndexes are the indexes _analytics needs.
//
// Beyond the unique key required for upsert correctness, each dashboard
// aggregate gets a covering index whose leading column is its GROUP BY target
// and which carries every column the query reads. That lets SQLite answer them
// from the index alone — no per-row table lookups, and no temp B-tree for the
// grouping. Writes now happen once per flush rather than once per request, so
// paying for the extra index maintenance is a good trade.
var counterIndexes = []struct {
	name    string
	unique  bool
	columns string
}{
	{"idx_analytics_upsert", true, "path, date, device_type, browser"},
	{"idx_analytics_totals", false, "date, views, unique_sessions, returning_sessions"},
	{"idx_analytics_pages", false, "path, date, views"},
	{"idx_analytics_devices", false, "device_type, date, views"},
	{"idx_analytics_browsers", false, "browser, date, views"},
}

// ensureIndexes adds any missing index to col, reporting whether it changed.
func ensureIndexes(col *core.Collection) bool {
	changed := false

	for _, idx := range counterIndexes {
		if hasIndex(col, idx.name) {
			continue
		}
		col.AddIndex(idx.name, idx.unique, idx.columns, "")
		changed = true
	}

	// Superseded by idx_analytics_totals, which leads with the same column.
	for i, existing := range col.Indexes {
		if strings.Contains(existing, "idx_analytics_date") {
			col.Indexes = append(col.Indexes[:i], col.Indexes[i+1:]...)
			changed = true
			break
		}
	}

	return changed
}

func hasIndex(col *core.Collection, name string) bool {
	for _, existing := range col.Indexes {
		if strings.Contains(existing, name) {
			return true
		}
	}
	return false
}

// dropLegacySessionsCollection removes _analytics_sessions. Recent visits are
// now held in an in-memory ring, which avoids an insert plus a prune query on
// every request.
func dropLegacySessionsCollection(app core.App) error {
	col, err := app.FindCollectionByNameOrId(LegacySessionsCollectionName)
	if err != nil {
		return nil // never existed, or already removed
	}

	app.Logger().Warn("Dropping _analytics_sessions — recent visits are now tracked in memory")
	if err := deleteSystemCollection(app, col); err != nil {
		return fmt.Errorf("migration: drop %s: %w", LegacySessionsCollectionName, err)
	}

	app.Logger().Info("Dropped _analytics_sessions collection")
	return nil
}

// deleteSystemCollection clears the system flag before deleting, since system
// collections are protected from deletion by a built-in PocketBase hook.
func deleteSystemCollection(app core.App, col *core.Collection) error {
	if col.System {
		col.System = false
		if err := app.SaveNoValidate(col); err != nil {
			return fmt.Errorf("unmark as system: %w", err)
		}
	}
	if err := app.Delete(col); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// SetupCollection is kept for backward compatibility with testutil.
// New code should call SetupCollections.
func SetupCollection(app core.App) error {
	return SetupCollections(app)
}
