package logging

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/pocketbase/pocketbase/tests"
)

func TestLogLevel_String(t *testing.T) {
	testCases := []struct {
		level    LogLevel
		expected string
	}{
		{Debug, "DEBUG"},
		{Info, "INFO"},
		{Warn, "WARN"},
		{Error, "ERROR"},
		{LogLevel(99), "LEVEL_99"},
		{LogLevel(-10), "LEVEL_-10"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("LogLevel_%d", tc.level), func(t *testing.T) {
			result := tc.level.String()
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestLogLevelValues(t *testing.T) {
	if Debug != -4 {
		t.Errorf("Expected Debug level to be -4, got %d", Debug)
	}
	if Info != 0 {
		t.Errorf("Expected Info level to be 0, got %d", Info)
	}
	if Warn != 4 {
		t.Errorf("Expected Warn level to be 4, got %d", Warn)
	}
	if Error != 8 {
		t.Errorf("Expected Error level to be 8, got %d", Error)
	}
}

func TestConstants(t *testing.T) {
	if TraceIDHeader != "X-Trace-ID" {
		t.Errorf("Expected TraceIDHeader to be 'X-Trace-ID', got %s", TraceIDHeader)
	}
	if RequestIDKey != "request_id" {
		t.Errorf("Expected RequestIDKey to be 'request_id', got %s", RequestIDKey)
	}
}

func TestLogContext(t *testing.T) {
	traceID := "test-trace-123"
	startTime := time.Now()
	duration := 100 * time.Millisecond

	logCtx := LogContext{
		TraceID:    traceID,
		StartTime:  startTime,
		Method:     "GET",
		Path:       "/api/test",
		StatusCode: 200,
		Duration:   duration,
		UserAgent:  "test-agent",
		IP:         "127.0.0.1",
	}

	if logCtx.TraceID != traceID {
		t.Errorf("Expected TraceID %s, got %s", traceID, logCtx.TraceID)
	}
	if logCtx.Method != "GET" {
		t.Errorf("Expected Method GET, got %s", logCtx.Method)
	}
	if logCtx.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", logCtx.StatusCode)
	}
	if logCtx.Duration != duration {
		t.Errorf("Expected Duration %v, got %v", duration, logCtx.Duration)
	}
}

func TestLogContextZeroValues(t *testing.T) {
	var logCtx LogContext

	if logCtx.TraceID != "" {
		t.Errorf("Expected empty TraceID, got %s", logCtx.TraceID)
	}
	if logCtx.StatusCode != 0 {
		t.Errorf("Expected StatusCode 0, got %d", logCtx.StatusCode)
	}
	if !logCtx.StartTime.IsZero() {
		t.Errorf("Expected zero time, got %v", logCtx.StartTime)
	}
}

func TestInfoWithContext(t *testing.T) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer testApp.Cleanup()

	ctx := context.WithValue(context.Background(), RequestIDKey, "test-request-123")
	message := "Test info message"
	data := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	// Should not panic
	InfoWithContext(ctx, testApp, message, data)

	// Test with nil context
	InfoWithContext(context.TODO(), testApp, message, data)

	// Test with empty data
	InfoWithContext(ctx, testApp, message, map[string]interface{}{})
}

func TestErrorWithContext(t *testing.T) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer testApp.Cleanup()

	ctx := context.WithValue(context.Background(), RequestIDKey, "test-request-456")
	message := "Test error message"
	testErr := errors.New("test error")
	data := map[string]any{
		"operation": "test_op",
		"user_id":   123,
	}

	// Should not panic
	ErrorWithContext(ctx, testApp, message, testErr, data)

	// Test with nil error
	ErrorWithContext(ctx, testApp, message, nil, data)

	// Test with nil context
	ErrorWithContext(context.TODO(), testApp, message, testErr, data)

	// Test with empty data
	ErrorWithContext(ctx, testApp, message, testErr, map[string]any{})
}

func TestHandleContextErrorsBasic(t *testing.T) {
	testCases := []struct {
		name         string
		ctx          context.Context
		err          error
		op           string
		expectedType string
		shouldBeNil  bool
	}{
		{
			name:        "nil error",
			ctx:         context.Background(),
			err:         nil,
			op:          "test_op",
			shouldBeNil: true,
		},
		{
			name: "context deadline exceeded",
			ctx: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				return ctx
			}(),
			err:          errors.New("some error"),
			op:           "test_op",
			expectedType: monitoring.ErrTypeTimeout,
		},
		{
			name: "context canceled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			err:          errors.New("some error"),
			op:           "test_op",
			expectedType: monitoring.ErrTypeTimeout,
		},
		{
			name:         "monitoring error passthrough",
			ctx:          context.Background(),
			err:          monitoring.NewSystemError("orig_op", "original error", errors.New("root cause")),
			op:           "test_op",
			expectedType: monitoring.ErrTypeSystem,
		},
		{
			name:         "generic error wrapped",
			ctx:          context.Background(),
			err:          errors.New("generic error"),
			op:           "test_op",
			expectedType: monitoring.ErrTypeSystem,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HandleContextErrors(tc.ctx, tc.err, tc.op)

			if tc.shouldBeNil {
				if result != nil {
					t.Errorf("Expected nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected error, got nil")
				return
			}

			// Check error type for monitoring errors
			if tc.expectedType != "" {
				if monErr, ok := result.(*monitoring.MonitoringError); ok {
					if monErr.Type != tc.expectedType {
						t.Errorf("Expected error type %s, got %s", tc.expectedType, monErr.Type)
					}
				} else {
					t.Errorf("Expected MonitoringError, got %T", result)
				}
			}
		})
	}
}

func TestHandleContextErrorsEdgeCases(t *testing.T) {
	// Test with nil context
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

// Benchmark tests
func BenchmarkLogLevelString(b *testing.B) {
	level := Info
	for i := 0; i < b.N; i++ {
		_ = level.String()
	}
}

func BenchmarkInfoWithContext(b *testing.B) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		b.Fatal(err)
	}
	defer testApp.Cleanup()

	ctx := context.WithValue(context.Background(), RequestIDKey, "bench-request")
	message := "Benchmark message"
	data := map[string]interface{}{
		"key": "value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		InfoWithContext(ctx, testApp, message, data)
	}
}

func BenchmarkErrorWithContext(b *testing.B) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		b.Fatal(err)
	}
	defer testApp.Cleanup()

	ctx := context.WithValue(context.Background(), RequestIDKey, "bench-request")
	message := "Benchmark error message"
	testErr := errors.New("benchmark error")
	data := map[string]any{
		"operation": "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ErrorWithContext(ctx, testApp, message, testErr, data)
	}
}

func BenchmarkHandleContextErrorsBasic(b *testing.B) {
	ctx := context.Background()
	err := errors.New("benchmark error")
	operation := "benchmark_op"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HandleContextErrors(ctx, err, operation)
	}
}

// Test helper functions
func TestCreateTestContext(t *testing.T) {
	requestID := "test-request-id"
	ctx := context.WithValue(context.Background(), RequestIDKey, requestID)

	value := ctx.Value(RequestIDKey)
	if value == nil {
		t.Error("Expected request ID to be set in context")
	}

	if str, ok := value.(string); !ok || str != requestID {
		t.Errorf("Expected request ID %s, got %v", requestID, value)
	}
}
