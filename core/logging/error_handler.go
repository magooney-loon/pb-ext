package logging

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/magooney-loon/pb-ext/core/server"

	"github.com/pocketbase/pocketbase"
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
}

// SetupErrorHandler configures global error handling
func SetupErrorHandler(app *pocketbase.PocketBase, e *core.ServeEvent) {
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

		app.Logger().Error("Request error",
			"trace_id", traceID,
			"error_type", errorType,
			"operation", operation,
			"message", message,
			"status_code", statusCode,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)

		response := ErrorResponse{
			Status:     "error",
			Message:    message,
			Type:       errorType,
			Operation:  operation,
			StatusCode: statusCode,
			TraceID:    traceID,
		}

		return c.JSON(statusCode, response)
	})
}

// HandleContextErrors handles context-related errors
func HandleContextErrors(ctx context.Context, err error, op string) error {
	if err == nil {
		return nil
	}

	if ctx.Err() != nil {
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
func RecoverFromPanic(app *pocketbase.PocketBase, c *core.RequestEvent) {
	if r := recover(); r != nil {
		traceID := c.Request.Header.Get(TraceIDHeader)

		app.Logger().Error("Panic recovered",
			"event", "panic",
			"trace_id", traceID,
			"error", r,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"stack", string(debug.Stack()),
		)

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
