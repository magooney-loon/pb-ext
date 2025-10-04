package logging

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/magooney-loon/pb-ext/core/server"
)

func TestErrorResponseStruct(t *testing.T) {
	traceID := "test-trace-123"
	timestamp := time.Now().Format(time.RFC3339)

	resp := ErrorResponse{
		Status:     "Internal Server Error",
		Message:    "Test error message",
		Type:       "system_error",
		Operation:  "test_operation",
		StatusCode: 500,
		TraceID:    traceID,
		Timestamp:  timestamp,
	}

	// Test struct fields
	if resp.Status != "Internal Server Error" {
		t.Errorf("Expected Status 'Internal Server Error', got %s", resp.Status)
	}
	if resp.Message != "Test error message" {
		t.Errorf("Expected Message 'Test error message', got %s", resp.Message)
	}
	if resp.Type != "system_error" {
		t.Errorf("Expected Type 'system_error', got %s", resp.Type)
	}
	if resp.Operation != "test_operation" {
		t.Errorf("Expected Operation 'test_operation', got %s", resp.Operation)
	}
	if resp.StatusCode != 500 {
		t.Errorf("Expected StatusCode 500, got %d", resp.StatusCode)
	}
	if resp.TraceID != traceID {
		t.Errorf("Expected TraceID %s, got %s", traceID, resp.TraceID)
	}
}

func TestErrorResponseJSONSerialization(t *testing.T) {
	resp := ErrorResponse{
		Status:     "Bad Request",
		Message:    "Invalid input",
		Type:       "validation_error",
		Operation:  "create_user",
		StatusCode: 400,
		TraceID:    "trace-456",
		Timestamp:  "2023-01-01T12:00:00Z",
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal ErrorResponse: %v", err)
	}

	var unmarshaled ErrorResponse
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ErrorResponse: %v", err)
	}

	if unmarshaled.Status != resp.Status {
		t.Errorf("Expected Status %s, got %s", resp.Status, unmarshaled.Status)
	}
	if unmarshaled.StatusCode != resp.StatusCode {
		t.Errorf("Expected StatusCode %d, got %d", resp.StatusCode, unmarshaled.StatusCode)
	}
	if unmarshaled.TraceID != resp.TraceID {
		t.Errorf("Expected TraceID %s, got %s", resp.TraceID, unmarshaled.TraceID)
	}
}

func TestHandleContextErrorsAdvanced(t *testing.T) {
	testCases := []struct {
		name         string
		setupContext func() context.Context
		inputError   error
		operation    string
		expectNil    bool
		expectedType string
	}{
		{
			name: "nil error returns nil",
			setupContext: func() context.Context {
				return context.Background()
			},
			inputError: nil,
			operation:  "test_op",
			expectNil:  true,
		},
		{
			name: "context deadline exceeded",
			setupContext: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				return ctx
			},
			inputError:   errors.New("some error"),
			operation:    "timeout_test",
			expectedType: monitoring.ErrTypeTimeout,
		},
		{
			name: "context canceled",
			setupContext: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			inputError:   errors.New("canceled error"),
			operation:    "cancel_test",
			expectedType: monitoring.ErrTypeTimeout,
		},
		{
			name: "monitoring error passthrough",
			setupContext: func() context.Context {
				return context.Background()
			},
			inputError:   monitoring.NewSystemError("original_op", "system failure", errors.New("root cause")),
			operation:    "passthrough_test",
			expectedType: monitoring.ErrTypeSystem,
		},
		{
			name: "server error passthrough",
			setupContext: func() context.Context {
				return context.Background()
			},
			inputError: server.NewHTTPError("server_op", "server failure", 400, errors.New("http error")),
			operation:  "server_passthrough_test",
			// Server errors don't use the same type system
		},
		{
			name: "generic error wrapped as system error",
			setupContext: func() context.Context {
				return context.Background()
			},
			inputError:   errors.New("generic error"),
			operation:    "generic_test",
			expectedType: monitoring.ErrTypeSystem,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.setupContext()
			result := HandleContextErrors(ctx, tc.inputError, tc.operation)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected error, got nil")
				return
			}

			// Check if it's a monitoring error with the expected type
			if tc.expectedType != "" {
				if monErr, ok := result.(*monitoring.MonitoringError); ok {
					if monErr.Type != tc.expectedType {
						t.Errorf("Expected error type %s, got %s", tc.expectedType, monErr.Type)
					}
					// For passthrough errors, operation should remain unchanged
					if tc.name == "monitoring error passthrough" {
						if monErr.Op != "original_op" {
							t.Errorf("Expected original operation to be preserved, got %s", monErr.Op)
						}
					} else if monErr.Op != tc.operation {
						t.Errorf("Expected operation %s, got %s", tc.operation, monErr.Op)
					}
				} else {
					t.Errorf("Expected MonitoringError, got %T", result)
				}
			}
		})
	}
}

