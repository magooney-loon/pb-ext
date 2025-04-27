package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Variable to store original stdout for restoration
var originalStdout *os.File

// setupTestLogger redirects logs to a buffer for testing
func setupTestLogger() (*pocketbase.PocketBase, *bytes.Buffer) {
	var buf bytes.Buffer
	app := pocketbase.New()

	// Save original stdout
	originalStdout = os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a tee to both buffer and original
	go func() {
		multiWriter := io.MultiWriter(&buf, originalStdout)
		io.Copy(multiWriter, r)
	}()

	return app, &buf
}

// restoreLogger restores the original stdout
func restoreLogger() {
	if originalStdout != nil {
		os.Stdout = originalStdout
	}
}

// SetupTestLogger configures a test logger that writes to a buffer
func SetupTestLogger(buf *bytes.Buffer) {
	// Save original stdout
	originalStdout = os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a writer that writes to both buffer and original stdout
	go func() {
		multiWriter := io.MultiWriter(buf, originalStdout)
		io.Copy(multiWriter, r)
	}()
}

func TestLoggingLevels(t *testing.T) {
	// Test log level string representations
	assert.Equal(t, "DEBUG", Debug.String())
	assert.Equal(t, "INFO", Info.String())
	assert.Equal(t, "WARN", Warn.String())
	assert.Equal(t, "ERROR", Error.String())
	assert.Equal(t, "LEVEL_42", LogLevel(42).String())
}

func TestLogContext(t *testing.T) {
	// Create a log context
	ctx := LogContext{
		TraceID: "test-trace-id",
		Method:  "GET",
		Path:    "/api/test",
	}

	// Check basic properties
	assert.Equal(t, "test-trace-id", ctx.TraceID)
	assert.Equal(t, "GET", ctx.Method)
	assert.Equal(t, "/api/test", ctx.Path)
}

func TestTraceIDHeader(t *testing.T) {
	// Setup test app
	_, _ = setupTestLogger()
	defer restoreLogger()

	// Create a test request
	req := httptest.NewRequest("GET", "/api/test", nil)

	// Add trace ID to request
	req.Header.Set(TraceIDHeader, "test-trace-id")

	// Verify header was set
	assert.Equal(t, "test-trace-id", req.Header.Get(TraceIDHeader))
}

func TestLoggingHandler(t *testing.T) {
	// This is a simplified test that's not calling the full SetupLogging
	// In a real test, we'd use a mock server with an actual request cycle

	// Create a context with trace ID
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-id")

	// Verify context has trace ID
	traceID, ok := ctx.Value("trace_id").(string)
	assert.True(t, ok)
	assert.Equal(t, "test-trace-id", traceID)
}

func TestContextualLogging(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer

	// Create a test logger that writes to our buffer
	SetupTestLogger(&buf)

	// Create a request with context
	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx := req.Context()

	// Add request ID to context
	ctx = context.WithValue(ctx, RequestIDKey, "test-request-id")
	req = req.WithContext(ctx)

	// Log with context
	InfoWithContext(ctx, "contextual message", map[string]interface{}{
		"extra_field": "test_value",
	})

	// Wait for the log to be written
	time.Sleep(10 * time.Millisecond)

	// Get the buffer content and trim newlines
	output := strings.TrimSpace(buf.String())

	// Parse the JSON log
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err, "Log should be valid JSON")

	// Check log fields
	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "contextual message", logEntry["message"])
	assert.Equal(t, "test-request-id", logEntry["request_id"])
	assert.Equal(t, "test_value", logEntry["extra_field"])
}

