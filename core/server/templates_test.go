package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/analytics"
)

// This file is in package server so it can use parseDashboardTemplates and the
// unexported templateFuncs — the same parse the dashboard route performs at
// runtime, where a missing function or renamed field would only be logged.

func renderAnalytics(t *testing.T, data *analytics.Data) string {
	t.Helper()

	tmpl, err := parseDashboardTemplates()
	if err != nil {
		t.Fatalf("parseDashboardTemplates: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "visitor_analytics", struct {
		AnalyticsData *analytics.Data
	}{AnalyticsData: data})
	if err != nil {
		t.Fatalf("execute visitor_analytics: %v", err)
	}

	return buf.String()
}

// populatedData mirrors what GetData returns on a busy site.
func populatedData(recentVisits int) *analytics.Data {
	data := analytics.DefaultData()

	data.UniqueVisitors = 1250
	data.NewVisitors = 1000
	data.ReturningVisitors = 250
	data.TotalPageViews = 8400
	data.ViewsPerVisitor = 6.72
	data.TodayPageViews = 320
	data.YesterdayPageViews = 410

	data.TopDeviceType = "desktop"
	data.TopDevicePercentage = 62.5
	data.DesktopPercentage = 62.5
	data.MobilePercentage = 31.25
	data.TabletPercentage = 6.25

	data.TopBrowser = "chrome"
	data.BrowserBreakdown = map[string]float64{"chrome": 61, "firefox": 20, "safari": 12}

	data.TopPages = []analytics.PageStat{
		{Path: "/", Views: 3200},
		{Path: "/pricing", Views: 1800},
		{Path: "/docs", Views: 900},
		{Path: analytics.OverflowPath, Views: 640},
		{Path: "/blog", Views: 400},
		{Path: "/about", Views: 120},
	}

	now := time.Now()
	for i := 0; i < recentVisits; i++ {
		data.RecentVisits = append(data.RecentVisits, analytics.RecentVisit{
			Time:       now.Add(-time.Duration(i) * time.Second),
			Path:       fmt.Sprintf("/p%d", i),
			DeviceType: []string{"desktop", "mobile", "tablet"}[i%3],
			Browser:    "chrome",
			OS:         "linux",
		})
	}
	data.RecentVisitCount = 4321
	data.HourlyActivityPercentage = 73.5

	return data
}

// TestDashboardTemplates_Parse is the guard for the whole template set: it
// fails if any template references a function that is not in templateFuncs.
func TestDashboardTemplates_Parse(t *testing.T) {
	if _, err := parseDashboardTemplates(); err != nil {
		t.Fatalf("dashboard templates do not parse: %v", err)
	}
}

func TestVisitorAnalytics_RendersPopulatedData(t *testing.T) {
	out := renderAnalytics(t, populatedData(10))

	// Every headline figure must reach the page.
	for _, want := range []string{"1250", "1000", "250", "8400", "6.7", "320", "410", "4321", "desktop", "chrome"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered dashboard is missing %q", want)
		}
	}
}

func TestVisitorAnalytics_RendersEmptyState(t *testing.T) {
	out := renderAnalytics(t, analytics.DefaultData())

	if strings.Contains(out, "NaN") || strings.Contains(out, "+Inf") {
		t.Error("empty state produced NaN/Inf — a percentage divided by zero")
	}
	if !strings.Contains(out, "Visitor Analytics") {
		t.Error("empty state did not render the section")
	}
}

// TestVisitorAnalytics_CapsRecentVisits is a layout regression guard. Recent
// visits now come from a 50-entry in-memory ring rather than a LIMIT 3 query,
// so the card must cap what it renders or it grows ~17x taller.
func TestVisitorAnalytics_CapsRecentVisits(t *testing.T) {
	const ringSize = analytics.SessionRingSize
	out := renderAnalytics(t, populatedData(ringSize))

	rendered := strings.Count(out, "ri-computer-line") +
		strings.Count(out, "ri-smartphone-line") +
		strings.Count(out, "ri-tablet-line")

	// The Devices card also uses these icons for its three labels.
	const deviceCardIcons = 3
	rendered -= deviceCardIcons

	if rendered > 8 {
		t.Errorf("recent activity rendered %d rows, want <= 8 (ring holds %d)", rendered, ringSize)
	}
	if rendered == 0 {
		t.Error("recent activity rendered no rows")
	}
}

func TestVisitorAnalytics_CapsTopPages(t *testing.T) {
	out := renderAnalytics(t, populatedData(5))

	// populatedData has 6 top pages; the card shows 5.
	if strings.Contains(out, ">120<") {
		t.Error("top pages rendered the 6th entry; the card should cap at 5")
	}
	if !strings.Contains(out, "3200") {
		t.Error("top pages did not render the leading entry")
	}
}

// TestVisitorAnalytics_LabelsOverflowPath checks that the cardinality overflow
// bucket is shown readably instead of as a bare "/*".
func TestVisitorAnalytics_LabelsOverflowPath(t *testing.T) {
	out := renderAnalytics(t, populatedData(5))

	if !strings.Contains(out, "other pages") {
		t.Errorf("overflow path %q was not given a readable label", analytics.OverflowPath)
	}
}

func TestPercentOfFunc(t *testing.T) {
	fn, ok := templateFuncs["percentOf"].(func(int, int) float64)
	if !ok {
		t.Fatal("percentOf missing or has an unexpected signature")
	}

	if got := fn(0, 0); got != 0 {
		t.Errorf("percentOf(0,0) = %v, want 0 (must not divide by zero)", got)
	}
	if got := fn(25, 100); got != 25 {
		t.Errorf("percentOf(25,100) = %v, want 25", got)
	}
	if got := fn(1000, 1250); got != 80 {
		t.Errorf("percentOf(1000,1250) = %v, want 80", got)
	}
}

func TestPathLabelFunc(t *testing.T) {
	fn, ok := templateFuncs["pathLabel"].(func(string) string)
	if !ok {
		t.Fatal("pathLabel missing or has an unexpected signature")
	}

	if got := fn(analytics.OverflowPath); got == analytics.OverflowPath {
		t.Errorf("pathLabel(%q) was not rewritten", analytics.OverflowPath)
	}
	if got := fn("/pricing"); got != "/pricing" {
		t.Errorf("pathLabel(/pricing) = %q, want unchanged", got)
	}
}
