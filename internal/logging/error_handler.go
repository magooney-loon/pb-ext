package logging

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"

	"magooney-loon/pb-ext/internal/monitoring"
	"magooney-loon/pb-ext/internal/server"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ErrorResponse represents a standardized error response structure
type ErrorResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	Type       string `json:"type,omitempty"`
	Operation  string `json:"operation,omitempty"`
	StatusCode int    `json:"status_code"`
	TraceID    string `json:"trace_id"`
}

// SetupErrorHandler configures global error handling for the application
func SetupErrorHandler(app *pocketbase.PocketBase, e *core.ServeEvent) {
	e.Router.BindFunc(func(c *core.RequestEvent) error {
		err := c.Next()
		if err == nil {
			return nil
		}

		// Get trace ID
		traceID := c.Request.Header.Get(TraceIDHeader)

		// Determine error type and status code
		statusCode := http.StatusInternalServerError
		errorType := "internal_error"
		operation := "unknown"
		message := err.Error()

		// Handle server errors
		var srvErr *server.ServerError
		if errors.As(err, &srvErr) {
			errorType = srvErr.Type
			operation = srvErr.Op
			message = srvErr.Message
			if srvErr.StatusCode > 0 {
				statusCode = srvErr.StatusCode
			}
		}

		// Handle monitoring errors
		var monErr *monitoring.MonitoringError
		if errors.As(err, &monErr) {
			errorType = monErr.Type
			operation = monErr.Op
			message = monErr.Message
		}

		// Log error with all available context
		app.Logger().Error("Request error",
			"trace_id", traceID,
			"error_type", errorType,
			"operation", operation,
			"message", message,
			"status_code", statusCode,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)

		// Prepare standardized error response
		response := ErrorResponse{
			Status:     "error",
			Message:    message,
			Type:       errorType,
			Operation:  operation,
			StatusCode: statusCode,
			TraceID:    traceID,
		}

		// Return JSON error response
		return c.JSON(statusCode, response)
	})
}

// HandleContextErrors is a utility function to handle context-related errors
func HandleContextErrors(ctx context.Context, err error, op string) error {
	if err == nil {
		return nil
	}

	// Handle context cancellation or timeout
	if ctx.Err() != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			return monitoring.NewTimeoutError(op, "operation timed out")
		case context.Canceled:
			return monitoring.NewTimeoutError(op, "operation was canceled")
		}
	}

	// If it's already a structured error, return it as is
	var monErr *monitoring.MonitoringError
	if errors.As(err, &monErr) {
		return err
	}

	var srvErr *server.ServerError
	if errors.As(err, &srvErr) {
		return err
	}

	// Default to system error
	return monitoring.NewSystemError(op, "unexpected error occurred", err)
}

// RecoverFromPanic is a utility function to recover from panics
func RecoverFromPanic(app *pocketbase.PocketBase, c *core.RequestEvent) {
	if r := recover(); r != nil {
		traceID := c.Request.Header.Get(TraceIDHeader)

		// Log the panic
		app.Logger().Error("Panic recovered",
			"event", "panic",
			"trace_id", traceID,
			"error", r,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"stack", string(debug.Stack()),
		)

		// Return a 500 response
		_ = c.JSON(http.StatusInternalServerError, ErrorResponse{
			Status:     "error",
			Message:    "Internal server error",
			Type:       "panic",
			Operation:  "request_handler",
			StatusCode: http.StatusInternalServerError,
			TraceID:    traceID,
		})
	}
}
