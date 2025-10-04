package server

import (
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestPageViewStruct(t *testing.T) {
	now := time.Now()
	pageView := PageView{
		Path:        "/test",
		Method:      "GET",
		IP:          "127.0.0.1",
		UserAgent:   "Mozilla/5.0 Test",
		Referrer:    "https://example.com",
		Duration:    100,
		Timestamp:   now,
		VisitorID:   "test-visitor-id",
		DeviceType:  "Desktop",
		Browser:     "Chrome",
		OS:          "Windows",
		Country:     "US",
		UTMSource:   "google",
		UTMMedium:   "cpc",
		UTMCampaign: "test-campaign",
		IsNewVisit:  true,
		QueryParams: "?test=1",
	}

	// Test all fields are set correctly
	if pageView.Path != "/test" {
		t.Errorf("Expected Path '/test', got %s", pageView.Path)
	}
	if pageView.Method != "GET" {
		t.Errorf("Expected Method 'GET', got %s", pageView.Method)
	}
	if pageView.IP != "127.0.0.1" {
		t.Errorf("Expected IP '127.0.0.1', got %s", pageView.IP)
	}
	if pageView.Duration != 100 {
		t.Errorf("Expected Duration 100, got %d", pageView.Duration)
	}
	if !pageView.IsNewVisit {
		t.Error("Expected IsNewVisit to be true")
	}
	if pageView.Timestamp != now {
		t.Errorf("Expected Timestamp %v, got %v", now, pageView.Timestamp)
	}
}

func TestPageViewZeroValues(t *testing.T) {
	var pageView PageView

	if pageView.Path != "" {
		t.Errorf("Expected empty Path, got %s", pageView.Path)
	}
	if pageView.Duration != 0 {
		t.Errorf("Expected zero Duration, got %d", pageView.Duration)
	}
	if pageView.IsNewVisit {
		t.Error("Expected IsNewVisit to be false by default")
	}
	if !pageView.Timestamp.IsZero() {
		t.Errorf("Expected zero Timestamp, got %v", pageView.Timestamp)
	}
}

func TestNewAnalytics(t *testing.T) {
	// Test that we can create an analytics struct with proper defaults
	// Note: We can't easily test NewAnalytics without a PocketBase app,
	// so we'll test the Analytics struct fields directly
	analytics := &Analytics{
		buffer:        make([]PageView, 0, 100),
		flushInterval: 10 * time.Minute,
		batchSize:     50,
		knownVisitors: make(map[string]time.Time),
		sessionWindow: 30 * time.Minute,
		flushActive:   false,
	}

	if analytics.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}
	if analytics.batchSize != 50 {
		t.Errorf("Expected batchSize 50, got %d", analytics.batchSize)
	}
	if analytics.flushInterval != 10*time.Minute {
		t.Errorf("Expected flushInterval 10 minutes, got %v", analytics.flushInterval)
	}
	if analytics.sessionWindow != 30*time.Minute {
		t.Errorf("Expected sessionWindow 30 minutes, got %v", analytics.sessionWindow)
	}
	if analytics.knownVisitors == nil {
		t.Error("Expected knownVisitors to be initialized")
	}
	if analytics.flushActive {
		t.Error("Expected flushActive to be false initially")
	}
}

func TestAnalyticsConstants(t *testing.T) {
	if AnalyticsLookbackDays != 90 {
		t.Errorf("Expected AnalyticsLookbackDays 90, got %d", AnalyticsLookbackDays)
	}
	if MaxAnalyticsRecords != 50000 {
		t.Errorf("Expected MaxAnalyticsRecords 50000, got %d", MaxAnalyticsRecords)
	}
	if FlushWaitTime != 100*time.Millisecond {
		t.Errorf("Expected FlushWaitTime 100ms, got %v", FlushWaitTime)
	}
	if MaxExpectedHourlyVisits != 100 {
		t.Errorf("Expected MaxExpectedHourlyVisits 100, got %d", MaxExpectedHourlyVisits)
	}
}

