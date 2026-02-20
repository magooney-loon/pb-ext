package analytics

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// RegisterRoutes attaches the request tracking middleware to the router.
func (a *Analytics) RegisterRoutes(e *core.ServeEvent) {
	e.Router.BindFunc(func(e *core.RequestEvent) error {
		path := e.Request.URL.Path
		if shouldExclude(path) {
			return e.Next()
		}

		err := e.Next()

		if !isBot(e.Request.UserAgent()) {
			a.track(e.Request)
		}

		return err
	})
}

// track records a page view: upserts the daily counter and inserts a session ring entry.
// No personal data (IP, UA, visitor ID) is written to the database.
func (a *Analytics) track(r *http.Request) {
	path := r.URL.Path
	ua := r.UserAgent()
	deviceType, browser, os := parseUA(ua)
	date := time.Now().Format("2006-01-02")

	// isNewSession uses the in-memory map keyed by hash(ip+ua) — never persisted.
	ip := clientIP(r)
	sessionKey := sessionHash(ip, ua)
	isNew := a.isNewSession(sessionKey)

	if err := a.upsertDailyCounter(path, date, deviceType, browser, isNew); err != nil {
		a.app.Logger().Error("analytics upsert failed", "path", path, "error", err)
	}

	if err := a.insertSessionEntry(path, deviceType, browser, os, isNew); err != nil {
		a.app.Logger().Error("analytics session insert failed", "path", path, "error", err)
	}
}

// upsertDailyCounter increments views (and optionally unique_sessions) for the
// (path, date, device_type, browser) row, creating it if it doesn't exist.
func (a *Analytics) upsertDailyCounter(path, date, deviceType, browser string, isNew bool) error {
	newSession := 0
	if isNew {
		newSession = 1
	}

	sql := fmt.Sprintf(`
		INSERT INTO %s (path, date, device_type, browser, views, unique_sessions)
		VALUES ({:path}, {:date}, {:device}, {:browser}, 1, {:ns})
		ON CONFLICT (path, date, device_type, browser)
		DO UPDATE SET
			views = views + 1,
			unique_sessions = unique_sessions + excluded.unique_sessions
	`, CollectionName)

	_, err := a.app.NonconcurrentDB().NewQuery(sql).
		Bind(dbx.Params{
			"path":    path,
			"date":    date,
			"device":  deviceType,
			"browser": browser,
			"ns":      newSession,
		}).
		Execute()
	return err
}

// insertSessionEntry adds a row to the ring buffer and prunes overflow.
func (a *Analytics) insertSessionEntry(path, deviceType, browser, os string, isNew bool) error {
	col, err := a.app.FindCollectionByNameOrId(SessionsCollectionName)
	if err != nil {
		return fmt.Errorf("find sessions collection: %w", err)
	}

	rec := core.NewRecord(col)
	rec.Set("path", path)
	rec.Set("device_type", deviceType)
	rec.Set("browser", browser)
	rec.Set("os", os)
	rec.Set("timestamp", time.Now())
	rec.Set("is_new_session", isNew)

	if err := a.app.SaveNoValidate(rec); err != nil {
		return fmt.Errorf("save session entry: %w", err)
	}

	// Prune ring — keep only the last SessionRingSize rows.
	pruneSQL := fmt.Sprintf(`
		DELETE FROM %s
		WHERE rowid NOT IN (
			SELECT rowid FROM %s ORDER BY created DESC LIMIT %d
		)
	`, SessionsCollectionName, SessionsCollectionName, SessionRingSize)

	if _, err := a.app.NonconcurrentDB().NewQuery(pruneSQL).Execute(); err != nil {
		a.app.Logger().Error("analytics session ring prune failed", "error", err)
	}

	return nil
}

// isNewSession returns true if sessionKey hasn't been seen within the session window.
// sessionKey is an ephemeral hash of IP+UA — never written to the database.
func (a *Analytics) isNewSession(sessionKey string) bool {
	a.visitorsMu.RLock()
	lastSeen, exists := a.knownVisitors[sessionKey]
	a.visitorsMu.RUnlock()

	now := time.Now()
	if exists && now.Sub(lastSeen) < a.sessionWindow {
		a.visitorsMu.Lock()
		a.knownVisitors[sessionKey] = now
		a.visitorsMu.Unlock()
		return false
	}

	a.visitorsMu.Lock()
	a.knownVisitors[sessionKey] = now
	a.visitorsMu.Unlock()
	return true
}

// --- pure helpers ---

// sessionHash produces a short hash used only for in-memory session deduplication.
// It is never written to the database.
func sessionHash(ip, ua string) string {
	// FNV-1a — fast, non-cryptographic, sufficient for session keying.
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for _, b := range []byte(ip + ua) {
		h ^= uint64(b)
		h *= prime64
	}
	return fmt.Sprintf("%016x", h)
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
