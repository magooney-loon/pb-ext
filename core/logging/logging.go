package logging

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/magooney-loon/pb-ext/core/server"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

// LogLevel represents different log levels
type LogLevel int

const (
	Debug LogLevel = -4 // Debug level
	Info  LogLevel = 0  // Info level
	Warn  LogLevel = 4  // Warning level
	Error LogLevel = 8  // Error level

	TraceIDHeader = "X-Trace-ID"
	RequestIDKey  = "request_id"
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

// shouldExcludeFromLogging returns true if the path should be excluded from logging
func shouldExcludeFromLogging(path string) bool {
	return path == "/service-worker.js" || path == "/favicon.ico" || path == "/manifest.json"
}

// InfoWithContext logs an info message with context data using PocketBase's logger
func InfoWithContext(ctx context.Context, app core.App, message string, data map[string]interface{}) {
	logger := app.Logger()

	// Add request ID if available
	if ctx != nil {
		if id, ok := ctx.Value(RequestIDKey).(string); ok {
			logger = logger.With("request_id", id)
		}
	}

	// Create a new logger with all the data fields
	for key, value := range data {
		logger = logger.With(key, value)
	}

	logger.Info(message)
}

// ErrorWithContext logs an error message with context data using PocketBase's logger
func ErrorWithContext(ctx context.Context, app core.App, message string, err error, data map[string]any) {
	logger := app.Logger()

	// Add request ID if available
	if ctx != nil {
		if id, ok := ctx.Value(RequestIDKey).(string); ok {
			logger = logger.With("request_id", id)
		}
	}

	// Add error information
	if err != nil {
		logger = logger.With("error", err.Error())
	}

	// Add all data fields
	for key, value := range data {
		logger = logger.With(key, value)
	}

	logger.Error(message)
}

// SetupLogging configures logging using PocketBase's logger
func SetupLogging(srv *server.Server) {
	app := srv.App()
	requestStats := monitoring.NewRequestStats()

	// Create a logger with common application fields
	appLogger := app.Logger().With(
		"pid", os.Getpid(),
		"start_time", time.Now().Format(time.RFC3339),
	)

	appLogger.Info("Application starting up",
		"event", "app_startup",
	)

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		appLogger.Info("Application shutting down",
			"event", "app_shutdown",
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

			traceID := security.RandomString(18)
			c.Request.Header.Set(TraceIDHeader, traceID)
			c.Response.Header().Set(TraceIDHeader, traceID)

			start := time.Now()

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

			// Stats tracking is handled by server.go to avoid duplication

			// Skip metrics tracking for service worker and favicon requests
			if !shouldExcludeFromLogging(logCtx.Path) {
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
			}

			if err != nil {
				return err
			}

			// Skip logging for service worker and favicon requests
			if !shouldExcludeFromLogging(logCtx.Path) {
				// Create a request-specific logger with all request context
				requestLogger := app.Logger().WithGroup("request").With(
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

				requestLogger.Debug("Request processed",
					"event", "http_request",
				)
			}

			return nil
		})
		return e.Next()
	})
}

// SetupRecovery configures panic recovery
func SetupRecovery(app core.App, e *core.ServeEvent) {
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
