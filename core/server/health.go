package server

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/spf13/cast"

	"github.com/pocketbase/pocketbase/core"
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
	"avgCPUUsage": averageCPUUsage,
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
	// The temperature helpers read the already-collected readings rather than
	// re-reading sensors: each of these is called several times per render, and
	// hitting the hwmon sysfs tree on every call is both slow and liable to
	// disagree with what the collector classified.
	"getDiskTemp": func(stats *monitoring.SystemStats) float64 {
		if stats == nil {
			return 0
		}
		return stats.Temperatures.DiskTemp
	},
	"getSystemTemp": func(stats *monitoring.SystemStats) float64 {
		if stats == nil {
			return 0
		}
		return stats.Temperatures.SystemTemp
	},
	"getAmbientTemp": func(stats *monitoring.SystemStats) float64 {
		if stats == nil {
			return 0
		}
		return stats.Temperatures.AmbientTemp
	},
	"getCPUTemp": func(stats *monitoring.SystemStats) float64 {
		if stats == nil {
			return 0
		}
		// Prefer the per-CPU reading, falling back to the classified sensor.
		for _, c := range stats.CPUInfo {
			if c.Temperature > 0 {
				return c.Temperature
			}
		}
		return stats.Temperatures.CPUTemp
	},
	"hasDiskTemps": func(stats *monitoring.SystemStats) bool {
		return stats != nil && stats.Temperatures.DiskTemp > 0
	},
	"formatTime": func(t time.Time) string {
		return t.Format("15:04:05")
	},
	// escapeHTML is mandatory for anything that did not originate in this
	// repository. The dashboard renders with text/template, which does not
	// escape anything, and alert titles and delivery errors carry error strings
	// that can quote a request path or a panic value — i.e. attacker-influenced
	// text — straight into the page.
	"escapeHTML": html.EscapeString,
	"inc": func(i int) int {
		return i + 1
	},
	// formatCount renders a plain count with thousands separators. Counts must
	// never be divided by 1024/1048576 and labelled as bytes.
	"formatCount": func(n uint64) string {
		s := strconv.FormatUint(n, 10)
		if len(s) <= 3 {
			return s
		}

		var b strings.Builder
		lead := len(s) % 3
		if lead > 0 {
			b.WriteString(s[:lead])
		}
		for i := lead; i < len(s); i += 3 {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(s[i : i+3])
		}
		return b.String()
	},
	// percentOf works on ints, unlike "divide" which only accepts float64/uint64.
	"percentOf": func(part, total int) float64 {
		if total == 0 {
			return 0
		}
		return float64(part) / float64(total) * 100
	},
	// pathLabel renders the analytics overflow bucket readably. Paths beyond the
	// per-day cardinality budget are collapsed into it, so it would otherwise
	// show up as a bare "/*".
	"pathLabel": func(path string) string {
		if path == analytics.OverflowPath {
			return "other pages"
		}
		return path
	},
	"isset": func(c interface{}, key interface{}) (bool, error) {
		// This code taken from:
		//   https://github.com/gohugoio/hugo/blob/e9bda21ce9d1ab80377044d8de1d7884142bfa14/tpl/collections/collections.go#L332
		// Thanks GoHugo
		if c == nil {
			return false, nil
		}

		av := reflect.ValueOf(c)
		kv := reflect.ValueOf(key)

		// Unwrap pointers so isset works on *SystemStats and friends.
		for av.Kind() == reflect.Ptr {
			if av.IsNil() {
				return false, nil
			}
			av = av.Elem()
		}

		switch av.Kind() {
		case reflect.Struct:
			// Without this a struct falls through to the default and always
			// reports false, which silently hides whatever the template was
			// guarding. Report whether the named field actually exists.
			name, err := cast.ToStringE(key)
			if err != nil {
				return false, fmt.Errorf("isset unable to use key of type %T as a field name", key)
			}
			return av.FieldByName(name).IsValid(), nil
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

// averageCPUUsage is the mean usage across cores, guarded so a collection that
// returned no entries renders 0 rather than NaN. Shared by the dashboard
// template and the alert rules so both read the same figure.
func averageCPUUsage(cpus []monitoring.CPUInfo) float64 {
	if len(cpus) == 0 {
		return 0
	}
	var total float64
	for _, cpu := range cpus {
		total += cpu.Usage
	}
	return total / float64(len(cpus))
}

// DashboardData is the payload the /_/_ dashboard templates render.
//
// It is a named type so tests can render the full page against deliberately
// degraded data — hosts without sensors, containers with no addressed network
// interfaces, a stats collection that partly failed — and prove none of it
// panics or renders NaN.
type DashboardData struct {
	Status           string
	UptimeDuration   string
	ServerStats      *ServerStats
	SystemStats      *monitoring.SystemStats
	AvgRequestTimeMs float64
	MemoryUsageStr   string
	DiskUsageStr     string
	LastCheckTime    time.Time
	RequestRate      float64
	AnalyticsData    *analytics.Data
	AlertsData       *alerts.Data
	AuditData        *audit.Data
	PBAdminURL       string
}

// requestRate returns requests per second, guarding the division so a dashboard
// hit in the same instant the server started cannot render +Inf or NaN.
func requestRate(total uint64, uptime time.Duration) float64 {
	seconds := uptime.Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(total) / seconds
}

// prepareTemplateData prepares the template data for the health dashboard
func (s *Server) prepareTemplateData() (interface{}, error) {
	// Create a timeout context for stats collection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect system stats with context
	sysStats, err := monitoring.CollectSystemStats(ctx, s.stats.StartTime, s.app.DataDir())
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
	var analyticsData *analytics.Data
	if s.analytics != nil {
		analyticsData, _ = s.analytics.GetData()
	} else {
		analyticsData = analytics.DefaultData()
	}

	// Data is nil-safe, so an uninitialised notifier renders the card as
	// "not configured" rather than taking the dashboard down.
	alertsData := s.alerts.Data()
	auditData := s.auditor.Data()

	// Prepare template data
	data := DashboardData{
		Status:           "Healthy",
		UptimeDuration:   time.Since(s.stats.StartTime).Round(time.Second).String(),
		ServerStats:      s.stats,
		SystemStats:      sysStats,
		AvgRequestTimeMs: float64(s.stats.AverageRequestTime.Load()) / 1e6,
		MemoryUsageStr:   fmt.Sprintf("%.2f/%.2f GB", float64(sysStats.MemoryInfo.Used)/1024/1024/1024, float64(sysStats.MemoryInfo.Total)/1024/1024/1024),
		DiskUsageStr:     fmt.Sprintf("%.2f/%.2f GB", float64(sysStats.DiskUsed)/1024/1024/1024, float64(sysStats.DiskTotal)/1024/1024/1024),
		LastCheckTime:    time.Now(),
		RequestRate:      requestRate(s.stats.TotalRequests.Load(), time.Since(s.stats.StartTime)),
		AnalyticsData:    analyticsData,
		AlertsData:       alertsData,
		AuditData:        auditData,
		PBAdminURL:       "/_/",
	}

	return data, nil
}

// parseDashboardTemplates discovers and parses every embedded .tmpl with the
// dashboard's function map. Kept separate from route registration so tests can
// exercise the exact same parse, which is otherwise only done at runtime where
// a missing template func would just be logged.
func parseDashboardTemplates() (*template.Template, error) {
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
		return nil, fmt.Errorf("discovering templates: %w", err)
	}

	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(TemplateFS, templateFiles...)
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return tmpl, nil
}

// recoverDashboardPanic converts a panic in the dashboard handler into an
// error response, so a failure to read one metric can never take down the
// request-handling goroutine.
func recoverDashboardPanic(s *Server, next func(*core.RequestEvent) error) func(*core.RequestEvent) error {
	return func(c *core.RequestEvent) (err error) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			stack := string(debug.Stack())
			if s != nil && s.App() != nil {
				s.App().Logger().Error("Recovered panic while rendering the dashboard",
					"panic", fmt.Sprint(r),
					"stack", stack,
				)
			} else {
				log.Printf("Recovered panic while rendering the dashboard: %v\n%s", r, stack)
			}

			err = NewHTTPError(
				"health_dashboard_panic",
				"Failed to render the dashboard",
				http.StatusInternalServerError,
				fmt.Errorf("panic: %v", r),
			)
		}()

		return next(c)
	}
}

// RegisterHealthRoute registers the health check endpoint
func (s *Server) RegisterHealthRoute(e *core.ServeEvent) {
	tmpl, err := parseDashboardTemplates()
	if err != nil {
		log.Printf("Error preparing health templates: %v", err)
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

	// Register the main health route. The dashboard reads a lot of optional,
	// platform-dependent system data; recovering here means a surprise from a
	// collector or a template degrades into a 500 for this one request instead
	// of unwinding the handler. It does not rely on the embedding app having
	// opted into logging.SetupRecovery.
	e.Router.GET("/_/_", recoverDashboardPanic(s, healthHandler))

}
