package analytics

import "github.com/pocketbase/pocketbase/core"

// SetupCollection creates the _analytics system collection if it doesn't exist.
func SetupCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(CollectionName); err == nil {
		app.Logger().Debug("_analytics collection already exists")
		return nil
	}

	app.Logger().Debug("Creating _analytics collection")

	col := core.NewBaseCollection(CollectionName)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "path", Required: true})
	col.Fields.Add(&core.TextField{Name: "method", Required: true})
	col.Fields.Add(&core.TextField{Name: "ip", Required: true})
	col.Fields.Add(&core.TextField{Name: "user_agent", Required: false})
	col.Fields.Add(&core.TextField{Name: "referrer", Required: false})
	col.Fields.Add(&core.NumberField{Name: "duration_ms", Required: true})
	col.Fields.Add(&core.DateField{Name: "timestamp", Required: true})
	col.Fields.Add(&core.TextField{Name: "visitor_id", Required: false})
	col.Fields.Add(&core.TextField{Name: "device_type", Required: false})
	col.Fields.Add(&core.TextField{Name: "browser", Required: false})
	col.Fields.Add(&core.TextField{Name: "os", Required: false})
	col.Fields.Add(&core.TextField{Name: "country", Required: false})
	col.Fields.Add(&core.TextField{Name: "utm_source", Required: false})
	col.Fields.Add(&core.TextField{Name: "utm_medium", Required: false})
	col.Fields.Add(&core.TextField{Name: "utm_campaign", Required: false})
	col.Fields.Add(&core.BoolField{Name: "is_new_visit", Required: false})
	col.Fields.Add(&core.TextField{Name: "query_params", Required: false})
	col.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	col.AddIndex("idx_analytics_timestamp", false, "timestamp", "")
	col.AddIndex("idx_analytics_path", false, "path", "")
	col.AddIndex("idx_analytics_ip", false, "ip", "")
	col.AddIndex("idx_analytics_visitor_id", false, "visitor_id", "")
	col.AddIndex("idx_analytics_device_type", false, "device_type", "")
	col.AddIndex("idx_analytics_utm_source", false, "utm_source", "")

	if err := app.SaveNoValidate(col); err != nil {
		app.Logger().Error("Failed to create _analytics collection", "error", err)
		return err
	}

	app.Logger().Info("Created _analytics collection")
	return nil
}
