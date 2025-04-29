package server

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// PageView represents a single page view event with enhanced metrics
type PageView struct {
	// Core data
	Path      string    `json:"path"`
	Method    string    `json:"method"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referrer  string    `json:"referrer"`
	Duration  int64     `json:"duration_ms"`
	Timestamp time.Time `json:"timestamp"`

	// Enhanced data
	VisitorID   string `json:"visitor_id"`   // Hashed identifier for anonymous visitor tracking
	DeviceType  string `json:"device_type"`  // Desktop, Mobile, Tablet
	Browser     string `json:"browser"`      // Chrome, Firefox, etc.
	OS          string `json:"os"`           // Windows, macOS, iOS, Android, etc.
	Country     string `json:"country"`      // Country (if available)
	UTMSource   string `json:"utm_source"`   // Marketing source
	UTMMedium   string `json:"utm_medium"`   // Marketing medium
	UTMCampaign string `json:"utm_campaign"` // Marketing campaign
	IsNewVisit  bool   `json:"is_new_visit"` // First time visit in session
	QueryParams string `json:"query_params"` // URL query parameters (for analysis)
}

// Analytics provides a lightweight solution for tracking page views
type Analytics struct {
	app           *pocketbase.PocketBase
	buffer        []PageView
	bufferMutex   sync.Mutex
	flushInterval time.Duration
	batchSize     int
	lastFlushTime time.Time
	flushChan     chan struct{}

	// Enhanced tracking
	knownVisitors map[string]time.Time // Track recent visitors (visitorID -> last seen)
	visitorsMutex sync.RWMutex         // Mutex for knownVisitors map
	sessionWindow time.Duration        // Time window to consider a visit part of the same session
}

// AnalyticsData contains statistics for the template
type AnalyticsData struct {
	// Visitor stats
	UniqueVisitors     int     `json:"unique_visitors"`
	NewVisitors        int     `json:"new_visitors"`
	ReturningVisitors  int     `json:"returning_visitors"`
	TotalPageViews     int     `json:"total_page_views"`
	ViewsPerVisitor    float64 `json:"views_per_visitor"`
	TodayPageViews     int     `json:"today_page_views"`
	YesterdayPageViews int     `json:"yesterday_page_views"`

	// Device breakdown
	TopDeviceType       string  `json:"top_device_type"`
	TopDevicePercentage float64 `json:"top_device_percentage"`
	DesktopPercentage   float64 `json:"desktop_percentage"`
	MobilePercentage    float64 `json:"mobile_percentage"`
	TabletPercentage    float64 `json:"tablet_percentage"`

	// Browser stats
	TopBrowser       string             `json:"top_browser"`
	BrowserBreakdown map[string]float64 `json:"browser_breakdown"`

	// Page stats
	TopPages []PageStat `json:"top_pages"`

	// Recent activity
	RecentVisits             []RecentVisit `json:"recent_visits"`
	RecentVisitCount         int           `json:"recent_visit_count"`
	HourlyActivityPercentage float64       `json:"hourly_activity_percentage"`
}

// PageStat represents stats for a single page
type PageStat struct {
	Path  string `json:"path"`
	Views int    `json:"views"`
}

// RecentVisit represents a single visitor entry for display
type RecentVisit struct {
	Time       time.Time `json:"time"`
	Path       string    `json:"path"`
	DeviceType string    `json:"device_type"`
	Browser    string    `json:"browser"`
	OS         string    `json:"os"`
}

// NewAnalytics creates a new analytics tracker instance with batching
func NewAnalytics(app *pocketbase.PocketBase) *Analytics {
	return &Analytics{
		app:           app,
		buffer:        make([]PageView, 0, 100),
		flushInterval: 10 * time.Minute,
		batchSize:     50,
		lastFlushTime: time.Now(),
		flushChan:     make(chan struct{}, 1),
		knownVisitors: make(map[string]time.Time),
		sessionWindow: 30 * time.Minute,
	}
}

// Initialize sets up the analytics system and required collections
func InitializeAnalytics(app *pocketbase.PocketBase) (*Analytics, error) {
	app.Logger().Info("Initializing analytics system")

	analytics := NewAnalytics(app)

	// Ensure collections exist
	if err := SetupAnalyticsCollections(app); err != nil {
		app.Logger().Error("Failed to set up analytics collections", "error", err)
		return nil, err
	}

	// Start background worker for flushing data
	go analytics.backgroundFlushWorker()

	// Start session cleanup worker
	go analytics.sessionCleanupWorker()

	return analytics, nil
}

// sessionCleanupWorker periodically cleans up expired visitor sessions
func (a *Analytics) sessionCleanupWorker() {
	ticker := time.NewTicker(a.sessionWindow)
	defer ticker.Stop()

	for range ticker.C {
		a.cleanupExpiredSessions()
	}
}

// cleanupExpiredSessions removes expired visitor sessions from memory
func (a *Analytics) cleanupExpiredSessions() {
	cutoff := time.Now().Add(-a.sessionWindow)

	a.visitorsMutex.Lock()
	defer a.visitorsMutex.Unlock()

	beforeCount := len(a.knownVisitors)

	// Remove expired sessions
	for id, lastSeen := range a.knownVisitors {
		if lastSeen.Before(cutoff) {
			delete(a.knownVisitors, id)
		}
	}

	afterCount := len(a.knownVisitors)

	if beforeCount != afterCount {
		a.app.Logger().Debug("Cleaned up expired sessions",
			"removed", beforeCount-afterCount,
			"remaining", afterCount)
	}
}

// backgroundFlushWorker processes the buffer at regular intervals
func (a *Analytics) backgroundFlushWorker() {
	ticker := time.NewTicker(a.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.flushChan:
			a.flushBuffer()
		case <-ticker.C:
			// Send a signal to flush
			select {
			case a.flushChan <- struct{}{}:
				// Signal sent
			default:
				// Channel already has a signal, worker is already processing
			}
		}
	}
}

// flushBuffer processes the current buffer of page views
func (a *Analytics) flushBuffer() {
	a.bufferMutex.Lock()

	// If buffer is empty, nothing to do
	if len(a.buffer) == 0 {
		a.bufferMutex.Unlock()
		a.app.Logger().Debug("Analytics buffer flush called with empty buffer")
		return
	}

	// Take current buffer and reset it
	pageViews := a.buffer
	a.buffer = make([]PageView, 0, a.batchSize)
	a.lastFlushTime = time.Now()

	a.bufferMutex.Unlock()

	a.app.Logger().Debug("Flushing analytics buffer", "count", len(pageViews))

	// Process in a transaction
	err := a.app.RunInTransaction(func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId("pageviews")
		if err != nil {
			a.app.Logger().Error("Failed to find pageviews collection", "error", err)
			return err
		}

		// Insert all records in batch
		for _, view := range pageViews {
			record := core.NewRecord(collection)
			record.Set("path", view.Path)
			record.Set("method", view.Method)
			record.Set("ip", view.IP)
			record.Set("user_agent", view.UserAgent)
			record.Set("referrer", view.Referrer)
			record.Set("duration_ms", view.Duration)
			record.Set("timestamp", view.Timestamp)

			// Enhanced fields
			record.Set("visitor_id", view.VisitorID)
			record.Set("device_type", view.DeviceType)
			record.Set("browser", view.Browser)
			record.Set("os", view.OS)
			record.Set("country", view.Country)
			record.Set("utm_source", view.UTMSource)
			record.Set("utm_medium", view.UTMMedium)
			record.Set("utm_campaign", view.UTMCampaign)
			record.Set("is_new_visit", view.IsNewVisit)
			record.Set("query_params", view.QueryParams)

			if err := txApp.SaveNoValidate(record); err != nil {
				a.app.Logger().Error("Failed to save pageview record", "error", err)
				return err
			}
		}

		return nil
	})

	if err != nil {
		a.app.Logger().Error("Failed to flush analytics data", "error", err)
	} else {
		a.app.Logger().Debug("Successfully flushed analytics data", "count", len(pageViews))
	}
}

// SetupAnalyticsCollections creates the necessary collections if they don't exist
func SetupAnalyticsCollections(app *pocketbase.PocketBase) error {
	// Check if pageviews collection exists
	pageviewsCol, err := app.FindCollectionByNameOrId("pageviews")
	if err != nil {
		// Create the collection
		app.Logger().Debug("Creating pageviews collection")
		pageviewsCol = core.NewBaseCollection("pageviews")
		pageviewsCol.System = true

		// Add base fields
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "path",
			Required: true,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "method",
			Required: true,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "ip",
			Required: true,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "user_agent",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "referrer",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.NumberField{
			Name:     "duration_ms",
			Required: true,
		})
		pageviewsCol.Fields.Add(&core.DateField{
			Name:     "timestamp",
			Required: true,
		})

		// Add enhanced fields
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "visitor_id",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "device_type",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "browser",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "os",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "country",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "utm_source",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "utm_medium",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "utm_campaign",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.BoolField{
			Name:     "is_new_visit",
			Required: false,
		})
		pageviewsCol.Fields.Add(&core.TextField{
			Name:     "query_params",
			Required: false,
		})

		pageviewsCol.Fields.Add(&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		})
		pageviewsCol.Fields.Add(&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		})

		// Add indexes for better query performance
		pageviewsCol.AddIndex("idx_pageviews_timestamp", false, "timestamp", "")
		pageviewsCol.AddIndex("idx_pageviews_path", false, "path", "")
		pageviewsCol.AddIndex("idx_pageviews_ip", false, "ip", "")
		pageviewsCol.AddIndex("idx_pageviews_visitor_id", false, "visitor_id", "")
		pageviewsCol.AddIndex("idx_pageviews_device_type", false, "device_type", "")
		pageviewsCol.AddIndex("idx_pageviews_utm_source", false, "utm_source", "")

		// Save the collection
		if err := app.SaveNoValidate(pageviewsCol); err != nil {
			app.Logger().Error("Failed to create pageviews collection", "error", err)
			return err
		}

		app.Logger().Info("Created pageviews collection")
	} else {
		app.Logger().Debug("pageviews collection already exists",
			"id", pageviewsCol.Id,
			"name", pageviewsCol.Name)
	}

	return nil
}

// RegisterRoutes sets up middleware for analytics tracking
func (a *Analytics) RegisterRoutes(e *core.ServeEvent) {
	// Add middleware to track pageviews
	e.Router.BindFunc(func(e *core.RequestEvent) error {
		// Skip tracking for certain paths
		path := e.Request.URL.Path
		if shouldExcludeFromAnalytics(path) {
			return e.Next()
		}

		start := time.Now()

		// Process the request
		err := e.Next()

		// Track the request after it's processed
		duration := time.Since(start)

		// Skip bot traffic
		if isBotUserAgent(e.Request.UserAgent()) {
			return err
		}

		// Add the pageview to buffer with enhanced data
		a.trackPageView(e.Request, duration.Milliseconds())

		return err
	})
}

// trackPageView processes a request and adds it to the buffer with enhanced data
func (a *Analytics) trackPageView(r *http.Request, durationMs int64) {
	// Basic data extraction
	path := r.URL.Path
	method := r.Method
	ip := extractClientIP(r)
	userAgent := r.UserAgent()
	referrer := r.Referer()

	// Extract enhanced data
	visitorID := generateVisitorID(ip, userAgent)
	deviceType, browser, os := parseUserAgent(userAgent)
	utmSource, utmMedium, utmCampaign := extractUTMParams(r.URL)
	queryParams := r.URL.RawQuery

	// Determine if this is a new visit
	isNewVisit := a.isNewVisit(visitorID)

	// Create the page view record
	pageView := PageView{
		// Basic data
		Path:      path,
		Method:    method,
		IP:        ip,
		UserAgent: userAgent,
		Referrer:  referrer,
		Duration:  durationMs,
		Timestamp: time.Now(),

		// Enhanced data
		VisitorID:   visitorID,
		DeviceType:  deviceType,
		Browser:     browser,
		OS:          os,
		Country:     "", // Would require GeoIP lookup
		UTMSource:   utmSource,
		UTMMedium:   utmMedium,
		UTMCampaign: utmCampaign,
		IsNewVisit:  isNewVisit,
		QueryParams: queryParams,
	}

	// Add to buffer
	a.addToBuffer(pageView)
}

// TrackRequest allows manual tracking of a request (for use outside middleware)
func (a *Analytics) TrackRequest(r *http.Request, durationMs int64) {
	// Skip bot traffic
	if isBotUserAgent(r.UserAgent()) {
		return
	}

	// Track page view with all enhanced data
	a.trackPageView(r, durationMs)
}

// ForceFlush manually triggers a buffer flush
func (a *Analytics) ForceFlush() {
	select {
	case a.flushChan <- struct{}{}:
		// Signal sent
		a.app.Logger().Debug("Manual analytics flush triggered")
	default:
		// Already signaled
	}
}

// addToBuffer adds a pageview to the buffer and triggers flush if needed
func (a *Analytics) addToBuffer(pageView PageView) {
	a.bufferMutex.Lock()
	a.buffer = append(a.buffer, pageView)
	bufferSize := len(a.buffer)
	timeSinceLastFlush := time.Since(a.lastFlushTime)
	a.bufferMutex.Unlock()

	a.app.Logger().Debug("Pageview added to buffer",
		"path", pageView.Path,
		"visitor", pageView.VisitorID,
		"device", pageView.DeviceType,
		"bufferSize", bufferSize)

	// Trigger flush if buffer size exceeds batch size or flush interval has passed
	if bufferSize >= a.batchSize || timeSinceLastFlush >= a.flushInterval {
		select {
		case a.flushChan <- struct{}{}:
			// Signal sent, worker will process
			a.app.Logger().Debug("Analytics flush signal sent")
		default:
			// Channel already has a signal, worker is or will be processing
			a.app.Logger().Debug("Analytics flush already signaled")
		}
	}
}

// isNewVisit checks if a visitor is new (not seen recently)
func (a *Analytics) isNewVisit(visitorID string) bool {
	a.visitorsMutex.RLock()
	lastSeen, exists := a.knownVisitors[visitorID]
	a.visitorsMutex.RUnlock()

	now := time.Now()

	// If visitor exists and was seen recently, not a new visit
	if exists && now.Sub(lastSeen) < a.sessionWindow {
		// Update the last seen time
		a.visitorsMutex.Lock()
		a.knownVisitors[visitorID] = now
		a.visitorsMutex.Unlock()
		return false
	}

	// New visitor or not seen recently
	a.visitorsMutex.Lock()
	a.knownVisitors[visitorID] = now
	a.visitorsMutex.Unlock()
	return true
}

// Helper functions

// generateVisitorID creates a consistent but anonymous ID for a visitor
func generateVisitorID(ip, userAgent string) string {
	// Create a hash of IP and user agent to identify the visitor without storing PII
	hash := sha256.New()
	hash.Write([]byte(ip + userAgent))
	return hex.EncodeToString(hash.Sum(nil))[:16] // First 16 chars of hash
}

// parseUserAgent extracts device type, browser, and OS from user agent
func parseUserAgent(userAgent string) (string, string, string) {
	ua := strings.ToLower(userAgent)

	// Device type detection
	deviceType := "desktop"
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
		deviceType = "mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		deviceType = "tablet"
	}

	// Browser detection (simplified)
	browser := "unknown"
	switch {
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg"):
		browser = "chrome"
	case strings.Contains(ua, "firefox"):
		browser = "firefox"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		browser = "safari"
	case strings.Contains(ua, "edg"):
		browser = "edge"
	case strings.Contains(ua, "opera"):
		browser = "opera"
	}

	// OS detection (simplified)
	os := "unknown"
	switch {
	case strings.Contains(ua, "windows"):
		os = "windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		os = "macos"
	case strings.Contains(ua, "linux") && !strings.Contains(ua, "android"):
		os = "linux"
	case strings.Contains(ua, "iphone"):
		os = "ios"
	case strings.Contains(ua, "ipad"):
		os = "ipados"
	case strings.Contains(ua, "android"):
		os = "android"
	}

	return deviceType, browser, os
}

// extractUTMParams extracts UTM parameters from a URL
func extractUTMParams(reqURL *url.URL) (source, medium, campaign string) {
	query := reqURL.Query()
	source = query.Get("utm_source")
	medium = query.Get("utm_medium")
	campaign = query.Get("utm_campaign")
	return
}

// shouldExcludeFromAnalytics returns true if the path should not be tracked
func shouldExcludeFromAnalytics(path string) bool {
	return strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/_/") ||
		path == "/favicon.ico" ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") ||
		strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".woff") ||
		strings.HasSuffix(path, ".woff2") ||
		strings.HasSuffix(path, ".ttf")
}

// isBotUserAgent returns true if the user agent appears to be a bot
func isBotUserAgent(userAgent string) bool {
	if userAgent == "" {
		return true
	}

	userAgent = strings.ToLower(userAgent)

	// Common bot patterns
	botPatterns := []string{
		"bot", "crawler", "spider", "lighthouse", "pagespeed",
		"prerender", "headless", "pingdom", "slurp", "googlebot",
		"baiduspider", "bingbot", "yandex", "facebookexternalhit",
		"ahrefsbot", "semrushbot", "screaming frog",
	}

	for _, pattern := range botPatterns {
		if strings.Contains(userAgent, pattern) {
			return true
		}
	}

	return false
}

// extractClientIP gets the client's IP address from the request
func extractClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check for X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	// Extract from RemoteAddr
	ip, _, err := strings.Cut(r.RemoteAddr, ":")
	if err || ip == "" {
		return r.RemoteAddr
	}
	return ip
}

// GetAnalyticsData retrieves analytics data for display in the template
func (a *Analytics) GetAnalyticsData() (*AnalyticsData, error) {
	// Create analytics data structure
	data := &AnalyticsData{
		BrowserBreakdown: make(map[string]float64),
		TopPages:         make([]PageStat, 0),
		RecentVisits:     make([]RecentVisit, 0),
	}

	// Get the pageviews collection
	collection, err := a.app.FindCollectionByNameOrId("pageviews")
	if err != nil {
		a.app.Logger().Error("Failed to find pageviews collection", "error", err)
		return defaultAnalyticsData(), nil
	}

	// First flush any pending data to ensure we have the latest
	a.ForceFlush()
	time.Sleep(100 * time.Millisecond) // Brief pause to let flush complete

	// Get all pageviews for analysis
	var records []*core.Record
	if err := a.app.RecordQuery(collection.Id).
		OrderBy("timestamp DESC").
		Limit(1000).
		All(&records); err != nil {
		a.app.Logger().Error("Failed to query pageviews", "error", err)
		return defaultAnalyticsData(), nil
	}

	// If no records, return default data
	if len(records) == 0 {
		return defaultAnalyticsData(), nil
	}

	// Process data for statistics
	uniqueVisitors := make(map[string]bool)
	newVisitors := 0
	returningVisitors := 0
	pageViews := 0
	todayViews := 0
	yesterdayViews := 0

	// Device stats
	deviceCounts := make(map[string]int)

	// Browser stats
	browserCounts := make(map[string]int)

	// Page stats
	pathCounts := make(map[string]int)

	// Recent activity
	recentVisits := make([]RecentVisit, 0)
	hourlyVisits := 0

	// Timestamps for today/yesterday
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfYesterday := startOfToday.Add(-24 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)

	// Process all records
	for _, record := range records {
		pageViews++

		// Extract data from record
		visitorID := record.GetString("visitor_id")
		deviceType := record.GetString("device_type")
		browser := record.GetString("browser")
		path := record.GetString("path")
		timestampValue := record.GetDateTime("timestamp")
		timestamp := timestampValue.Time() // Convert types.DateTime to time.Time
		isNewVisit := record.GetBool("is_new_visit")
		os := record.GetString("os")

		// Update unique visitors and new/returning
		if !uniqueVisitors[visitorID] {
			uniqueVisitors[visitorID] = true
			if isNewVisit {
				newVisitors++
			} else {
				returningVisitors++
			}
		}

		// Count device types
		deviceCounts[deviceType]++

		// Count browsers
		browserCounts[browser]++

		// Count page views by path
		pathCounts[path]++

		// Check if view is from today or yesterday
		if timestamp.After(startOfToday) {
			todayViews++
		} else if timestamp.After(startOfYesterday) {
			yesterdayViews++
		}

		// Add to recent visits if within last hour
		if timestamp.After(oneHourAgo) {
			hourlyVisits++

			// Only keep the 3 most recent for display
			if len(recentVisits) < 3 {
				recentVisits = append(recentVisits, RecentVisit{
					Time:       timestamp,
					Path:       path,
					DeviceType: deviceType,
					Browser:    browser,
					OS:         os,
				})
			}
		}
	}

	// Calculate unique visitor count
	uniqueVisitorCount := len(uniqueVisitors)

	// Fill in the analytics data
	data.UniqueVisitors = uniqueVisitorCount
	data.NewVisitors = newVisitors
	data.ReturningVisitors = returningVisitors
	data.TotalPageViews = pageViews
	data.TodayPageViews = todayViews
	data.YesterdayPageViews = yesterdayViews

	// Calculate views per visitor
	if uniqueVisitorCount > 0 {
		data.ViewsPerVisitor = float64(pageViews) / float64(uniqueVisitorCount)
	} else {
		data.ViewsPerVisitor = 0
	}

	// Process device stats
	totalDevices := deviceCounts["desktop"] + deviceCounts["mobile"] + deviceCounts["tablet"]
	if totalDevices > 0 {
		data.DesktopPercentage = float64(deviceCounts["desktop"]) / float64(totalDevices) * 100
		data.MobilePercentage = float64(deviceCounts["mobile"]) / float64(totalDevices) * 100
		data.TabletPercentage = float64(deviceCounts["tablet"]) / float64(totalDevices) * 100

		// Find top device type
		maxCount := 0
		topDevice := "unknown"
		for device, count := range deviceCounts {
			if count > maxCount {
				maxCount = count
				topDevice = device
			}
		}
		data.TopDeviceType = topDevice
		data.TopDevicePercentage = float64(maxCount) / float64(totalDevices) * 100
	}

	// Process browser stats
	totalBrowsers := 0
	for _, count := range browserCounts {
		totalBrowsers += count
	}

	if totalBrowsers > 0 {
		// Find top browser and calculate percentages
		maxCount := 0
		topBrowser := "unknown"

		for browser, count := range browserCounts {
			percent := float64(count) / float64(totalBrowsers) * 100
			data.BrowserBreakdown[browser] = percent

			if count > maxCount {
				maxCount = count
				topBrowser = browser
			}
		}

		data.TopBrowser = topBrowser
	}

	// Process page stats
	for path, count := range pathCounts {
		data.TopPages = append(data.TopPages, PageStat{
			Path:  path,
			Views: count,
		})
	}

	// Sort top pages by view count (descending)
	sort.Slice(data.TopPages, func(i, j int) bool {
		return data.TopPages[i].Views > data.TopPages[j].Views
	})

	// Limit to top 10 pages
	if len(data.TopPages) > 10 {
		data.TopPages = data.TopPages[:10]
	}

	// Add recent visit data
	data.RecentVisits = recentVisits
	data.RecentVisitCount = hourlyVisits

	// Calculate hourly activity percentage (relative to 100 being our max expected hourly visits)
	maxExpectedHourlyVisits := 100 // Adjust based on your site's traffic
	data.HourlyActivityPercentage = math.Min(100, float64(hourlyVisits)/float64(maxExpectedHourlyVisits)*100)

	return data, nil
}

// defaultAnalyticsData returns a default analytics data structure when no data is available
func defaultAnalyticsData() *AnalyticsData {
	return &AnalyticsData{
		UniqueVisitors:           0,
		NewVisitors:              0,
		ReturningVisitors:        0,
		TotalPageViews:           0,
		ViewsPerVisitor:          0,
		TodayPageViews:           0,
		YesterdayPageViews:       0,
		TopDeviceType:            "none",
		TopDevicePercentage:      0,
		DesktopPercentage:        0,
		MobilePercentage:         0,
		TabletPercentage:         0,
		TopBrowser:               "none",
		BrowserBreakdown:         map[string]float64{"none": 0},
		TopPages:                 []PageStat{},
		RecentVisits:             []RecentVisit{},
		RecentVisitCount:         0,
		HourlyActivityPercentage: 0,
	}
}
