package analytics

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/dbx"
)

// dbAggregates is the memoized, database-derived half of the dashboard payload.
// The live half (recent visits, trailing-hour activity) comes from memory and is
// never cached.
type dbAggregates struct {
	TotalViews        int
	NewSessions       int
	ReturningSessions int
	TodayViews        int
	YesterdayViews    int
	Devices           map[string]int
	Browsers          []PageStat // Path field carries the browser name
	TopPages          []PageStat
}

// GetData computes aggregated analytics for the dashboard.
//
// All aggregation happens in SQLite over the trailing LookbackDays, indexed by
// date, and the result is memoized for CacheTTL. No records are loaded into Go
// memory.
func (a *Analytics) GetData() (*Data, error) {
	now := time.Now()

	agg, err := a.aggregates(now)
	if err != nil {
		a.app.Logger().Error("analytics aggregate query failed", "error", err)
		return a.liveOnly(now), nil
	}

	return a.buildData(agg, now), nil
}

// aggregates returns the cached database aggregate, recomputing it (after
// flushing pending counters so the numbers are current) when stale.
func (a *Analytics) aggregates(now time.Time) (*dbAggregates, error) {
	a.cacheMu.Lock()
	if a.cached != nil && now.Before(a.cachedUntil) {
		cached := a.cached
		a.cacheMu.Unlock()
		return cached, nil
	}
	a.cacheMu.Unlock()

	// The dashboard is superuser-only and infrequently loaded, so paying for a
	// synchronous flush here keeps it exactly consistent with live traffic.
	if err := a.Flush(); err != nil {
		a.app.Logger().Error("analytics flush before read failed", "error", err)
	}

	agg, err := a.queryAggregates(now)
	if err != nil {
		return nil, err
	}

	a.cacheMu.Lock()
	a.cached = agg
	a.cachedUntil = now.Add(a.cfg.CacheTTL)
	a.cacheMu.Unlock()

	return agg, nil
}

// invalidateCache drops the memoized aggregate after a successful write.
func (a *Analytics) invalidateCache() {
	a.cacheMu.Lock()
	a.cached = nil
	a.cachedUntil = time.Time{}
	a.cacheMu.Unlock()
}

// queryAggregates runs the four grouped scans that back the dashboard. Each is
// bounded to the retention window so cost tracks LookbackDays, not total history.
//
// All four read auxiliary.db through AuxDB, which routes to the concurrent pool
// — they never contend with the flush worker's writer, and never touch data.db.
func (a *Analytics) queryAggregates(now time.Time) (*dbAggregates, error) {
	if !a.app.AuxHasTable(TableName) {
		return nil, fmt.Errorf("%s table is missing from auxiliary.db", TableName)
	}

	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	cutoff := now.AddDate(0, 0, -LookbackDays).Format("2006-01-02")

	window := dbx.NewExp("date >= {:cutoff}", dbx.Params{"cutoff": cutoff})
	agg := &dbAggregates{Devices: map[string]int{}}

	// 1. Totals — one pass yields lifetime-in-window, today and yesterday.
	err := a.app.AuxDB().
		Select(
			"COALESCE(SUM(views),0)",
			"COALESCE(SUM(unique_sessions),0)",
			"COALESCE(SUM(returning_sessions),0)",
			"COALESCE(SUM(CASE WHEN date = {:today} THEN views ELSE 0 END),0)",
			"COALESCE(SUM(CASE WHEN date = {:yesterday} THEN views ELSE 0 END),0)",
		).
		From(TableName).
		Where(window).
		Bind(dbx.Params{"today": today, "yesterday": yesterday}).
		Row(&agg.TotalViews, &agg.NewSessions, &agg.ReturningSessions, &agg.TodayViews, &agg.YesterdayViews)
	if err != nil {
		return nil, err
	}

	// 2. Device breakdown.
	type groupRow struct {
		Name  string `db:"name"`
		Views int    `db:"views"`
	}
	var deviceRows []groupRow
	if err := a.app.AuxDB().
		Select("device_type AS name", "SUM(views) AS views").
		From(TableName).
		Where(window).
		GroupBy("device_type").
		All(&deviceRows); err != nil {
		return nil, err
	}
	for _, r := range deviceRows {
		agg.Devices[r.Name] += r.Views
	}

	// 3. Browser breakdown (top 5).
	var browserRows []groupRow
	if err := a.app.AuxDB().
		Select("browser AS name", "SUM(views) AS views").
		From(TableName).
		Where(window).
		GroupBy("browser").
		OrderBy("views DESC").
		Limit(5).
		All(&browserRows); err != nil {
		return nil, err
	}
	for _, r := range browserRows {
		agg.Browsers = append(agg.Browsers, PageStat{Path: r.Name, Views: r.Views})
	}

	// 4. Top pages.
	var pageRows []groupRow
	if err := a.app.AuxDB().
		Select("path AS name", "SUM(views) AS views").
		From(TableName).
		Where(window).
		GroupBy("path").
		OrderBy("views DESC").
		Limit(10).
		All(&pageRows); err != nil {
		return nil, err
	}
	for _, r := range pageRows {
		agg.TopPages = append(agg.TopPages, PageStat{Path: r.Name, Views: r.Views})
	}

	return agg, nil
}

