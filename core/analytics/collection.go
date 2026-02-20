package analytics

import "github.com/pocketbase/pocketbase/core"

// SetupCollections creates both pb-ext analytics system collections if they don't exist.
func SetupCollections(app core.App) error {
	if err := setupCounterCollection(app); err != nil {
		return err
	}
	return setupSessionsCollection(app)
}

// setupCounterCollection creates _analytics — one row per (path, date, device, browser).
func setupCounterCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(CollectionName); err == nil {
		app.Logger().Debug("_analytics collection already exists")
		return nil
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
	col.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	// Unique index enforces one row per combination — required for upsert correctness.
	col.AddIndex("idx_analytics_upsert", true, "path, date, device_type, browser", "")
	col.AddIndex("idx_analytics_date", false, "date", "")

	if err := app.SaveNoValidate(col); err != nil {
		app.Logger().Error("Failed to create _analytics collection", "error", err)
		return err
	}

	app.Logger().Info("Created _analytics collection")
	return nil
}

// setupSessionsCollection creates _analytics_sessions — ring buffer of recent visits.
func setupSessionsCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(SessionsCollectionName); err == nil {
		app.Logger().Debug("_analytics_sessions collection already exists")
		return nil
	}

	app.Logger().Debug("Creating _analytics_sessions collection")

	col := core.NewBaseCollection(SessionsCollectionName)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "path", Required: true})
	col.Fields.Add(&core.TextField{Name: "device_type", Required: false})
	col.Fields.Add(&core.TextField{Name: "browser", Required: false})
	col.Fields.Add(&core.TextField{Name: "os", Required: false})
	col.Fields.Add(&core.DateField{Name: "timestamp", Required: true})
	col.Fields.Add(&core.BoolField{Name: "is_new_session", Required: false})
	col.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})

	col.AddIndex("idx_analytics_sessions_ts", false, "timestamp", "")

	if err := app.SaveNoValidate(col); err != nil {
		app.Logger().Error("Failed to create _analytics_sessions collection", "error", err)
		return err
	}

	app.Logger().Info("Created _analytics_sessions collection")
	return nil
}

// SetupCollection is kept for backward compatibility with testutil.
// New code should call SetupCollections.
func SetupCollection(app core.App) error {
	return SetupCollections(app)
}
