package analytics

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// RegisterRoutes attaches the request tracking middleware to the router.
//
// Tracking runs after the handler and does no I/O, so it adds a fixed
// sub-microsecond cost to a request rather than queueing behind SQLite's
// single writer connection.
func (a *Analytics) RegisterRoutes(e *core.ServeEvent) {
	e.Router.BindFunc(func(e *core.RequestEvent) error {
		if !isTrackableRequest(e.Request) {
			return e.Next()
		}

		err := e.Next()

		if isTrackableStatus(e.Status()) && !isBot(e.Request.UserAgent()) {
			// RealIP honours the admin-configured TrustedProxy settings and
			// falls back to the socket address, so forged X-Forwarded-For
			// headers cannot inflate the visitor map on a direct-facing server.
			a.Track(e.RealIP(), e.Request)
		}

		return err
	})
}

// Track records a page view in memory. It performs no database work and is safe
// for concurrent use.
func (a *Analytics) Track(ip string, r *http.Request) {
	now := time.Now()
	ua := r.UserAgent()
	deviceType, browser, os := parseUA(ua)

	// The session key is an in-memory, non-reversible hash and is never stored.
	class := a.visitors.classify(sessionHash(ip, ua), now)

	needsFlush := a.agg.record(visit{
		Path:       a.normalizePath(r.URL.Path),
		DeviceType: deviceType,
		Browser:    browser,
		OS:         os,
		Class:      class,
		At:         now,
	})

	if needsFlush {
		a.requestFlush()
	}
}

// normalizePath collapses absurdly long paths into the overflow bucket before
// they can occupy a distinct-path slot. Per-day cardinality is bounded
// separately by the aggregator.
func (a *Analytics) normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if len(path) > a.cfg.MaxPathLength {
		return OverflowPath
	}
	return path
}

// VisitorMemory reports the current and maximum number of tracked visitors.
// Exposed for monitoring and tests; the value never exceeds max.
func (a *Analytics) VisitorMemory() (current, max int) {
	return a.visitors.size(), a.visitors.capacity()
}

// PendingCounters reports how many counter rows are waiting to be flushed.
func (a *Analytics) PendingCounters() int {
	return a.agg.pendingLen()
}

// --- pure helpers ---

// isTrackableRequest filters to page navigations. Restricting to GET keeps form
// posts, API writes and preflights out of the page-view counters.
func isTrackableRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	return !shouldExclude(r.URL.Path)
}

// isTrackableStatus keeps errors and redirect-loop probes from creating rows.
// A zero status means the handler wrote a body without an explicit code, which
// net/http reports to the client as 200.
func isTrackableStatus(status int) bool {
	return status == 0 || (status >= 200 && status < 400)
}

// sessionHash produces the key used only for in-memory visitor tracking.
// It is never written to the database, and it is not reversible into an
// IP/user-agent pair.
func sessionHash(ip, ua string) uint64 {
	// FNV-1a — fast, non-cryptographic, sufficient for session keying.
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for i := 0; i < len(ip); i++ {
		h ^= uint64(ip[i])
		h *= prime64
	}
	// Separator byte, so ("1.2.3.4", "5x") and ("1.2.3.45", "x") stay distinct.
	h ^= 0
	h *= prime64
	for i := 0; i < len(ua); i++ {
		h ^= uint64(ua[i])
		h *= prime64
	}
	return h
}

func parseUA(userAgent string) (deviceType, browser, os string) {
	ua := strings.ToLower(userAgent)

	deviceType = "desktop"
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
		deviceType = "mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		deviceType = "tablet"
	}

	browser = "unknown"
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

	os = "unknown"
	// iOS user agents contain "like Mac OS X" and Android ones contain "Linux",
	// so the specific platforms must be matched before the generic ones.
	switch {
	case strings.Contains(ua, "iphone"):
		os = "ios"
	case strings.Contains(ua, "ipad"):
		os = "ipados"
	case strings.Contains(ua, "android"):
		os = "android"
	case strings.Contains(ua, "windows"):
		os = "windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		os = "macos"
	case strings.Contains(ua, "linux"):
		os = "linux"
	}

	return
}

func shouldExclude(path string) bool {
	if strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/_/") ||
		strings.HasPrefix(path, "/_app/immutable/") ||
		strings.HasPrefix(path, "/.well-known/") {
		return true
	}

	switch path {
	case "/favicon.ico", "/service-worker.js", "/manifest.json", "/robots.txt":
		return true
	}

	lower := strings.ToLower(path)
	for _, ext := range staticExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isBot(ua string) bool {
	if ua == "" {
		return true
	}
	lower := strings.ToLower(ua)
	for _, p := range botPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

var botPatterns = []string{
	"bot", "crawler", "spider", "lighthouse", "pagespeed",
	"prerender", "headless", "pingdom", "slurp", "googlebot",
	"baiduspider", "bingbot", "yandex", "facebookexternalhit",
	"ahrefsbot", "semrushbot", "screaming frog",
}

var staticExtensions = []string{
	".css", ".js", ".json", ".map", ".webmanifest",
	".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".bmp",
	".tiff", ".tif", ".heic", ".heif", ".avif",
	".mp4", ".webm", ".ogg", ".ogv", ".mov", ".avi", ".wmv", ".flv", ".mkv", ".m4v", ".3gp",
	".mp3", ".wav", ".flac", ".aac", ".m4a", ".wma", ".opus",
	".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".rtf", ".csv", ".md",
	".zip", ".rar", ".7z", ".tar", ".gz", ".bz2",
	".woff", ".woff2", ".ttf", ".eot", ".otf",
}