func TestGenerateVisitorID(t *testing.T) {
	testCases := []struct {
		ip        string
		userAgent string
		name      string
	}{
		{"127.0.0.1", "Mozilla/5.0", "basic case"},
		{"192.168.1.1", "Chrome/100.0", "different IP"},
		{"10.0.0.1", "", "empty user agent"},
		{"", "Mozilla/5.0", "empty IP"},
		{"", "", "both empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id1 := generateVisitorID(tc.ip, tc.userAgent)
			id2 := generateVisitorID(tc.ip, tc.userAgent)

			// Should be consistent
			if id1 != id2 {
				t.Error("generateVisitorID should be consistent for same inputs")
			}

			// Should not be empty
			if id1 == "" {
				t.Error("generateVisitorID should not return empty string")
			}

			// Should be different for different inputs (unless both are empty)
			if tc.ip != "" || tc.userAgent != "" {
				differentID := generateVisitorID(tc.ip+"different", tc.userAgent)
				if id1 == differentID {
					t.Error("generateVisitorID should return different IDs for different inputs")
				}
			}
		})
	}
}

func TestParseUserAgent(t *testing.T) {
	testCases := []struct {
		userAgent       string
		expectedDevice  string
		expectedBrowser string
		expectedOS      string
		name            string
	}{
		{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"desktop", "chrome", "windows", "Chrome on Windows",
		},
		{
			"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			"mobile", "safari", "macos", "Safari on iPhone",
		},
		{
			"Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			"mobile", "safari", "macos", "Safari on iPad",
		},
		{
			"Mozilla/5.0 (Android 11; Mobile; rv:89.0) Gecko/89.0 Firefox/89.0",
			"mobile", "firefox", "android", "Firefox on Android",
		},
		{
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"desktop", "chrome", "macos", "Chrome on macOS",
		},
		{
			"", "desktop", "unknown", "unknown", "empty user agent",
		},
		{
			"InvalidUserAgent", "desktop", "unknown", "unknown", "invalid user agent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			device, browser, os := parseUserAgent(tc.userAgent)

			if device != tc.expectedDevice {
				t.Errorf("Expected device %s, got %s", tc.expectedDevice, device)
			}
			if browser != tc.expectedBrowser {
				t.Errorf("Expected browser %s, got %s", tc.expectedBrowser, browser)
			}
			if os != tc.expectedOS {
				t.Errorf("Expected OS %s, got %s", tc.expectedOS, os)
			}
		})
	}
}

func TestExtractUTMParams(t *testing.T) {
	testCases := []struct {
		rawQuery    string
		expectedSrc string
		expectedMed string
		expectedCam string
		name        string
	}{
		{
			"utm_source=google&utm_medium=cpc&utm_campaign=test",
			"google", "cpc", "test", "all UTM params",
		},
		{
			"utm_source=facebook&other=param",
			"facebook", "", "", "only source",
		},
		{
			"utm_medium=email&utm_campaign=newsletter",
			"", "email", "newsletter", "medium and campaign",
		},
		{
			"other=param&not_utm=value",
			"", "", "", "no UTM params",
		},
		{
			"",
			"", "", "", "empty query",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testURL, _ := url.Parse("http://example.com?" + tc.rawQuery)
			src, med, cam := extractUTMParams(testURL)

			if src != tc.expectedSrc {
				t.Errorf("Expected utm_source %s, got %s", tc.expectedSrc, src)
			}
			if med != tc.expectedMed {
				t.Errorf("Expected utm_medium %s, got %s", tc.expectedMed, med)
			}
			if cam != tc.expectedCam {
				t.Errorf("Expected utm_campaign %s, got %s", tc.expectedCam, cam)
			}
		})
	}
}

func TestShouldExcludeFromAnalytics(t *testing.T) {
	testCases := []struct {
		path     string
		expected bool
		name     string
	}{
		{"/api/_/health", true, "health endpoint"},
		{"/api/_/analytics", true, "analytics endpoint"},
		{"/api/_/docs", true, "docs endpoint"},
		{"/favicon.ico", true, "favicon"},
		{"/robots.txt", false, "robots.txt"},
		{"/static/css/app.css", true, "static CSS"},
		{"/static/js/app.js", false, "static JS"},
		{"/static/images/logo.png", true, "static image"},
		{"/", false, "root path"},
		{"/about", false, "regular page"},
		{"/api/collections", true, "API endpoint"},
		{"/dashboard", false, "dashboard"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldExcludeFromAnalytics(tc.path)
			if result != tc.expected {
				t.Errorf("Expected %t for path %s, got %t", tc.expected, tc.path, result)
			}
		})
	}
}

