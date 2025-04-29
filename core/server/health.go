package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shirou/gopsutil/v3/host"
)

// HealthResponse represents health check response data
type HealthResponse struct {
	Status        string       `json:"status"`
	ServerStats   *ServerStats `json:"server_stats"`
	SystemStats   interface{}  `json:"system_stats"`
	LastCheckTime time.Time    `json:"last_check_time"`
}

// Template functions map
var templateFuncs = template.FuncMap{
	"multiply": func(a, b float64) float64 {
		return a * b
	},
	"divide": func(a, b interface{}) float64 {
		var af, bf float64

		switch v := a.(type) {
		case float64:
			af = v
		case uint64:
			af = float64(v)
		default:
			return 0
		}

		switch v := b.(type) {
		case float64:
			bf = v
		case uint64:
			bf = float64(v)
		default:
			return 0
		}

		if bf == 0 {
			return 0
		}
		return af / bf
	},
	"divideFloat64": func(a interface{}, b float64) float64 {
		if b == 0 {
			return 0
		}

		var af float64
		switch v := a.(type) {
		case float64:
			af = v
		case uint64:
			af = float64(v)
		case int64:
			af = float64(v)
		case int:
			af = float64(v)
		default:
			return 0
		}

		return af / b
	},
	"int64": func(v interface{}) int64 {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case float64:
			return int64(val)
		case time.Duration:
			return int64(val)
		default:
			return 0
		}
	},
	"errorRate": func(errors, total uint64) float64 {
		if total == 0 {
			return 0
		}
		return float64(errors) * 100 / float64(total)
	},
	"avgCPUUsage": func(cpus []monitoring.CPUInfo) float64 {
		if len(cpus) == 0 {
			return 0
		}
		var total float64
		for _, cpu := range cpus {
			total += cpu.Usage
		}
		return total / float64(len(cpus))
	},
	"formatBytes": func(bytes uint64) string {
		const unit = 1024
		if bytes < unit {
			return fmt.Sprintf("%d B", bytes)
		}
		div, exp := uint64(unit), 0
		for n := bytes / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
	},
	"getDiskTemp": func(stats *monitoring.SystemStats) float64 {
		if !stats.HasTempData {
			return 0
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return 0
		}

		for _, temp := range temps {
			if monitoring.IsDiskTemp(temp.SensorKey) {
				return temp.Temperature
			}
		}
		return 0
	},
	"getSystemTemp": func(stats *monitoring.SystemStats) float64 {
		if !stats.HasTempData {
			return 0
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return 0
		}

		for _, temp := range temps {
			if monitoring.IsSystemTemp(temp.SensorKey) {
				return temp.Temperature
			}
		}
		return 0
	},
	"getAmbientTemp": func(stats *monitoring.SystemStats) float64 {
		if !stats.HasTempData {
			return 0
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return 0
		}

		for _, temp := range temps {
			if strings.Contains(strings.ToLower(temp.SensorKey), "ambient") {
				return temp.Temperature
			}
		}
		return 0
	},
	"hasDiskTemps": func(stats *monitoring.SystemStats) bool {
		if !stats.HasTempData {
			return false
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return false
		}

		for _, temp := range temps {
			if monitoring.IsDiskTemp(temp.SensorKey) {
				return true
			}
		}
		return false
	},
	"formatTime": func(t time.Time) string {
		return t.Format("15:04:05")
	},
	"inc": func(i int) int {
		return i + 1
	},
}

