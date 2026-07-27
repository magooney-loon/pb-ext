package jobs

import (
	"github.com/pocketbase/pocketbase/core"
)

// logFields is the _job_logs schema — one row per job execution.
func logFields() []core.Field {
	return []core.Field{
		&core.TextField{Name: "job_id", Required: true, Max: 255},
		&core.TextField{Name: "job_name", Required: true, Max: 255},
		&core.TextField{Name: "description", Required: false, Max: 1000},
		&core.TextField{Name: "expression", Required: false, Max: 255},
		&core.DateField{Name: "start_time", Required: true},
		&core.DateField{Name: "end_time", Required: false},
		&core.NumberField{Name: "duration", Required: false},
		&core.SelectField{
			Name:     "status",
			Required: true,
			Values:   []string{StatusStarted, StatusCompleted, StatusFailed, StatusTimeout},
		},
		&core.TextField{Name: "output", Required: false, Max: 10000},
		&core.TextField{Name: "error", Required: false, Max: 2000},
		&core.SelectField{
			Name:     "trigger_type",
			Required: true,
			Values:   []string{"scheduled", "manual", "api"},
		},
		&core.TextField{Name: "trigger_by", Required: false, Max: 255},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	}
}

// newLogCollection builds the _job_logs collection definition.
func newLogCollection() *core.Collection {
	col := core.NewBaseCollection(Collection)
	col.System = true

	for _, field := range logFields() {
		col.Fields.Add(field)
	}

	return col
}