// buildData renders the dashboard payload from cached database aggregates plus
// the live in-memory ring and activity buckets.
func (a *Analytics) buildData(agg *dbAggregates, now time.Time) *Data {
	data := DefaultData()

	data.TotalPageViews = agg.TotalViews
	data.NewVisitors = agg.NewSessions
	data.ReturningVisitors = agg.ReturningSessions
	// Sessions are the closest thing to a unique visitor that can be counted
	// without persisting an identifier.
	data.UniqueVisitors = agg.NewSessions + agg.ReturningSessions
	data.TodayPageViews = agg.TodayViews
	data.YesterdayPageViews = agg.YesterdayViews

	if data.UniqueVisitors > 0 {
		data.ViewsPerVisitor = float64(agg.TotalViews) / float64(data.UniqueVisitors)
	}

	// Every counter row carries a device and a browser, so the view total is a
	// valid denominator for both breakdowns.
	total := float64(agg.TotalViews)
	if total > 0 {
		data.DesktopPercentage = float64(agg.Devices["desktop"]) / total * 100
		data.MobilePercentage = float64(agg.Devices["mobile"]) / total * 100
		data.TabletPercentage = float64(agg.Devices["tablet"]) / total * 100

		maxCount, top := 0, "none"
		for device, count := range agg.Devices {
			if count > maxCount {
				maxCount, top = count, device
			}
		}
		data.TopDeviceType = top
		data.TopDevicePercentage = float64(maxCount) / total * 100
	}

	if len(agg.Browsers) > 0 {
		data.BrowserBreakdown = make(map[string]float64, len(agg.Browsers))
		for _, b := range agg.Browsers {
			if total > 0 {
				data.BrowserBreakdown[b.Path] = math.Round(float64(b.Views) / total * 100)
			}
		}
		// Query 3 orders by views, so the first row is the leader.
		if agg.Browsers[0].Views > 0 {
			data.TopBrowser = agg.Browsers[0].Path
		}
	}

	if len(agg.TopPages) > 0 {
		data.TopPages = agg.TopPages
	}

	a.applyLive(data, now)
	return data
}

// liveOnly returns a payload with just the in-memory fields populated, used when
// the database side is unavailable.
func (a *Analytics) liveOnly(now time.Time) *Data {
	data := DefaultData()
	a.applyLive(data, now)
	return data
}

// applyLive fills the fields served straight from memory.
func (a *Analytics) applyLive(data *Data, now time.Time) {
	data.RecentVisits = a.agg.recentVisits(SessionRingSize)

	count, scale := a.agg.activity(now)
	data.RecentVisitCount = count
	if scale > 0 {
		data.HourlyActivityPercentage = math.Min(100, float64(count)/float64(scale)*100)
	}
}