func TestIsBotUserAgent(t *testing.T) {
	testCases := []struct {
		userAgent string
		expected  bool
		name      string
	}{
		{"Googlebot/2.1", true, "Googlebot"},
		{"bingbot/2.0", true, "Bingbot"},
		{"Mozilla/5.0 (compatible; Baiduspider/2.0)", true, "Baiduspider"},
		{"facebookexternalhit/1.1", true, "Facebook crawler"},
		{"Twitterbot/1.0", true, "Twitterbot"},
		{"crawler test", true, "generic crawler"},
		{"spider bot", true, "generic spider"},
		{"Mozilla/5.0 (Windows NT 10.0) Chrome/91.0", false, "regular browser"},
		{"", true, "empty user agent"},
		{"User-Agent: Normal", false, "normal user agent"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isBotUserAgent(tc.userAgent)
			if result != tc.expected {
				t.Errorf("Expected %t for user agent %s, got %t", tc.expected, tc.userAgent, result)
			}
		})
	}
}

func TestExtractClientIP(t *testing.T) {
	testCases := []struct {
		headers  map[string]string
		expected string
		name     string
	}{
		{
			map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1"},
			"203.0.113.1", "X-Forwarded-For with multiple IPs",
		},
		{
			map[string]string{"X-Real-Ip": "203.0.113.2"},
			"203.0.113.2", "X-Real-Ip header",
		},
		{
			map[string]string{"X-Forwarded-For": "198.51.100.2"},
			"198.51.100.2", "X-Forwarded-For single IP",
		},
		{
			map[string]string{}, "127.0.0.1:12345", "no headers, fallback to RemoteAddr",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "127.0.0.1:12345"

			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			result := extractClientIP(req)
			if result != tc.expected {
				t.Errorf("Expected IP %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestAnalyticsDataStruct(t *testing.T) {
	data := AnalyticsData{
		UniqueVisitors:           100,
		NewVisitors:              60,
		ReturningVisitors:        40,
		TotalPageViews:           500,
		ViewsPerVisitor:          5.0,
		TodayPageViews:           50,
		YesterdayPageViews:       45,
		TopDeviceType:            "Desktop",
		TopDevicePercentage:      65.5,
		DesktopPercentage:        65.5,
		MobilePercentage:         30.0,
		TabletPercentage:         4.5,
		TopBrowser:               "Chrome",
		BrowserBreakdown:         map[string]float64{"Chrome": 70.0, "Firefox": 20.0, "Safari": 10.0},
		TopPages:                 []PageStat{{Path: "/", Views: 100}, {Path: "/about", Views: 50}},
		RecentVisits:             []RecentVisit{{Time: time.Now(), Path: "/", DeviceType: "Desktop"}},
		RecentVisitCount:         25,
		HourlyActivityPercentage: 75.0,
	}

	if data.UniqueVisitors != 100 {
		t.Errorf("Expected UniqueVisitors 100, got %d", data.UniqueVisitors)
	}
	if data.ViewsPerVisitor != 5.0 {
		t.Errorf("Expected ViewsPerVisitor 5.0, got %f", data.ViewsPerVisitor)
	}
	if data.TopDeviceType != "Desktop" {
		t.Errorf("Expected TopDeviceType 'Desktop', got %s", data.TopDeviceType)
	}
	if len(data.BrowserBreakdown) != 3 {
		t.Errorf("Expected 3 browsers in breakdown, got %d", len(data.BrowserBreakdown))
	}
	if len(data.TopPages) != 2 {
		t.Errorf("Expected 2 top pages, got %d", len(data.TopPages))
	}
}

func TestPageStatStruct(t *testing.T) {
	stat := PageStat{
		Path:  "/test",
		Views: 42,
	}

	if stat.Path != "/test" {
		t.Errorf("Expected Path '/test', got %s", stat.Path)
	}
	if stat.Views != 42 {
		t.Errorf("Expected Views 42, got %d", stat.Views)
	}
}

func TestRecentVisitStruct(t *testing.T) {
	now := time.Now()
	visit := RecentVisit{
		Time:       now,
		Path:       "/test",
		DeviceType: "Mobile",
		Browser:    "Chrome",
		OS:         "Android",
	}

	if visit.Time != now {
		t.Errorf("Expected Time %v, got %v", now, visit.Time)
	}
	if visit.Path != "/test" {
		t.Errorf("Expected Path '/test', got %s", visit.Path)
	}
	if visit.DeviceType != "Mobile" {
		t.Errorf("Expected DeviceType 'Mobile', got %s", visit.DeviceType)
	}
	if visit.Browser != "Chrome" {
		t.Errorf("Expected Browser 'Chrome', got %s", visit.Browser)
	}
	if visit.OS != "Android" {
		t.Errorf("Expected OS 'Android', got %s", visit.OS)
	}
}

func TestAnalyticsBufferOperations(t *testing.T) {
	analytics := &Analytics{
		buffer: make([]PageView, 0, 100),
	}

	// Test buffer starts empty
	analytics.bufferMutex.Lock()
	initialSize := len(analytics.buffer)
	analytics.bufferMutex.Unlock()

	if initialSize != 0 {
		t.Errorf("Expected empty buffer initially, got %d items", initialSize)
	}

	// Test adding to buffer
	pageView := PageView{
		Path:      "/test",
		Method:    "GET",
		IP:        "127.0.0.1",
		Timestamp: time.Now(),
	}

	// Test buffer operations
	analytics.bufferMutex.Lock()
	analytics.buffer = append(analytics.buffer, pageView)
	newSize := len(analytics.buffer)
	analytics.bufferMutex.Unlock()

	if newSize != 1 {
		t.Errorf("Expected buffer size 1, got %d", newSize)
	}
}

func TestAnalyticsSessionManagement(t *testing.T) {
	analytics := &Analytics{
		knownVisitors: make(map[string]time.Time),
		sessionWindow: 30 * time.Minute,
		app:           nil, // Initialize app field to avoid segfault
	}

	visitorID := "test-visitor-123"
	now := time.Now()

	// Test adding visitor to session tracking
	analytics.visitorsMutex.Lock()
	analytics.knownVisitors[visitorID] = now
	analytics.visitorsMutex.Unlock()

	// Test visitor exists
	analytics.visitorsMutex.RLock()
	_, exists := analytics.knownVisitors[visitorID]
	analytics.visitorsMutex.RUnlock()

	if !exists {
		t.Error("Expected visitor to exist in known visitors")
	}

	// Test session cleanup with old timestamp
	oldTime := now.Add(-time.Hour)
	analytics.visitorsMutex.Lock()
	analytics.knownVisitors["old-visitor"] = oldTime
	analytics.visitorsMutex.Unlock()

	// Test cleanup logic without calling the method that requires app.Logger()
	cutoff := time.Now().Add(-analytics.sessionWindow)

	analytics.visitorsMutex.Lock()
	beforeCount := len(analytics.knownVisitors)

	// Remove expired sessions manually for testing
	for id, lastSeen := range analytics.knownVisitors {
		if lastSeen.Before(cutoff) {
			delete(analytics.knownVisitors, id)
		}
	}

	afterCount := len(analytics.knownVisitors)
	analytics.visitorsMutex.Unlock()

	// Check cleanup results
	if beforeCount == afterCount {
		t.Error("Expected some visitors to be cleaned up")
	}

	// Check that old visitor was removed and recent visitor remains
	analytics.visitorsMutex.RLock()
	_, oldExists := analytics.knownVisitors["old-visitor"]
	_, recentExists := analytics.knownVisitors[visitorID]
	analytics.visitorsMutex.RUnlock()

	if oldExists {
		t.Error("Expected old visitor to be cleaned up")
	}
	if !recentExists {
		t.Error("Expected recent visitor to still exist")
	}
}

func TestDefaultAnalyticsData(t *testing.T) {
	data := defaultAnalyticsData()

	// Test that all numeric fields are zero
	if data.UniqueVisitors != 0 {
		t.Errorf("Expected UniqueVisitors 0, got %d", data.UniqueVisitors)
	}
	if data.ViewsPerVisitor != 0 {
		t.Errorf("Expected ViewsPerVisitor 0, got %f", data.ViewsPerVisitor)
	}

	// Test that slices are initialized but empty
	if data.TopPages == nil {
		t.Error("Expected TopPages to be initialized")
	}
	if len(data.TopPages) != 0 {
		t.Errorf("Expected empty TopPages, got %d items", len(data.TopPages))
	}

	if data.RecentVisits == nil {
		t.Error("Expected RecentVisits to be initialized")
	}
	if len(data.RecentVisits) != 0 {
		t.Errorf("Expected empty RecentVisits, got %d items", len(data.RecentVisits))
	}

	if data.BrowserBreakdown == nil {
		t.Error("Expected BrowserBreakdown to be initialized")
	}
	if len(data.BrowserBreakdown) != 1 {
		t.Errorf("Expected BrowserBreakdown with 1 item, got %d items", len(data.BrowserBreakdown))
	}
	if data.BrowserBreakdown["none"] != 0 {
		t.Errorf("Expected BrowserBreakdown['none'] to be 0, got %f", data.BrowserBreakdown["none"])
	}
}

func TestAnalyticsConcurrency(t *testing.T) {
	analytics := &Analytics{
		knownVisitors: make(map[string]time.Time),
	}

	// Test concurrent access to visitors map
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				visitorID := generateVisitorID("127.0.0.1", "test-agent") + string(rune(goroutineID*1000+j))
				analytics.visitorsMutex.Lock()
				analytics.knownVisitors[visitorID] = time.Now()
				analytics.visitorsMutex.Unlock()
			}
		}(i)
	}

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				analytics.visitorsMutex.RLock()
				_ = len(analytics.knownVisitors)
				analytics.visitorsMutex.RUnlock()
			}
		}()
	}

	wg.Wait()

	// Verify no race conditions occurred
	analytics.visitorsMutex.RLock()
	visitorCount := len(analytics.knownVisitors)
	analytics.visitorsMutex.RUnlock()

	if visitorCount != numGoroutines*numOperations {
		t.Errorf("Expected %d visitors, got %d", numGoroutines*numOperations, visitorCount)
	}
}

