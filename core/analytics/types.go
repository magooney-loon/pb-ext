package analytics

import "time"

// Analytics configuration constants.
const (
	LookbackDays   = 90           // Days of history included in dashboard aggregates
	CollectionName = "_analytics" // Daily aggregated counters

	// LegacySessionsCollectionName was a per-request ring buffer table used by
	// older pb-ext versions. Recent visits are now held in memory, so the
	// collection is dropped on startup if present.
	LegacySessionsCollectionName = "_analytics_sessions"

	// SessionRingSize is how many recent visits are kept in the in-memory ring
	// that backs the dashboard's "Recent Activity" list.
	SessionRingSize = 50

	// MaxExpectedHourlyVisits is the floor for the hourly-activity denominator.
	// Once observed traffic exceeds it, the running peak is used instead so the
	// progress bar stays meaningful at any scale.
	MaxExpectedHourlyVisits = 100

	// OverflowPath is the bucket that absorbs page views once the per-day
	// distinct-path budget is exhausted. It bounds table growth against
	// high-cardinality routes (/order/{id}) and junk-URL floods.
	OverflowPath = "/*"

	hourlyBuckets = 60 // one-minute buckets covering the trailing hour
)

// Defaults for Config. Override with the With* options.
const (
	DefaultFlushInterval            = 10 * time.Second
	DefaultMaxPendingCounters       = 10000
	DefaultSessionWindow            = 30 * time.Minute
	DefaultVisitorGenerations       = 4
	DefaultMaxVisitorsPerGeneration = 25000
	DefaultMaxDistinctPaths         = 5000
	DefaultMaxPathLength            = 255
	DefaultCacheTTL                 = 5 * time.Second
)

// Config tunes the collector's memory ceilings and write cadence.
// Every field has a bounded default; nothing here grows without limit.
type Config struct {
	// FlushInterval is how often accumulated counters are written to SQLite.
	FlushInterval time.Duration

	// MaxPendingCounters caps distinct (path, date, device, browser) keys held
	// between flushes. Hitting it triggers an early flush.
	MaxPendingCounters int

	// SessionWindow is the idle gap after which a visitor's next request starts
	// a new session.
	SessionWindow time.Duration

	// VisitorGenerations and MaxVisitorsPerGeneration bound the visitor memory
	// at generations*perGeneration entries. Visitor identity is remembered for
	// up to VisitorGenerations*SessionWindow, which is how far back a
	// "returning" visitor can be recognised.
	VisitorGenerations       int
	MaxVisitorsPerGeneration int

	// MaxDistinctPaths caps distinct paths recorded per day; the rest collapse
	// into OverflowPath.
	MaxDistinctPaths int

	// MaxPathLength collapses paths longer than this into OverflowPath.
	MaxPathLength int

	// CacheTTL is how long a computed dashboard aggregate is reused.
	CacheTTL time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		FlushInterval:            DefaultFlushInterval,
		MaxPendingCounters:       DefaultMaxPendingCounters,
		SessionWindow:            DefaultSessionWindow,
		VisitorGenerations:       DefaultVisitorGenerations,
		MaxVisitorsPerGeneration: DefaultMaxVisitorsPerGeneration,
		MaxDistinctPaths:         DefaultMaxDistinctPaths,
		MaxPathLength:            DefaultMaxPathLength,
		CacheTTL:                 DefaultCacheTTL,
	}
}

// normalize replaces non-positive fields with their defaults so a partially
// filled Config is always usable.
func (c *Config) normalize() {
	d := DefaultConfig()
	if c.FlushInterval <= 0 {
		c.FlushInterval = d.FlushInterval
	}
	if c.MaxPendingCounters <= 0 {
		c.MaxPendingCounters = d.MaxPendingCounters
	}
	if c.SessionWindow <= 0 {
		c.SessionWindow = d.SessionWindow
	}
	if c.VisitorGenerations <= 0 {
		c.VisitorGenerations = d.VisitorGenerations
	}
	if c.MaxVisitorsPerGeneration <= 0 {
		c.MaxVisitorsPerGeneration = d.MaxVisitorsPerGeneration
	}
	if c.MaxDistinctPaths <= 0 {
		c.MaxDistinctPaths = d.MaxDistinctPaths
	}
	if c.MaxPathLength <= 0 {
		c.MaxPathLength = d.MaxPathLength
	}
	if c.CacheTTL < 0 {
		c.CacheTTL = d.CacheTTL
	}
}

// Option customizes an Analytics instance.
type Option func(*Config)

// WithFlushInterval sets how often counters are persisted.
func WithFlushInterval(d time.Duration) Option {
	return func(c *Config) { c.FlushInterval = d }
}

// WithMaxPendingCounters sets the pending-counter ceiling that forces an early flush.
func WithMaxPendingCounters(n int) Option {
	return func(c *Config) { c.MaxPendingCounters = n }
}

// WithSessionWindow sets the idle gap that ends a session.
func WithSessionWindow(d time.Duration) Option {
	return func(c *Config) { c.SessionWindow = d }
}

// WithVisitorMemory bounds the visitor map at generations*perGeneration entries.
func WithVisitorMemory(generations, perGeneration int) Option {
	return func(c *Config) {
		c.VisitorGenerations = generations
		c.MaxVisitorsPerGeneration = perGeneration
	}
}

// WithMaxDistinctPaths caps distinct paths recorded per day.
func WithMaxDistinctPaths(n int) Option {
	return func(c *Config) { c.MaxDistinctPaths = n }
}

// WithMaxPathLength collapses longer paths into OverflowPath.
func WithMaxPathLength(n int) Option {
	return func(c *Config) { c.MaxPathLength = n }
}

// WithCacheTTL sets how long dashboard aggregates are reused.
func WithCacheTTL(d time.Duration) Option {
	return func(c *Config) { c.CacheTTL = d }
}

// Data contains aggregated analytics statistics for the dashboard.
type Data struct {
	// UniqueVisitors is the number of sessions started in the lookback window
	// (NewVisitors + ReturningVisitors).
	UniqueVisitors int `json:"unique_visitors"`
	// NewVisitors is sessions opened by a visitor not seen within the
	// collector's visitor memory.
	NewVisitors int `json:"new_visitors"`
	// ReturningVisitors is sessions opened by a visitor who was seen before but
	// whose previous session had lapsed.
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
