package analytics

import "time"

// Analytics configuration constants
const (
	LookbackDays          = 90            // Days to look back for detailed analysis
	MaxRecords            = 50000         // Maximum records to fetch for analysis
	FlushWaitTime         = 100 * time.Millisecond
	MaxExpectedHourlyVisits = 100         // For hourly activity percentage calculation
	CollectionName        = "_analytics"
)

// PageView represents a single tracked request.
type PageView struct {
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	Referrer    string    `json:"referrer"`
	Duration    int64     `json:"duration_ms"`
	Timestamp   time.Time `json:"timestamp"`
	VisitorID   string    `json:"visitor_id"`
	DeviceType  string    `json:"device_type"`
	Browser     string    `json:"browser"`
	OS          string    `json:"os"`
	Country     string    `json:"country"`
	UTMSource   string    `json:"utm_source"`
	UTMMedium   string    `json:"utm_medium"`
	UTMCampaign string    `json:"utm_campaign"`
	IsNewVisit  bool      `json:"is_new_visit"`
	QueryParams string    `json:"query_params"`
}

// Data contains aggregated analytics statistics for the dashboard.
type Data struct {
	UniqueVisitors     int     `json:"unique_visitors"`
	NewVisitors        int     `json:"new_visitors"`
	ReturningVisitors  int     `json:"returning_visitors"`
	TotalPageViews     int     `json:"total_page_views"`
	ViewsPerVisitor    float64 `json:"views_per_visitor"`
	TodayPageViews     int     `json:"today_page_views"`
	YesterdayPageViews int     `json:"yesterday_page_views"`

	TopDeviceType       string  `json:"top_device_type"`
	TopDevicePercentage float64 `json:"top_device_percentage"`
	DesktopPercentage   float64 `json:"desktop_percentage"`
	MobilePercentage    float64 `json:"mobile_percentage"`
	TabletPercentage    float64 `json:"tablet_percentage"`

	TopBrowser       string             `json:"top_browser"`
	BrowserBreakdown map[string]float64 `json:"browser_breakdown"`

	TopPages []PageStat `json:"top_pages"`

	RecentVisits             []RecentVisit `json:"recent_visits"`
	RecentVisitCount         int           `json:"recent_visit_count"`
	HourlyActivityPercentage float64       `json:"hourly_activity_percentage"`
}

// PageStat holds view counts for a single path.
type PageStat struct {
	Path  string `json:"path"`
	Views int    `json:"views"`
}

// RecentVisit is a single entry for the recent visitors display.
type RecentVisit struct {
	Time       time.Time `json:"time"`
	Path       string    `json:"path"`
	DeviceType string    `json:"device_type"`
	Browser    string    `json:"browser"`
	OS         string    `json:"os"`
}

// DefaultData returns a zero-value Data struct for when no records exist.
func DefaultData() *Data {
	return &Data{
		TopDeviceType:    "none",
		TopBrowser:       "none",
		BrowserBreakdown: map[string]float64{"none": 0},
		TopPages:         []PageStat{},
		RecentVisits:     []RecentVisit{},
	}
}