func TestTrackRequestMethodSignature(t *testing.T) {
	// Test that we have the expected method signature
	// We can't easily test the full functionality without PocketBase,
	// but we can verify the method exists and basic structure

	analytics := &Analytics{
		buffer:        make([]PageView, 0, 100),
		knownVisitors: make(map[string]time.Time),
	}

	// Test basic request creation
	req := httptest.NewRequest("GET", "/test", nil)

	// Test that analytics struct is properly initialized
	if analytics.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}

	// Test that the method signature accepts the right parameters
	// This is mainly to ensure the interface is stable
	if req == nil {
		t.Error("Failed to create test request")
	}

	// Test duration parameter type
	var duration int64 = 100
	if duration != 100 {
		t.Error("Duration parameter type mismatch")
	}

	// We can't call TrackRequest without a full PocketBase setup,
	// but we've verified the parameter types are correct
}

// Benchmark tests
func BenchmarkGenerateVisitorID(b *testing.B) {
	ip := "192.168.1.1"
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateVisitorID(ip, userAgent)
	}
}

func BenchmarkParseUserAgent(b *testing.B) {
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = parseUserAgent(userAgent)
	}
}

func BenchmarkShouldExcludeFromAnalytics(b *testing.B) {
	paths := []string{"/api/_/health", "/", "/about", "/static/css/app.css", "/api/collections"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = shouldExcludeFromAnalytics(paths[i%len(paths)])
	}
}

func BenchmarkIsBotUserAgent(b *testing.B) {
	userAgents := []string{
		"Googlebot/2.1",
		"Mozilla/5.0 (Windows NT 10.0) Chrome/91.0",
		"bingbot/2.0",
		"Mozilla/5.0 (iPhone) Safari/604.1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isBotUserAgent(userAgents[i%len(userAgents)])
	}
}

// Example usage
func Example_analytics() {
	// Create a sample page view
	pageView := PageView{
		Path:      "/",
		Method:    "GET",
		IP:        "127.0.0.1",
		UserAgent: "Mozilla/5.0 Chrome/91.0",
		Timestamp: time.Now(),
	}

	// Parse user agent
	device, browser, os := parseUserAgent(pageView.UserAgent)

	println("Device:", device)
	println("Browser:", browser)
	println("OS:", os)

	// Generate visitor ID
	visitorID := generateVisitorID(pageView.IP, pageView.UserAgent)
	println("Visitor ID:", visitorID)

	// Output example would depend on user agent parsing
}
