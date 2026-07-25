package analytics

import (
	"github.com/pocketbase/pocketbase/core"
)

// counterFields is the _analytics schema: one row per
// (path, date, device_type, browser) with the counters folded into it.
func counterFields() []core.Field {
	return []core.Field{
		&core.TextField{Name: "path", Required: true},
		&core.TextField{Name: "date", Required: true},
		&core.TextField{Name: "device_type", Required: false},
		&core.TextField{Name: "browser", Required: false},
		&core.NumberField{Name: "views", Required: true},
		&core.NumberField{Name: "unique_sessions", Required: true},
		&core.NumberField{Name: "returning_sessions", Required: false},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	}
}

// counterIndexes are the indexes _analytics needs.
//
// Beyond the unique key required for upsert correctness, each dashboard
// aggregate gets a covering index whose leading column is its GROUP BY target
// and which carries every column the query reads. That lets SQLite answer them
// from the index alone — no per-row table lookups, and no temp B-tree for the
// grouping. Writes happen once per flush rather than once per request, so the
// extra index maintenance is a good trade.
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

// newCounterCollection builds the _analytics collection definition.
func newCounterCollection() *core.Collection {
	col := core.NewBaseCollection(CollectionName)
	col.System = true

	for _, field := range counterFields() {
		col.Fields.Add(field)
	}
	for _, idx := range counterIndexes {
		col.AddIndex(idx.name, idx.unique, idx.columns, "")
	}

	return col
}
