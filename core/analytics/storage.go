package analytics

import (
	"database/sql"
	"math"
	"time"

	"github.com/pocketbase/dbx"
)

// GetData computes aggregated analytics from the two collections via SQL.
// All aggregation happens in SQLite — no records are loaded into Go memory.
func (a *Analytics) GetData() (*Data, error) {
	// Verify collections exist before querying.
	if _, err := a.app.FindCollectionByNameOrId(CollectionName); err != nil {
		a.app.Logger().Error("_analytics collection not found", "error", err)
		return DefaultData(), nil
	}
	if _, err := a.app.FindCollectionByNameOrId(SessionsCollectionName); err != nil {
		a.app.Logger().Error("_analytics_sessions collection not found", "error", err)
		return DefaultData(), nil
	}

	data := DefaultData()

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// --- 1. Total page views and total unique sessions ---
	var totalViews, totalSessions int
	err := a.app.DB().
		Select("COALESCE(SUM(views),0)", "COALESCE(SUM(unique_sessions),0)").
		From(CollectionName).
		Row(&totalViews, &totalSessions)
	if err != nil {
		a.app.Logger().Error("analytics total query failed", "error", err)
		return DefaultData(), nil
	}
	data.TotalPageViews = totalViews
	data.UniqueVisitors = totalSessions

	// --- 2. Today and yesterday page views ---
	var todayViews, yesterdayViews int
	_ = a.app.DB().
		Select("COALESCE(SUM(views),0)").
		From(CollectionName).
		Where(dbx.NewExp("date = {:d}", dbx.Params{"d": today})).
		Row(&todayViews)
	_ = a.app.DB().
		Select("COALESCE(SUM(views),0)").
		From(CollectionName).
		Where(dbx.NewExp("date = {:d}", dbx.Params{"d": yesterday})).
		Row(&yesterdayViews)
	data.TodayPageViews = todayViews
	data.YesterdayPageViews = yesterdayViews

	// --- 3. New vs returning (from sessions ring) ---
	var newSessions, returningSessions int
	_ = a.app.DB().
		Select("COALESCE(SUM(CASE WHEN is_new_session THEN 1 ELSE 0 END),0)",
			"COALESCE(SUM(CASE WHEN is_new_session THEN 0 ELSE 1 END),0)").
		From(SessionsCollectionName).
		Row(&newSessions, &returningSessions)
	data.NewVisitors = newSessions
	data.ReturningVisitors = returningSessions

	// ViewsPerVisitor
	if totalSessions > 0 {
		data.ViewsPerVisitor = float64(totalViews) / float64(totalSessions)
	}

	// --- 4. Device breakdown ---
	type deviceRow struct {
		DeviceType string `db:"device_type"`
		Views      int    `db:"views"`
	}
	var deviceRows []deviceRow
	_ = a.app.DB().
		Select("device_type", "SUM(views) AS views").
		From(CollectionName).
		GroupBy("device_type").
		All(&deviceRows)

	var deviceTotal int
	deviceMap := make(map[string]int)
	for _, r := range deviceRows {
		deviceMap[r.DeviceType] += r.Views
		deviceTotal += r.Views
	}
	if deviceTotal > 0 {
		data.DesktopPercentage = float64(deviceMap["desktop"]) / float64(deviceTotal) * 100
		data.MobilePercentage = float64(deviceMap["mobile"]) / float64(deviceTotal) * 100
		data.TabletPercentage = float64(deviceMap["tablet"]) / float64(deviceTotal) * 100

		maxC, top := 0, "unknown"
		for d, c := range deviceMap {
			if c > maxC {
				maxC, top = c, d
			}
		}
		data.TopDeviceType = top
		data.TopDevicePercentage = float64(maxC) / float64(deviceTotal) * 100
	}

	// --- 5. Browser breakdown (top 5) ---
	type browserRow struct {
		Browser string `db:"browser"`
		Views   int    `db:"views"`
	}
	var browserRows []browserRow
	_ = a.app.DB().
		Select("browser", "SUM(views) AS views").
		From(CollectionName).
		GroupBy("browser").
		OrderBy("views DESC").
		Limit(5).
		All(&browserRows)

	data.BrowserBreakdown = make(map[string]float64)
	var browserTotal int
	for _, r := range browserRows {
		browserTotal += r.Views
	}
	maxB, topB := 0, "unknown"
	for _, r := range browserRows {
		if browserTotal > 0 {
			data.BrowserBreakdown[r.Browser] = math.Round(float64(r.Views) / float64(browserTotal) * 100)
		}
		if r.Views > maxB {
			maxB, topB = r.Views, r.Browser
		}
	}
	if topB != "unknown" {
		data.TopBrowser = topB
	}

	// --- 6. Top pages ---
	type pageRow struct {
		Path  string `db:"path"`
		Views int    `db:"views"`
	}
	var pageRows []pageRow
	_ = a.app.DB().
		Select("path", "SUM(views) AS views").
		From(CollectionName).
		GroupBy("path").
		OrderBy("views DESC").
		Limit(10).
		All(&pageRows)

	data.TopPages = make([]PageStat, 0, len(pageRows))
	for _, r := range pageRows {
		data.TopPages = append(data.TopPages, PageStat{Path: r.Path, Views: r.Views})
	}

	// --- 7. Recent visits (from sessions ring) ---
	type sessionRow struct {
		Path      string    `db:"path"`
		Device    string    `db:"device_type"`
		Browser   string    `db:"browser"`
		OS        string    `db:"os"`
		Timestamp time.Time `db:"timestamp"`
	}
	var sessionRows []sessionRow
	_ = a.app.DB().
		Select("path", "device_type", "browser", "os", "timestamp").
		From(SessionsCollectionName).
		OrderBy("created DESC").
		Limit(3).
		All(&sessionRows)

	data.RecentVisits = make([]RecentVisit, 0, len(sessionRows))
	for _, r := range sessionRows {
		data.RecentVisits = append(data.RecentVisits, RecentVisit{
			Time:       r.Timestamp,
			Path:       r.Path,
			DeviceType: r.Device,
			Browser:    r.Browser,
			OS:         r.OS,
		})
	}

	// --- 8. Hourly activity ---
	oneHourAgo := time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
	var hourlyCount int
	hourlyErr := a.app.DB().
		Select("COUNT(*)").
		From(SessionsCollectionName).
		Where(dbx.NewExp("timestamp >= {:ts}", dbx.Params{"ts": oneHourAgo})).
		Row(&hourlyCount)
	if hourlyErr != nil && hourlyErr != sql.ErrNoRows {
		a.app.Logger().Error("analytics hourly query failed", "error", hourlyErr)
	}
	data.RecentVisitCount = hourlyCount
	data.HourlyActivityPercentage = math.Min(100, float64(hourlyCount)/float64(MaxExpectedHourlyVisits)*100)

	return data, nil
}
