package analytics

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// addToBuffer appends a PageView and triggers a flush if thresholds are met.
func (a *Analytics) addToBuffer(pv PageView) {
	a.bufferMu.Lock()
	a.buffer = append(a.buffer, pv)
	size := len(a.buffer)
	since := time.Since(a.lastFlush)
	a.bufferMu.Unlock()

	a.app.Logger().Debug("Pageview buffered", "path", pv.Path, "visitor", pv.VisitorID, "size", size)

	a.startFlushTimer()

	if size >= a.batchSize || since >= a.flushInterval {
		select {
		case a.flushChan <- struct{}{}:
		default:
		}
	}
}

// ForceFlush triggers an immediate buffer flush.
func (a *Analytics) ForceFlush() {
	a.bufferMu.Lock()
	has := len(a.buffer) > 0
	a.bufferMu.Unlock()

	if has {
		select {
		case a.flushChan <- struct{}{}:
		default:
		}
	}
}

// GetData flushes pending data then computes aggregated analytics.
func (a *Analytics) GetData() (*Data, error) {
	col, err := a.app.FindCollectionByNameOrId(CollectionName)
	if err != nil {
		a.app.Logger().Error("Failed to find _analytics collection", "error", err)
		return DefaultData(), nil
	}

	a.ForceFlush()
	time.Sleep(FlushWaitTime)

	total, err := a.totalCount(col)
	if err != nil {
		a.app.Logger().Error("Failed to get total page views", "error", err)
	}

	lookback := time.Now().AddDate(0, 0, -LookbackDays)
	var records []*core.Record
	if err := a.app.RecordQuery(col.Id).
		OrderBy("timestamp DESC").
		AndWhere(dbx.NewExp("timestamp >= {:ts}", dbx.Params{"ts": lookback})).
		Limit(MaxRecords).
		All(&records); err != nil {
		a.app.Logger().Error("Failed to query _analytics collection", "error", err)
		return DefaultData(), nil
	}

	if len(records) == 0 {
		return DefaultData(), nil
	}

	return a.aggregate(records, total), nil
}

// --- internal ---

func (a *Analytics) backgroundFlushWorker() {
	for range a.flushChan {
		a.flushBuffer()
	}
}

func (a *Analytics) startFlushTimer() {
	a.flushMu.Lock()
	defer a.flushMu.Unlock()

	if a.flushActive {
		return
	}

	a.app.Logger().Debug("Starting analytics flush timer")
	a.flushTicker = time.NewTicker(a.flushInterval)
	a.flushActive = true

	go func() {
		for range a.flushTicker.C {
			a.bufferMu.Lock()
			size := len(a.buffer)
			a.bufferMu.Unlock()

			a.flushMu.Lock()
			if size > 0 {
				select {
				case a.flushChan <- struct{}{}:
				default:
				}
				a.flushMu.Unlock()
			} else {
				a.app.Logger().Debug("Stopping analytics flush timer — buffer empty")
				a.flushTicker.Stop()
				a.flushActive = false
				a.flushMu.Unlock()
				return
			}
		}
	}()
}

func (a *Analytics) sessionCleanupWorker() {
	ticker := time.NewTicker(a.sessionWindow)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-a.sessionWindow)
		a.visitorsMu.Lock()
		before := len(a.knownVisitors)
		for id, t := range a.knownVisitors {
			if t.Before(cutoff) {
				delete(a.knownVisitors, id)
			}
		}
		after := len(a.knownVisitors)
		a.visitorsMu.Unlock()

		if before != after {
			a.app.Logger().Debug("Cleaned up expired sessions", "removed", before-after, "remaining", after)
		}
	}
}

