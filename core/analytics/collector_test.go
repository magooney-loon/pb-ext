package analytics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsTrackableRequest_OnlyPageNavigations(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodGet, "/", true},
		{http.MethodGet, "/pricing", true},
		{http.MethodGet, "/blog/post-1", true},

		// Writes and preflights are not page views.
		{http.MethodPost, "/checkout", false},
		{http.MethodPut, "/account", false},
		{http.MethodDelete, "/account", false},
		{http.MethodPatch, "/account", false},
		{http.MethodOptions, "/", false},
		{http.MethodHead, "/", false},

		// Excluded prefixes and assets.
		{http.MethodGet, "/api/collections/x", false},
		{http.MethodGet, "/_/", false},
		{http.MethodGet, "/_app/immutable/chunk.js", false},
		{http.MethodGet, "/.well-known/acme", false},
		{http.MethodGet, "/favicon.ico", false},
		{http.MethodGet, "/style.css", false},
		{http.MethodGet, "/logo.PNG", false},
	}

	for _, tc := range tests {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if got := isTrackableRequest(r); got != tc.want {
			t.Errorf("isTrackableRequest(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestIsTrackableStatus(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{0, true}, // implicit 200
		{200, true},
		{201, true},
		{301, true},
		{304, true},
		{399, true},
		{400, false},
		{401, false},
		{404, false}, // the main source of junk-path rows
		{429, false},
		{500, false},
	}

	for _, tc := range tests {
		if got := isTrackableStatus(tc.status); got != tc.want {
			t.Errorf("isTrackableStatus(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	a := New(nil, WithMaxPathLength(32))

	if got := a.normalizePath(""); got != "/" {
		t.Errorf("normalizePath(\"\") = %q, want \"/\"", got)
	}
	if got := a.normalizePath("/pricing"); got != "/pricing" {
		t.Errorf("normalizePath(/pricing) = %q, want unchanged", got)
	}

	long := "/" + strings.Repeat("a", 64)
	if got := a.normalizePath(long); got != OverflowPath {
		t.Errorf("normalizePath(long) = %q, want %q", got, OverflowPath)
	}
}

func TestSessionHash_DeterministicAndDistinct(t *testing.T) {
	const ua = "Mozilla/5.0 Chrome/120"

	if sessionHash("1.2.3.4", ua) != sessionHash("1.2.3.4", ua) {
		t.Error("sessionHash is not deterministic")
	}
	if sessionHash("1.2.3.4", ua) == sessionHash("1.2.3.5", ua) {
		t.Error("different IPs produced the same hash")
	}
	if sessionHash("1.2.3.4", ua) == sessionHash("1.2.3.4", ua+"x") {
		t.Error("different user agents produced the same hash")
	}
	// Concatenation must not be ambiguous across the ip/ua boundary.
	if sessionHash("1.2.3.4", "5abc") == sessionHash("1.2.3.45", "abc") {
		t.Error("hash is ambiguous across the ip/ua boundary")
	}
	if sessionHash("", "") == 0 {
		t.Error("empty input hashed to zero")
	}
}

func TestParseUA(t *testing.T) {
	tests := []struct {
		name                  string
		ua                    string
		device, browser, oSys string
	}{
		{
			name:   "windows chrome",
			ua:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36",
			device: "desktop", browser: "chrome", oSys: "windows",
		},
		{
			name:   "android mobile chrome",
			ua:     "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36",
			device: "mobile", browser: "chrome", oSys: "android",
		},
		{
			name:   "ipad safari",
			ua:     "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15",
			device: "tablet", browser: "safari", oSys: "ipados",
		},
		{
			name:   "macos firefox",
			ua:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
			device: "desktop", browser: "firefox", oSys: "macos",
		},
		{
			name:   "windows edge",
			ua:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36 Edg/120.0",
			device: "desktop", browser: "edge", oSys: "windows",
		},
		{
			name:   "iphone safari",
			ua:     "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
			device: "mobile", browser: "safari", oSys: "ios",
		},
		{
			name:   "empty",
			ua:     "",
			device: "desktop", browser: "unknown", oSys: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			device, browser, oSys := parseUA(tc.ua)
			if device != tc.device {
				t.Errorf("device = %q, want %q", device, tc.device)
			}
			if browser != tc.browser {
				t.Errorf("browser = %q, want %q", browser, tc.browser)
			}
			if oSys != tc.oSys {
				t.Errorf("os = %q, want %q", oSys, tc.oSys)
			}
		})
	}
}

func TestIsBot(t *testing.T) {
	bots := []string{
		"",
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0)",
		"Mozilla/5.0 AhrefsBot/7.0",
		"HeadlessChrome/120.0",
		"Chrome-Lighthouse",
		"facebookexternalhit/1.1",
	}
	for _, ua := range bots {
		if !isBot(ua) {
			t.Errorf("isBot(%q) = false, want true", ua)
		}
	}

	humans := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15) Firefox/121.0",
	}
	for _, ua := range humans {
		if isBot(ua) {
			t.Errorf("isBot(%q) = true, want false", ua)
		}
	}
}