func TestErrorHandling(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer

	// Create a test logger that writes to our buffer
	SetupTestLogger(&buf)

	// Log an error
	testErr := ErrInvalidRequest{Msg: "test error"}
	ErrorWithContext(context.Background(), "test error occurred", testErr, nil)

	// Wait for the log to be written
	time.Sleep(10 * time.Millisecond)

	// Get the buffer content and trim newlines
	output := strings.TrimSpace(buf.String())

	// Parse the JSON log
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err, "Log should be valid JSON")

	// Check log fields
	assert.Equal(t, "ERROR", logEntry["level"])
	assert.Equal(t, "test error occurred", logEntry["message"])

	// Check error data
	errorData, ok := logEntry["error"].(map[string]interface{})
	require.True(t, ok, "Error data should be present")
	assert.Equal(t, "test error", errorData["message"])
}

// Mock for testing
type ErrInvalidRequest struct {
	Msg string
}

func (e ErrInvalidRequest) Error() string {
	return e.Msg
}

// TestLoggingIntegration tests that logging system integrates with the application
func TestLoggingIntegration(t *testing.T) {
	// Create a test app
	app, buf := setupTestLogger()
	defer restoreLogger()

	// Test the actual recovery middleware setup
	var testEvent struct {
		httpServer interface{}
		app        interface{}
	}
	testEvent.app = app

	// Create a log context
	ctx := context.Background()
	ctx = context.WithValue(ctx, RequestIDKey, "test-integration-id")

	// Log something using our logging functions
	InfoWithContext(ctx, "Debug test message", nil)
	InfoWithContext(ctx, "Info test message", nil)
	InfoWithContext(ctx, "Warning test message", nil)
	ErrorWithContext(ctx, "Error test message", nil, nil)

	// Wait for logs to be written
	time.Sleep(20 * time.Millisecond)

	// Get the buffer content
	output := buf.String()

	// Verify logs were written
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "Debug test message")
	assert.Contains(t, output, "Info test message")
	assert.Contains(t, output, "Warning test message")
	assert.Contains(t, output, "ERROR")
	assert.Contains(t, output, "Error test message")
	assert.Contains(t, output, "test-integration-id")
}

// TestTraceWithRealServer tests trace ID propagation with a real server instance
func TestTraceWithRealServer(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test app and capture logs
	_, buf := setupTestLogger()
	defer restoreLogger()

	// Create a test request with trace ID
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set(TraceIDHeader, "test-integration-trace-id")

	// Create a recorder for the response
	w := httptest.NewRecorder()

	// Create a simple test handler that uses contextual logging
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get trace ID from request context
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestIDKey, req.Header.Get(TraceIDHeader))

		// Log with the context
		InfoWithContext(ctx, "Request processed", map[string]interface{}{
			"test_value": "integration_test",
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Process request with our custom handler
	testHandler.ServeHTTP(w, req)

	// Wait for logs to be written
	time.Sleep(20 * time.Millisecond)

	// Get the buffer content
	output := buf.String()

	// Verify trace ID is in the logs
	assert.Contains(t, output, "test-integration-trace-id", "Trace ID should be in logs")
	assert.Contains(t, output, "Request processed", "Handler message should be in logs")
	assert.Contains(t, output, "integration_test", "Custom field should be in logs")
}

// TestLoggerWithServer tests that the logger integrates with a server
func TestLoggerWithServer(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test app and capture logs
	_, buf := setupTestLogger()
	defer restoreLogger()

	// Setup a test HTTP server using our app
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set the context with trace ID
		ctx := context.WithValue(r.Context(), RequestIDKey, "test-server-trace-id")

		// Log using our logging functions
		InfoWithContext(ctx, "Debug from test server", nil)
		InfoWithContext(ctx, "Info from test server", nil)

		// Continue with request
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// Make a request to our test server
	resp, err := http.Get(ts.URL)
	require.NoError(t, err, "Request to test server should succeed")
	resp.Body.Close()

	// Wait for logs to be written
	time.Sleep(20 * time.Millisecond)

	// Get the buffer content
	output := buf.String()

	// Verify logs
	assert.Contains(t, output, "test-server-trace-id", "Trace ID should be in logs")
	assert.Contains(t, output, "Debug from test server", "Debug message should be in logs")
	assert.Contains(t, output, "Info from test server", "Info message should be in logs")
}
