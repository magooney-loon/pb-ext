package logging

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"text/template"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/magooney-loon/pb-ext/core/server"

	"github.com/pocketbase/pocketbase/core"
)

// ErrorResponse defines standardized error response
type ErrorResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	Type       string `json:"type,omitempty"`
	Operation  string `json:"operation,omitempty"`
	StatusCode int    `json:"status_code"`
	TraceID    string `json:"trace_id"`
	Timestamp  string `json:"timestamp"`
}

// SetupErrorHandler configures global error handling
func SetupErrorHandler(app core.App, e *core.ServeEvent) {
	// Parse error template
	tmpl, err := template.ParseFS(server.TemplateFS, "templates/error.tmpl")
	if err != nil {
		app.Logger().Error("Failed to parse error template", "error", err)
	}

	e.Router.BindFunc(func(c *core.RequestEvent) error {
		err := c.Next()
		if err == nil {
			return nil
		}

		traceID := c.Request.Header.Get(TraceIDHeader)

		statusCode := http.StatusInternalServerError
		errorType := "internal_error"
		operation := "unknown"
		message := err.Error()

		var srvErr *server.ServerError
		if errors.As(err, &srvErr) {
			errorType = srvErr.Type
			operation = srvErr.Op
			message = srvErr.Message
			if srvErr.StatusCode > 0 {
				statusCode = srvErr.StatusCode
			}
		}

		var monErr *monitoring.MonitoringError
		if errors.As(err, &monErr) {
			errorType = monErr.Type
			operation = monErr.Op
			message = monErr.Message
		}

		// Skip error logging for service worker and favicon requests
		if !shouldExcludeFromLogging(c.Request.URL.Path) {
			app.Logger().Error("Request error",
				"trace_id", traceID,
				"error_type", errorType,
				"operation", operation,
				"message", message,
				"status_code", statusCode,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			)
		}

		response := ErrorResponse{
			Status:     http.StatusText(statusCode),
			Message:    message,
			Type:       errorType,
			Operation:  operation,
			StatusCode: statusCode,
			TraceID:    traceID,
			Timestamp:  time.Now().Format(time.RFC3339),
		}

		// Check if client accepts HTML
		accept := c.Request.Header.Get("Accept")
		userAgent := c.Request.Header.Get("User-Agent")
		isBrowser := strings.Contains(strings.ToLower(userAgent), "mozilla") ||
			strings.Contains(strings.ToLower(userAgent), "chrome") ||
			strings.Contains(strings.ToLower(userAgent), "safari") ||
			strings.Contains(strings.ToLower(userAgent), "firefox")

		app.Logger().Debug("Error response details",
			"accept", accept,
			"user_agent", userAgent,
			"is_browser", isBrowser,
		)

		// Return HTML for browsers or when explicitly requested
		if isBrowser || strings.Contains(strings.ToLower(accept), "text/html") {
			// Return HTML error page
			if tmpl != nil {
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, response); err == nil {
					app.Logger().Debug("Serving HTML error page")
					return c.HTML(statusCode, buf.String())
				} else {
					app.Logger().Error("Failed to execute error template", "error", err)
				}
			} else {
				app.Logger().Error("Error template is nil")
			}
		}

		// Return JSON response
		app.Logger().Debug("Serving JSON error response")
		return c.JSON(statusCode, response)
	})
}

// HandleContextErrors handles context-related errors
func HandleContextErrors(ctx context.Context, err error, op string) error {
	if err == nil {
		return nil
	}

	if ctx != nil && ctx.Err() != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			return monitoring.NewTimeoutError(op, "operation timed out")
		case context.Canceled:
			return monitoring.NewTimeoutError(op, "operation was canceled")
		}
	}

	var monErr *monitoring.MonitoringError
	if errors.As(err, &monErr) {
		return err
	}

	var srvErr *server.ServerError
	if errors.As(err, &srvErr) {
		return err
	}

	return monitoring.NewSystemError(op, "unexpected error occurred", err)
}

// RecoverFromPanic recovers from panics and returns a 500 response
func RecoverFromPanic(app core.App, c *core.RequestEvent) {
	if r := recover(); r != nil {
		traceID := c.Request.Header.Get(TraceIDHeader)

		// Skip panic logging for service worker and favicon requests
		if !shouldExcludeFromLogging(c.Request.URL.Path) {
			app.Logger().Error("Panic recovered",
				"event", "panic",
				"trace_id", traceID,
				"error", r,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"stack", string(debug.Stack()),
			)
		}

		response := ErrorResponse{
			Status:     "Internal Server Error",
			Message:    "A panic occurred while processing your request",
			Type:       "panic",
			Operation:  "request_handler",
			StatusCode: http.StatusInternalServerError,
			TraceID:    traceID,
			Timestamp:  time.Now().Format(time.RFC3339),
		}

		// Check if client accepts HTML
		accept := c.Request.Header.Get("Accept")
		userAgent := c.Request.Header.Get("User-Agent")
		isBrowser := strings.Contains(strings.ToLower(userAgent), "mozilla") ||
			strings.Contains(strings.ToLower(userAgent), "chrome") ||
			strings.Contains(strings.ToLower(userAgent), "safari") ||
			strings.Contains(strings.ToLower(userAgent), "firefox")

		app.Logger().Debug("Panic response details",
			"accept", accept,
			"user_agent", userAgent,
			"is_browser", isBrowser,
		)

		// Return HTML for browsers or when explicitly requested
		if isBrowser || strings.Contains(strings.ToLower(accept), "text/html") {
			// Return HTML error page
			if tmpl, err := template.ParseFS(server.TemplateFS, "templates/error.tmpl"); err == nil {
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, response); err == nil {
					app.Logger().Debug("Serving HTML error page for panic")
					_ = c.HTML(http.StatusInternalServerError, buf.String())
					return
				} else {
					app.Logger().Error("Failed to execute error template for panic", "error", err)
				}
			} else {
				app.Logger().Error("Failed to parse error template for panic", "error", err)
			}
		}

		// Return JSON response
		app.Logger().Debug("Serving JSON error response for panic")
		_ = c.JSON(http.StatusInternalServerError, response)
	}
}
