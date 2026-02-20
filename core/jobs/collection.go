package jobs

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// SetupCollection creates the _job_logs system collection if it doesn't exist.
// If an older schema is detected (missing fields added in the refactor), those
// fields are added in-place without touching existing records.
func SetupCollection(app core.App) error {
	if col, err := app.FindCollectionByNameOrId(Collection); err == nil {
		// Collection exists — check for fields added in the refactor.
		changed := false
		if col.Fields.GetByName("description") == nil {
			col.Fields.Add(&core.TextField{Name: "description", Required: false, Max: 1000})
			changed = true
		}
		if col.Fields.GetByName("expression") == nil {
			col.Fields.Add(&core.TextField{Name: "expression", Required: false, Max: 255})
			changed = true
		}
		if changed {
			app.Logger().Warn("Migrating _job_logs: adding missing fields (description, expression)")
			if err := app.SaveNoValidate(col); err != nil {
				return fmt.Errorf("migration: failed to update _job_logs schema: %w", err)
			}
			app.Logger().Info("Migrated _job_logs collection")
		} else {
			app.Logger().Debug("_job_logs collection already exists (current schema)")
		}
		return nil
	}

	col := core.NewBaseCollection(Collection)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "job_id", Required: true, Max: 255})
	col.Fields.Add(&core.TextField{Name: "job_name", Required: true, Max: 255})
	col.Fields.Add(&core.TextField{Name: "description", Required: false, Max: 1000})
	col.Fields.Add(&core.TextField{Name: "expression", Required: false, Max: 255})
	col.Fields.Add(&core.DateField{Name: "start_time", Required: true})
	col.Fields.Add(&core.DateField{Name: "end_time", Required: false})
	col.Fields.Add(&core.NumberField{Name: "duration", Required: false})
	col.Fields.Add(&core.SelectField{
		Name:     "status",
		Required: true,
		Values:   []string{StatusStarted, StatusCompleted, StatusFailed, StatusTimeout},
	})
	col.Fields.Add(&core.TextField{Name: "output", Required: false, Max: 10000})
	col.Fields.Add(&core.TextField{Name: "error", Required: false, Max: 2000})
	col.Fields.Add(&core.SelectField{
		Name:     "trigger_type",
		Required: true,
		Values:   []string{"scheduled", "manual", "api"},
	})
	col.Fields.Add(&core.TextField{Name: "trigger_by", Required: false, Max: 255})
	col.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	if err := app.SaveNoValidate(col); err != nil {
		return fmt.Errorf("failed to create job logs collection: %w", err)
	}

	app.Logger().Info("Created job logs collection", "name", Collection)
	return nil
}