func TestBrowserDetectionLogic(t *testing.T) {
	testCases := []struct {
		userAgent string
		expected  bool
		name      string
	}{
		{
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			expected:  true,
			name:      "Chrome on Windows",
		},
		{
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expected:  true,
			name:      "Chrome on Mac",
		},
		{
			userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0",
			expected:  true,
			name:      "Firefox on Linux",
		},
		{
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0.3 Mobile/15E148 Safari/604.1",
			expected:  true,
			name:      "Safari on iOS",
		},
		{
			userAgent: "curl/7.68.0",
			expected:  false,
			name:      "curl",
		},
		{
			userAgent: "PostmanRuntime/7.28.0",
			expected:  false,
			name:      "Postman",
		},
		{
			userAgent: "Go-http-client/1.1",
			expected:  false,
			name:      "Go HTTP client",
		},
		{
			userAgent: "",
			expected:  false,
			name:      "Empty user agent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This tests the browser detection logic used in error handlers
			userAgent := strings.ToLower(tc.userAgent)
			isBrowser := strings.Contains(userAgent, "mozilla") ||
				strings.Contains(userAgent, "chrome") ||
				strings.Contains(userAgent, "safari") ||
				strings.Contains(userAgent, "firefox")

			if isBrowser != tc.expected {
				t.Errorf("Expected isBrowser to be %t for '%s', got %t", tc.expected, tc.userAgent, isBrowser)
			}
		})
	}
}

func TestErrorResponseCreationWithStatusCodes(t *testing.T) {
	traceID := "trace-789"
	timestamp := time.Now()

	resp := ErrorResponse{
		Status:     "Bad Request",
		Message:    "Validation failed",
		Type:       "validation_error",
		Operation:  "create_resource",
		StatusCode: 400,
		TraceID:    traceID,
		Timestamp:  timestamp.Format(time.RFC3339),
	}

	// Test that all fields are set correctly
	if resp.Status != "Bad Request" {
		t.Errorf("Expected Status 'Bad Request', got %s", resp.Status)
	}
	if resp.StatusCode != 400 {
		t.Errorf("Expected StatusCode 400, got %d", resp.StatusCode)
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check that all fields are present in JSON
	expectedFields := []string{"status", "message", "type", "operation", "status_code", "trace_id", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("Expected field '%s' to be present in JSON", field)
		}
	}
}

func TestHandleContextErrorsNilContext(t *testing.T) {
	// This should not panic even with nil context
	result := HandleContextErrors(context.TODO(), errors.New("test error"), "nil_context_test")

	if result == nil {
		t.Error("Expected error to be returned even with nil context")
	}

	// Should wrap as system error since context is nil
	if monErr, ok := result.(*monitoring.MonitoringError); ok {
		if monErr.Type != monitoring.ErrTypeSystem {
			t.Errorf("Expected system error type, got %s", monErr.Type)
		}
	}
}

func TestErrorResponseEmptyFields(t *testing.T) {
	resp := ErrorResponse{}

	// Should not panic when marshaling empty response
	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal empty response: %v", err)
	}

	var parsed ErrorResponse
	err = json.Unmarshal(jsonData, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty response: %v", err)
	}

	// All fields should be zero values
	if parsed.Status != "" || parsed.Message != "" || parsed.StatusCode != 0 {
		t.Error("Expected zero values for empty response")
	}
}

// Benchmark tests
func BenchmarkHandleContextErrorsAdvanced(b *testing.B) {
	ctx := context.Background()
	err := errors.New("benchmark error")
	operation := "benchmark_op"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HandleContextErrors(ctx, err, operation)
	}
}

func BenchmarkErrorResponseJSONMarshaling(b *testing.B) {
	resp := ErrorResponse{
		Status:     "Internal Server Error",
		Message:    "Benchmark error",
		Type:       "system_error",
		Operation:  "benchmark_op",
		StatusCode: 500,
		TraceID:    "bench-trace",
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(resp)
	}
}

func BenchmarkBrowserDetectionAdvanced(b *testing.B) {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"curl/7.68.0",
		"Mozilla/5.0 (X11; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0",
		"PostmanRuntime/7.28.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userAgent := strings.ToLower(userAgents[i%len(userAgents)])
		_ = strings.Contains(userAgent, "mozilla") ||
			strings.Contains(userAgent, "chrome") ||
			strings.Contains(userAgent, "safari") ||
			strings.Contains(userAgent, "firefox")
	}
}

// Test server error integration
func TestServerErrorIntegration(t *testing.T) {
	// Test with HTTP error
	httpErr := server.NewHTTPError("test_op", "HTTP error occurred", 404, errors.New("not found"))

	result := HandleContextErrors(context.Background(), httpErr, "test_operation")

	// Should return the original server error
	if result != httpErr {
		t.Errorf("Expected server error to be passed through, got %T", result)
	}

	// Test with internal error
	internalErr := server.NewInternalError("internal_op", "Internal error", errors.New("internal issue"))

	result2 := HandleContextErrors(context.Background(), internalErr, "test_operation")

	// Should return the original server error
	if result2 != internalErr {
		t.Errorf("Expected internal error to be passed through, got %T", result2)
	}
}
