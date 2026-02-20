package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// RegisterRoutes attaches the request tracking middleware to the router.
func (a *Analytics) RegisterRoutes(e *core.ServeEvent) {
	e.Router.BindFunc(func(e *core.RequestEvent) error {
		path := e.Request.URL.Path
		if shouldExclude(path) {
			return e.Next()
		}

		start := time.Now()
		err := e.Next()
		duration := time.Since(start)

		if !isBot(e.Request.UserAgent()) {
			a.track(e.Request, duration.Milliseconds())
		}

		return err
	})
}

// track builds a PageView from the request and adds it to the buffer.
func (a *Analytics) track(r *http.Request, durationMs int64) {
	ip := clientIP(r)
	ua := r.UserAgent()
	visitorID := visitorHash(ip, ua)
	deviceType, browser, os := parseUA(ua)
	utmSrc, utmMedium, utmCampaign := extractUTM(r.URL)

	pv := PageView{
		Path:        r.URL.Path,
		Method:      r.Method,
		IP:          ip,
		UserAgent:   ua,
		Referrer:    r.Referer(),
		Duration:    durationMs,
		Timestamp:   time.Now(),
		VisitorID:   visitorID,
		DeviceType:  deviceType,
		Browser:     browser,
		OS:          os,
		Country:     "",
		UTMSource:   utmSrc,
		UTMMedium:   utmMedium,
		UTMCampaign: utmCampaign,
		IsNewVisit:  a.isNewVisit(visitorID),
		QueryParams: r.URL.RawQuery,
	}

	a.addToBuffer(pv)
}

// isNewVisit returns true if the visitor hasn't been seen within the session window.
func (a *Analytics) isNewVisit(visitorID string) bool {
	a.visitorsMu.RLock()
	lastSeen, exists := a.knownVisitors[visitorID]
	a.visitorsMu.RUnlock()

	now := time.Now()
	if exists && now.Sub(lastSeen) < a.sessionWindow {
		a.visitorsMu.Lock()
		a.knownVisitors[visitorID] = now
		a.visitorsMu.Unlock()
		return false
	}

	a.visitorsMu.Lock()
	a.knownVisitors[visitorID] = now
	a.visitorsMu.Unlock()
	return true
}

// --- pure helpers (no receiver) ---

func visitorHash(ip, ua string) string {
	h := sha256.New()
	h.Write([]byte(ip + ua))
	return hex.EncodeToString(h.Sum(nil))[:16]
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

	return
}

func extractUTM(u *url.URL) (source, medium, campaign string) {
	q := u.Query()
	return q.Get("utm_source"), q.Get("utm_medium"), q.Get("utm_campaign")
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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	ip, _, _ := strings.Cut(r.RemoteAddr, ":")
	if ip == "" {
		return r.RemoteAddr
	}
	return ip
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