// RegisterHealthRoute registers the health check endpoint
func (s *Server) RegisterHealthRoute(e *core.ServeEvent) {
	// Get health dashboard credentials from environment
	healthUser := os.Getenv("PB_HEALTH_USER")
	healthPass := os.Getenv("PB_HEALTH_PASS")
	sessionTimeout := 24 * time.Hour // Default timeout
	if timeoutStr := os.Getenv("PB_SESSION_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			sessionTimeout = timeout
		}
	}

	// Parse templates from embedded filesystem
	tmpl, err := template.New("index.tmpl").Funcs(templateFuncs).ParseFS(templateFS,
		"templates/index.tmpl",
		"templates/login.tmpl",
		"templates/styles/main.tmpl",
		"templates/scripts/main.tmpl",
		"templates/components/header.tmpl",
		"templates/components/critical_metrics.tmpl",
		"templates/components/cpu_details.tmpl",
		"templates/components/memory_details.tmpl",
		"templates/components/network_details.tmpl",
		"templates/components/visitor_analytics.tmpl",
	)
	if err != nil {
		log.Printf("Error parsing health templates: %v", err)
		return
	}

	// Health check endpoint handler
	healthHandler := func(c *core.RequestEvent) error {
		// Session-based authentication check
		cookie, err := c.Request.Cookie("pb_health_session")
		if err != nil {
			// No cookie found, redirect to login page
			return serveLoginPage(c, tmpl, "")
		}

		// Validate session
		if !isValidSession(cookie.Value) {
			return serveLoginPage(c, tmpl, "Session expired. Please login again.")
		}

		// Create a timeout context for stats collection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Collect system stats with context
		sysStats, err := monitoring.CollectSystemStats(ctx, s.stats.StartTime)
		if err != nil {
			return NewHTTPError("health_check", "Failed to collect system stats", http.StatusInternalServerError, err)
		}

		// Get analytics data if available
		var analyticsData *AnalyticsData
		if s.analytics != nil {
			analyticsData, _ = s.analytics.GetAnalyticsData()
		} else {
			analyticsData = defaultAnalyticsData()
		}

		// Prepare template data
		data := struct {
			Status           string
			UptimeDuration   string
			ServerStats      *ServerStats
			SystemStats      *monitoring.SystemStats
			AvgRequestTimeMs float64
			MemoryUsageStr   string
			DiskUsageStr     string
			LastCheckTime    time.Time
			RequestRate      float64
			AnalyticsData    *AnalyticsData
		}{
			Status:           "Healthy",
			UptimeDuration:   time.Since(s.stats.StartTime).Round(time.Second).String(),
			ServerStats:      s.stats,
			SystemStats:      sysStats,
			AvgRequestTimeMs: float64(s.stats.AverageRequestTime.Load()) / 1e6,
			MemoryUsageStr:   fmt.Sprintf("%.2f/%.2f GB", float64(sysStats.MemoryInfo.Used)/1024/1024/1024, float64(sysStats.MemoryInfo.Total)/1024/1024/1024),
			DiskUsageStr:     fmt.Sprintf("%.2f/%.2f GB", float64(sysStats.DiskUsed)/1024/1024/1024, float64(sysStats.DiskTotal)/1024/1024/1024),
			LastCheckTime:    time.Now(),
			RequestRate:      float64(s.stats.TotalRequests.Load()) / time.Since(s.stats.StartTime).Seconds(),
			AnalyticsData:    analyticsData,
		}

		// Execute template
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return NewTemplateError("health_template_execution", "Failed to execute template", err)
		}

		return c.HTML(http.StatusOK, buf.String())
	}

	// Login form submission handler
	loginHandler := func(c *core.RequestEvent) error {
		if err := c.Request.ParseForm(); err != nil {
			return serveLoginPage(c, tmpl, "Failed to parse form data")
		}

		username := c.Request.Form.Get("username")
		password := c.Request.Form.Get("password")

		// Validate credentials
		if subtle.ConstantTimeCompare([]byte(username), []byte(healthUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(healthPass)) != 1 {
			return serveLoginPage(c, tmpl, "Invalid username or password")
		}

		// Create and set session cookie
		sessionID := generateSessionID()
		http.SetCookie(c.Response, &http.Cookie{
			Name:     "pb_health_session",
			Value:    sessionID,
			Path:     "/_/_",
			HttpOnly: true,
			Secure:   c.Request.TLS != nil,
			MaxAge:   int(sessionTimeout.Seconds()),
			SameSite: http.SameSiteStrictMode,
		})

		// Store session
		storeSession(sessionID, sessionTimeout)

		// Redirect to health dashboard
		c.Response.Header().Set("Location", "/_/_")
		c.Response.WriteHeader(http.StatusSeeOther)
		return nil
	}

	// Register routes
	e.Router.GET("/_/_", healthHandler)
	e.Router.POST("/_/_/login", loginHandler)
}

// Session management functions
var activeSessions = make(map[string]time.Time)

func generateSessionID() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		// Fall back to timestamp if crypto random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.StdEncoding.EncodeToString(b)
}

func storeSession(sessionID string, timeout time.Duration) {
	activeSessions[sessionID] = time.Now().Add(timeout)
}

func isValidSession(sessionID string) bool {
	expiryTime, exists := activeSessions[sessionID]
	if !exists {
		return false
	}

	// Check if session has expired
	if time.Now().After(expiryTime) {
		delete(activeSessions, sessionID)
		return false
	}

	return true
}

// Serve login page with optional error message
func serveLoginPage(c *core.RequestEvent, tmpl *template.Template, errorMsg string) error {
	data := struct {
		ErrorMessage string
	}{
		ErrorMessage: errorMsg,
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "login.tmpl", data); err != nil {
		return NewTemplateError("login_template_execution", "Failed to execute template", err)
	}

	return c.HTML(http.StatusOK, buf.String())
}
