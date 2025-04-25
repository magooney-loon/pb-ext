package logging

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"magooney-loon/pb-ext-dash/internal/monitoring"
	"magooney-loon/pb-ext-dash/internal/server"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// LogLevel represents different log levels
type LogLevel int

const (
	// Debug level for detailed information
	Debug LogLevel = -4
	// Info level for general information
	Info LogLevel = 0
	// Warn level for warning messages
	Warn LogLevel = 4
	// Error level for error messages
	Error LogLevel = 8

	// TraceIDHeader is the header key for trace ID
	TraceIDHeader = "X-Trace-ID"
)

// String converts the log level to a string
func (l LogLevel) String() string {
	switch l {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL_%d", l)
	}
}

// LogContext holds contextual information for logging
type LogContext struct {
	TraceID    string
	StartTime  time.Time
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	UserAgent  string
	IP         string
}

// SetupLogging configures logging for the application
func SetupLogging(srv *server.Server) {
	app := srv.App()
	requestStats := monitoring.NewRequestStats()

	// Log application startup with structured fields
	app.Logger().Info("Application starting up",
		"event", "app_startup",
		"time", time.Now().Format(time.RFC3339),
		"pid", os.Getpid(),
	)

	// Handle graceful shutdown
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		app.Logger().Info("Application shutting down",
			"event", "app_shutdown",
			"time", time.Now().Format(time.RFC3339),
			"is_restart", e.IsRestart,
			"uptime", time.Since(srv.Stats().StartTime).Round(time.Second).String(),
			"total_requests", srv.Stats().TotalRequests.Load(),
			"avg_request_time_ms", srv.Stats().AverageRequestTime.Load(),
		)
		return e.Next()
	})

	// Add request logging middleware
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Setup global error handler first
		SetupErrorHandler(app, e)

		e.Router.BindFunc(func(c *core.RequestEvent) error {
			// Setup panic recovery
			defer func() {
				RecoverFromPanic(app, c)
			}()

			// Generate trace ID
			traceID := uuid.New().String()
			c.Request.Header.Set(TraceIDHeader, traceID)
			c.Response.Header().Set(TraceIDHeader, traceID)

			start := time.Now()

			// Update active connections
			srv.Stats().ActiveConnections.Add(1)
			defer srv.Stats().ActiveConnections.Add(-1)

			// Execute the next handler
			err := c.Next()

			duration := time.Since(start)

			// Get status code
			statusCode := http.StatusOK
			if status := c.Response.Header().Get("Status"); status != "" {
				if code, err := strconv.Atoi(status); err == nil {
					statusCode = code
				}
			}

			// Create log context
			logCtx := LogContext{
				TraceID:    traceID,
				StartTime:  start,
				Method:     c.Request.Method,
				Path:       c.Request.URL.Path,
				StatusCode: statusCode,
				Duration:   duration,
				UserAgent:  c.Request.UserAgent(),
				IP:         c.Request.RemoteAddr,
			}

			// Update server stats
			stats := srv.Stats()
			stats.TotalRequests.Add(1)
			stats.LastRequestTime.Store(time.Now().Unix())
			if statusCode >= 400 {
				stats.TotalErrors.Add(1)
			}

			// Update average request time
			currentAvg := float64(stats.AverageRequestTime.Load())
			totalReqs := stats.TotalRequests.Load()
			if totalReqs > 1 {
				newAvg := ((currentAvg * float64(totalReqs-1)) + duration.Seconds()*1000) / float64(totalReqs)
				stats.AverageRequestTime.Store(int64(newAvg))
			} else {
				stats.AverageRequestTime.Store(int64(duration.Seconds() * 1000))
			}

			// Track request metrics
			metrics := monitoring.RequestMetrics{
				Path:          logCtx.Path,
				Method:        logCtx.Method,
				StatusCode:    logCtx.StatusCode,
				Duration:      logCtx.Duration,
				Timestamp:     logCtx.StartTime,
				UserAgent:     logCtx.UserAgent,
				ContentLength: c.Request.ContentLength,
				RemoteAddr:    logCtx.IP,
			}
			requestStats.TrackRequest(metrics)

			// Log the error with structure if present
			if err != nil {
				// Note: Error handling is done in SetupErrorHandler
				return err
			}

			// Log request with structured fields
			app.Logger().Debug("Request processed",
				"event", "http_request",
				"trace_id", logCtx.TraceID,
				"method", logCtx.Method,
				"path", logCtx.Path,
				"status", fmt.Sprintf("%d [%s]", logCtx.StatusCode, monitoring.GetStatusString(logCtx.StatusCode)),
				"duration", monitoring.FormatDuration(duration),
				"ip", logCtx.IP,
				"user_agent", logCtx.UserAgent,
				"content_length", c.Request.ContentLength,
				"request_rate", requestStats.GetRequestRate(),
			)

			return nil
		})
		return e.Next()
	})
}

// SetupRecovery configures panic recovery for the application
func SetupRecovery(app *pocketbase.PocketBase, e *core.ServeEvent) {
	app.Logger().Info("Server recovery starting",
		"event", "recovery_setup",
		"time", time.Now().Format(time.RFC3339),
	)

	e.Router.BindFunc(func(c *core.RequestEvent) error {
		defer func() {
			RecoverFromPanic(app, c)
		}()
		return c.Next()
	})
}
