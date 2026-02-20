package jobs

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// SetupCollection creates the _job_logs system collection if it doesn't exist.
func SetupCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(Collection); err == nil {
		app.Logger().Debug("Job logs collection already exists")
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