func (a *Analytics) flushBuffer() {
	a.bufferMu.Lock()
	if len(a.buffer) == 0 {
		a.bufferMu.Unlock()
		return
	}
	batch := a.buffer
	a.buffer = make([]PageView, 0, a.batchSize)
	a.lastFlush = time.Now()
	a.bufferMu.Unlock()

	a.app.Logger().Debug("Flushing analytics buffer", "count", len(batch))

	err := a.app.RunInTransaction(func(tx core.App) error {
		col, err := tx.FindCollectionByNameOrId(CollectionName)
		if err != nil {
			return err
		}
		for _, pv := range batch {
			rec := core.NewRecord(col)
			rec.Set("path", pv.Path)
			rec.Set("method", pv.Method)
			rec.Set("ip", pv.IP)
			rec.Set("user_agent", pv.UserAgent)
			rec.Set("referrer", pv.Referrer)
			rec.Set("duration_ms", pv.Duration)
			rec.Set("timestamp", pv.Timestamp)
			rec.Set("visitor_id", pv.VisitorID)
			rec.Set("device_type", pv.DeviceType)
			rec.Set("browser", pv.Browser)
			rec.Set("os", pv.OS)
			rec.Set("country", pv.Country)
			rec.Set("utm_source", pv.UTMSource)
			rec.Set("utm_medium", pv.UTMMedium)
			rec.Set("utm_campaign", pv.UTMCampaign)
			rec.Set("is_new_visit", pv.IsNewVisit)
			rec.Set("query_params", pv.QueryParams)
			if err := tx.SaveNoValidate(rec); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		a.app.Logger().Error("Failed to flush analytics data", "error", err)
	} else {
		a.app.Logger().Debug("Analytics flush complete", "count", len(batch))
	}
}

func (a *Analytics) totalCount(col *core.Collection) (int, error) {
	var count int
	err := a.app.DB().Select("COUNT(*)").From(col.Name).Row(&count)
	return count, err
}

func (a *Analytics) aggregate(records []*core.Record, totalCount int) *Data {
	data := &Data{
		BrowserBreakdown: make(map[string]float64),
		TopPages:         make([]PageStat, 0),
		RecentVisits:     make([]RecentVisit, 0),
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfYesterday := startOfToday.Add(-24 * time.Hour)
	oneHourAgo := now.Add(-time.Hour)

	unique := make(map[string]bool)
	newV, returningV := 0, 0
	pageViews, todayV, yesterdayV, hourlyV := 0, 0, 0, 0
	deviceCounts := make(map[string]int)
	browserCounts := make(map[string]int)
	pathCounts := make(map[string]int)
	recentVisits := make([]RecentVisit, 0, 3)

	for _, rec := range records {
		pageViews++
		visitorID := rec.GetString("visitor_id")
		deviceType := rec.GetString("device_type")
		browser := rec.GetString("browser")
		path := rec.GetString("path")
		ts := rec.GetDateTime("timestamp").Time()
		isNew := rec.GetBool("is_new_visit")
		os := rec.GetString("os")

		if !unique[visitorID] {
			unique[visitorID] = true
			if isNew {
				newV++
			} else {
				returningV++
			}
		}

		deviceCounts[deviceType]++
		browserCounts[browser]++

		if !strings.Contains(path, "/_app/immutable/") {
			pathCounts[path]++
		}

		if ts.After(startOfToday) {
			todayV++
		} else if ts.After(startOfYesterday) {
			yesterdayV++
		}

		if ts.After(oneHourAgo) {
			hourlyV++
			if len(recentVisits) < 3 && !strings.Contains(path, "/_app/immutable/") {
				recentVisits = append(recentVisits, RecentVisit{
					Time: ts, Path: path, DeviceType: deviceType, Browser: browser, OS: os,
				})
			}
		}
	}

	uCount := len(unique)
	data.UniqueVisitors = uCount
	data.NewVisitors = newV
	data.ReturningVisitors = returningV
	data.TotalPageViews = totalCount
	data.TodayPageViews = todayV
	data.YesterdayPageViews = yesterdayV
	if uCount > 0 {
		data.ViewsPerVisitor = float64(pageViews) / float64(uCount)
	}

	// Device breakdown
	total := deviceCounts["desktop"] + deviceCounts["mobile"] + deviceCounts["tablet"]
	if total > 0 {
		data.DesktopPercentage = float64(deviceCounts["desktop"]) / float64(total) * 100
		data.MobilePercentage = float64(deviceCounts["mobile"]) / float64(total) * 100
		data.TabletPercentage = float64(deviceCounts["tablet"]) / float64(total) * 100
		maxC, top := 0, "unknown"
		for d, c := range deviceCounts {
			if c > maxC {
				maxC, top = c, d
			}
		}
		data.TopDeviceType = top
		data.TopDevicePercentage = float64(maxC) / float64(total) * 100
	}

	// Browser breakdown (top 5)
	type bStat struct{ name string; count int }
	bStats := make([]bStat, 0, len(browserCounts))
	totalB := 0
	for b, c := range browserCounts {
		bStats = append(bStats, bStat{b, c})
		totalB += c
	}
	sort.Slice(bStats, func(i, j int) bool { return bStats[i].count > bStats[j].count })
	if len(bStats) > 5 {
		bStats = bStats[:5]
	}
	maxB, topB := 0, "unknown"
	for _, bs := range bStats {
		if totalB > 0 {
			data.BrowserBreakdown[bs.name] = math.Round(float64(bs.count) / float64(totalB) * 100)
		}
		if bs.count > maxB {
			maxB, topB = bs.count, bs.name
		}
	}
	data.TopBrowser = topB

	// Top pages
	for path, count := range pathCounts {
		data.TopPages = append(data.TopPages, PageStat{Path: path, Views: count})
	}
	sort.Slice(data.TopPages, func(i, j int) bool { return data.TopPages[i].Views > data.TopPages[j].Views })
	if len(data.TopPages) > 10 {
		data.TopPages = data.TopPages[:10]
	}

	data.RecentVisits = recentVisits
	data.RecentVisitCount = hourlyV
	data.HourlyActivityPercentage = math.Min(100, float64(hourlyV)/float64(MaxExpectedHourlyVisits)*100)

	return data
}
