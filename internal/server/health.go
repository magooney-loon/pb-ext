package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"magooney-loon/pb-ext-dash/internal/monitoring"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shirou/gopsutil/v3/host"
)

// HealthResponse represents the health check response structure
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

		// Get temperatures and find disk temperature
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

		// Get temperatures and find system temperature
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

		// Get temperatures and find ambient temperature
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
}

// RegisterHealthRoute registers the health check endpoint
func (s *Server) RegisterHealthRoute(e *core.ServeEvent) {
	// Determine base path for template files
	basePath := "."
	// Check if we're running in development or production mode
	if _, err := os.Stat("internal/server/templates"); err == nil {
		basePath = "."
	} else if _, err := os.Stat("pb_public/templates"); err == nil {
		basePath = "pb_public"
	}

	// Define template paths
	templatePaths := []string{
		filepath.Join(basePath, "internal/server/templates/health.tmpl"),
		filepath.Join(basePath, "internal/server/templates/styles/main.tmpl"),
		filepath.Join(basePath, "internal/server/templates/scripts/main.tmpl"),
		filepath.Join(basePath, "internal/server/templates/components/header.tmpl"),
		filepath.Join(basePath, "internal/server/templates/components/critical_metrics.tmpl"),
		filepath.Join(basePath, "internal/server/templates/components/cpu_details.tmpl"),
		filepath.Join(basePath, "internal/server/templates/components/memory_details.tmpl"),
		filepath.Join(basePath, "internal/server/templates/components/network_details.tmpl"),
	}

	// Parse all templates with functions
	tmpl, err := template.New("health.tmpl").Funcs(templateFuncs).ParseFiles(templatePaths...)
	if err != nil {
		log.Printf("Error parsing health templates: %v", err)
		return
	}

	handler := func(c *core.RequestEvent) error {
		// Create a timeout context for stats collection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Collect system stats with context
		sysStats, err := monitoring.CollectSystemStats(ctx, s.stats.StartTime)
		if err != nil {
			return NewHTTPError("health_check", "Failed to collect system stats", http.StatusInternalServerError, err)
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
		}

		// Execute template
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return NewTemplateError("health_template_execution", "Failed to execute template", err)
		}

		return c.HTML(http.StatusOK, buf.String())
	}

	// Register route without auth
	e.Router.GET("/_/_", handler)
}
