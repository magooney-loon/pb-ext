package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