func TestShouldExclude(t *testing.T) {
	excluded := []string{
		"/api/health", "/_/settings", "/_app/immutable/x.js", "/.well-known/y",
		"/favicon.ico", "/service-worker.js", "/manifest.json", "/robots.txt",
		"/a.css", "/a.js", "/a.png", "/a.WOFF2", "/doc.pdf", "/archive.tar.gz",
	}
	for _, p := range excluded {
		if !shouldExclude(p) {
			t.Errorf("shouldExclude(%q) = false, want true", p)
		}
	}

	included := []string{"/", "/pricing", "/blog/post", "/docs/api-reference"}
	for _, p := range included {
		if shouldExclude(p) {
			t.Errorf("shouldExclude(%q) = true, want false", p)
		}
	}
}

func TestConfigNormalizeFillsDefaults(t *testing.T) {
	c := Config{}
	c.normalize()

	if c.FlushInterval != DefaultFlushInterval {
		t.Errorf("FlushInterval = %v, want %v", c.FlushInterval, DefaultFlushInterval)
	}
	if c.SessionWindow != DefaultSessionWindow {
		t.Errorf("SessionWindow = %v, want %v", c.SessionWindow, DefaultSessionWindow)
	}
	if c.MaxDistinctPaths != DefaultMaxDistinctPaths {
		t.Errorf("MaxDistinctPaths = %d, want %d", c.MaxDistinctPaths, DefaultMaxDistinctPaths)
	}
	if c.VisitorGenerations != DefaultVisitorGenerations {
		t.Errorf("VisitorGenerations = %d, want %d", c.VisitorGenerations, DefaultVisitorGenerations)
	}
}

func TestOptionsAreApplied(t *testing.T) {
	a := New(nil,
		WithFlushInterval(time.Second),
		WithSessionWindow(5*time.Minute),
		WithVisitorMemory(2, 10),
		WithMaxDistinctPaths(7),
		WithMaxPathLength(9),
		WithCacheTTL(0),
		WithMaxPendingCounters(11),
	)

	cfg := a.Config()
	if cfg.FlushInterval != time.Second {
		t.Errorf("FlushInterval = %v", cfg.FlushInterval)
	}
	if cfg.SessionWindow != 5*time.Minute {
		t.Errorf("SessionWindow = %v", cfg.SessionWindow)
	}
	if cfg.MaxDistinctPaths != 7 {
		t.Errorf("MaxDistinctPaths = %d", cfg.MaxDistinctPaths)
	}
	if cfg.MaxPathLength != 9 {
		t.Errorf("MaxPathLength = %d", cfg.MaxPathLength)
	}
	if cfg.MaxPendingCounters != 11 {
		t.Errorf("MaxPendingCounters = %d", cfg.MaxPendingCounters)
	}
	if current, max := a.VisitorMemory(); current != 0 || max != 20 {
		t.Errorf("VisitorMemory() = (%d, %d), want (0, 20)", current, max)
	}
}
