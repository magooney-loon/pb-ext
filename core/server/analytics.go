package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Analytics provides a lightweight solution for tracking page views and user activity
type Analytics struct {
	app *pocketbase.PocketBase
}

// NewAnalytics creates a new analytics tracker instance
func NewAnalytics(app *pocketbase.PocketBase) *Analytics {
	return &Analytics{
		app: app,
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

	return analytics, nil
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

		// Add fields
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

// RegisterRoutes sets up middleware and endpoints for analytics
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

		// Get request details
		ip := extractClientIP(e.Request)
		userAgent := e.Request.UserAgent()
		referrer := e.Request.Referer()

		// Skip bot traffic
		if isBotUserAgent(userAgent) {
			return err
		}

		// Record the pageview asynchronously
		go a.recordPageview(path, e.Request.Method, ip, userAgent, referrer, duration.Milliseconds())

		return err
	})

	// Add endpoints to access analytics data
	e.Router.GET("/api/analytics/summary", func(e *core.RequestEvent) error {
		stats, err := a.getSummaryStats()
		if err != nil {
			return err
		}
		return e.JSON(http.StatusOK, stats)
	})

	e.Router.GET("/api/analytics/pages", func(e *core.RequestEvent) error {
		topPages, err := a.getTopPages(10) // Get top 10 pages
		if err != nil {
			return err
		}
		return e.JSON(http.StatusOK, topPages)
	})
}

// recordPageview saves a pageview record to the database
func (a *Analytics) recordPageview(path, method, ip, userAgent, referrer string, durationMs int64) {
	collection, err := a.app.FindCollectionByNameOrId("pageviews")
	if err != nil {
		a.app.Logger().Error("Failed to find pageviews collection", "error", err)
		return
	}

	// Clean up IP address by removing port if present
	if idx := strings.Index(ip, ":"); idx >= 0 {
		ip = ip[:idx]
	}

	record := core.NewRecord(collection)
	record.Set("path", path)
	record.Set("method", method)
	record.Set("ip", ip)
	record.Set("user_agent", userAgent)
	record.Set("referrer", referrer)
	record.Set("duration_ms", durationMs)
	record.Set("timestamp", time.Now())

	if err := a.app.SaveNoValidate(record); err != nil {
		a.app.Logger().Error("Failed to save pageview record", "error", err)
	} else {
		a.app.Logger().Debug("Recorded pageview",
			"path", path,
			"ip", ip,
			"duration_ms", durationMs)
	}
}

// getSummaryStats returns basic analytics statistics
func (a *Analytics) getSummaryStats() (map[string]interface{}, error) {
	collection, err := a.app.FindCollectionByNameOrId("pageviews")
	if err != nil {
		a.app.Logger().Error("Failed to find pageviews collection", "error", err)
		return map[string]interface{}{
			"total_pageviews": 0,
			"unique_visitors": 0,
		}, nil
	}

	// Get total pageviews
	var totalRecords []struct {
		ID string `db:"id" json:"id"`
	}
	if err := a.app.RecordQuery(collection.Id).
		Limit(1000).
		All(&totalRecords); err != nil {
		a.app.Logger().Error("Failed to query pageviews", "error", err)
		return map[string]interface{}{
			"total_pageviews": 0,
			"unique_visitors": 0,
		}, nil
	}

	// Get unique IPs (estimate of unique visitors)
	// Note: In a production system, this would use SQL's DISTINCT
	var ipRecords []struct {
		IP string `db:"ip" json:"ip"`
	}
	if err := a.app.RecordQuery(collection.Id).
		Limit(1000).
		All(&ipRecords); err != nil {
		a.app.Logger().Error("Failed to query unique visitors", "error", err)
		return map[string]interface{}{
			"total_pageviews": len(totalRecords),
			"unique_visitors": 0,
		}, nil
	}

	// Count unique IPs
	uniqueIPs := make(map[string]bool)
	for _, r := range ipRecords {
		uniqueIPs[r.IP] = true
	}

	return map[string]interface{}{
		"total_pageviews": len(totalRecords),
		"unique_visitors": len(uniqueIPs),
		"last_updated":    time.Now(),
	}, nil
}

// getTopPages returns the most visited pages
func (a *Analytics) getTopPages(limit int) ([]map[string]interface{}, error) {
	collection, err := a.app.FindCollectionByNameOrId("pageviews")
	if err != nil {
		a.app.Logger().Error("Failed to find pageviews collection", "error", err)
		return []map[string]interface{}{}, nil
	}

	var records []struct {
		ID   string `db:"id" json:"id"`
		Path string `db:"path" json:"path"`
	}

	if err := a.app.RecordQuery(collection.Id).
		Limit(int64(limit * 5)). // Fetch more records to improve our sample
		All(&records); err != nil {
		a.app.Logger().Error("Failed to query pageviews for top pages", "error", err)
		return []map[string]interface{}{}, nil
	}

	// Count occurrences of each path
	pathCounts := make(map[string]int)
	for _, r := range records {
		pathCounts[r.Path]++
	}

	// Convert to slice and sort (simple approach)
	type pageCount struct {
		Path  string
		Count int
	}
	pages := make([]pageCount, 0, len(pathCounts))
	for path, count := range pathCounts {
		pages = append(pages, pageCount{Path: path, Count: count})
	}

	// Sort by count (descending)
	// Using simple bubble sort for simplicity
	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			if pages[i].Count < pages[j].Count {
				pages[i], pages[j] = pages[j], pages[i]
			}
		}
	}

	// Limit to requested number
	if len(pages) > limit {
		pages = pages[:limit]
	}

	// Format result
	result := make([]map[string]interface{}, len(pages))
	for i, p := range pages {
		result[i] = map[string]interface{}{
			"path":      p.Path,
			"pageviews": p.Count,
		}
	}

	return result, nil
}

// Helper functions

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
	return strings.Contains(userAgent, "bot") ||
		strings.Contains(userAgent, "crawler") ||
		strings.Contains(userAgent, "spider") ||
		strings.Contains(userAgent, "lighthouse") ||
		strings.Contains(userAgent, "pagespeed") ||
		strings.Contains(userAgent, "prerender") ||
		strings.Contains(userAgent, "headless")
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
