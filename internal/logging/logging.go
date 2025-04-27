package logging

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"magooney-loon/pb-ext/internal/monitoring"
	"magooney-loon/pb-ext/internal/server"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// LogLevel represents different log levels
type LogLevel int

const (
	Debug LogLevel = -4 // Debug level
	Info  LogLevel = 0  // Info level
	Warn  LogLevel = 4  // Warning level
	Error LogLevel = 8  // Error level

	TraceIDHeader = "X-Trace-ID"
)

// String converts log level to string
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

// SetupLogging configures logging
func SetupLogging(srv *server.Server) {
	app := srv.App()
	requestStats := monitoring.NewRequestStats()

	app.Logger().Info("Application starting up",
		"event", "app_startup",
		"time", time.Now().Format(time.RFC3339),
		"pid", os.Getpid(),
	)

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

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		SetupErrorHandler(app, e)

		e.Router.BindFunc(func(c *core.RequestEvent) error {
			defer func() {
				RecoverFromPanic(app, c)
			}()

			traceID := uuid.New().String()
			c.Request.Header.Set(TraceIDHeader, traceID)
			c.Response.Header().Set(TraceIDHeader, traceID)

			start := time.Now()

			srv.Stats().ActiveConnections.Add(1)
			defer srv.Stats().ActiveConnections.Add(-1)

			err := c.Next()

			duration := time.Since(start)

			statusCode := http.StatusOK
			if status := c.Response.Header().Get("Status"); status != "" {
				if code, err := strconv.Atoi(status); err == nil {
					statusCode = code
				}
			}

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

			stats := srv.Stats()
			stats.TotalRequests.Add(1)
			stats.LastRequestTime.Store(time.Now().Unix())
			if statusCode >= 400 {
				stats.TotalErrors.Add(1)
			}

			currentAvg := float64(stats.AverageRequestTime.Load())
			totalReqs := stats.TotalRequests.Load()
			if totalReqs > 1 {
				newAvg := ((currentAvg * float64(totalReqs-1)) + duration.Seconds()*1000) / float64(totalReqs)
				stats.AverageRequestTime.Store(int64(newAvg))
			} else {
				stats.AverageRequestTime.Store(int64(duration.Seconds() * 1000))
			}

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

			if err != nil {
				return err
			}

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

// SetupRecovery configures panic recovery
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
