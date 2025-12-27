package server

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/spf13/cast"

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
	"divide": func(a, b any) float64 {
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
	"isset": func(c interface{}, key interface{}) (bool, error) {
		// This code taken from:
		//   https://github.com/gohugoio/hugo/blob/e9bda21ce9d1ab80377044d8de1d7884142bfa14/tpl/collections/collections.go#L332
		// Thanks GoHugo
		av := reflect.ValueOf(c)
		kv := reflect.ValueOf(key)

		switch av.Kind() {
		case reflect.Array, reflect.Chan, reflect.Slice:
			k, err := cast.ToIntE(key)
			if err != nil {
				return false, fmt.Errorf("isset unable to use key of type %T as index", key)
			}
			if av.Len() > k {
				return true, nil
			}
		case reflect.Map:
			if kv.Type() == av.Type().Key() {
				return av.MapIndex(kv).IsValid(), nil
			}
		default:
			//ns.deps.Log.Warnf("calling IsSet with unsupported type %q (%T) will always return false.\n", av.Kind(), c)
		}

		return false, nil
	},
}

// prepareTemplateData prepares the template data for the health dashboard
func (s *Server) prepareTemplateData() (interface{}, error) {
	// Create a timeout context for stats collection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect system stats with context
	sysStats, err := monitoring.CollectSystemStats(ctx, s.stats.StartTime)
	if err != nil {
		if errs, ok := err.(interface{ Unwrap() []error }); ok {
			for _, k := range errs.Unwrap() {
				s.App().Logger().Warn("Failed to collect system stats", "error", k)
			}
		}
	}
	if sysStats == nil {
		return nil, err
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
		PBAdminURL       string
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
		PBAdminURL:       "/_/",
	}

	return data, nil
}

// RegisterHealthRoute registers the health check endpoint
func (s *Server) RegisterHealthRoute(e *core.ServeEvent) {
	// Automatically discover and parse all templates from embedded filesystem
	var templateFiles []string

	err := fs.WalkDir(TemplateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only include .tmpl files
		if !d.IsDir() && filepath.Ext(path) == ".tmpl" {
			templateFiles = append(templateFiles, path)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error discovering templates: %v", err)
		return
	}

	// Parse all discovered templates
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(TemplateFS, templateFiles...)
	if err != nil {
		log.Printf("Error parsing health templates: %v", err)
		return
	}

	// Health check endpoint handler
	healthHandler := func(c *core.RequestEvent) error {
		// If not already authenticated, show login template
		if c.Auth == nil || !c.Auth.IsSuperuser() {
			// Prepare login template data
			loginData := struct {
				PBAdminURL string
			}{
				PBAdminURL: "/_/",
			}

			// Execute login template
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, "login.tmpl", loginData); err != nil {
				log.Printf("Error executing login template: %v", err)
				return NewTemplateError("login_template_execution", "Failed to execute login template", err)
			}

			return c.HTML(http.StatusOK, buf.String())
		}

		// User is authenticated, show the dashboard
		// Prepare template data using the extracted method
		data, err := s.prepareTemplateData()
		if err != nil {
			return NewHTTPError("health_check", "Failed to collect system stats", http.StatusInternalServerError, err)
		}

		// Execute dashboard template
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "index.tmpl", data); err != nil {
			return NewTemplateError("health_template_execution", "Failed to execute template", err)
		}

		return c.HTML(http.StatusOK, buf.String())
	}

	// Register the main health route
	e.Router.GET("/_/_", healthHandler)

}
