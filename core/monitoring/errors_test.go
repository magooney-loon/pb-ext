package monitoring

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorConstants(t *testing.T) {
	expectedValues := map[string]string{
		"ErrTypeSensor":       ErrTypeSensor,
		"ErrTypeSystem":       ErrTypeSystem,
		"ErrTypeIO":           ErrTypeIO,
		"ErrTypeTimeout":      ErrTypeTimeout,
		"ErrTypePermission":   ErrTypePermission,
		"ErrTypeUnsupported":  ErrTypeUnsupported,
		"ErrTypeNetworkStats": ErrTypeNetworkStats,
		"ErrTypeProcessStats": ErrTypeProcessStats,
	}

	expectedConstants := map[string]string{
		"ErrTypeSensor":       "sensor_error",
		"ErrTypeSystem":       "system_error",
		"ErrTypeIO":           "io_error",
		"ErrTypeTimeout":      "timeout_error",
		"ErrTypePermission":   "permission_error",
		"ErrTypeUnsupported":  "unsupported_error",
		"ErrTypeNetworkStats": "network_stats_error",
		"ErrTypeProcessStats": "process_stats_error",
	}

	for name, actual := range expectedValues {
		if expected, exists := expectedConstants[name]; exists {
			if actual != expected {
				t.Errorf("Expected %s to be %s, got %s", name, expected, actual)
			}
		}
	}
}

func TestMonitoringError_Error(t *testing.T) {
	testCases := []struct {
		name     string
		error    *MonitoringError
		expected string
	}{
		{
			name: "error with wrapped error",
			error: &MonitoringError{
				Type:    ErrTypeSystem,
				Message: "failed to read file",
				Op:      "read_config",
				Err:     errors.New("file not found"),
			},
			expected: "system_error: read_config failed: file not found",
		},
		{
			name: "error without wrapped error",
			error: &MonitoringError{
				Type:    ErrTypeSensor,
				Message: "sensor not responding",
				Op:      "read_temperature",
				Err:     nil,
			},
			expected: "sensor_error: read_temperature failed: sensor not responding",
		},
		{
			name: "error with empty message",
			error: &MonitoringError{
				Type:    ErrTypeTimeout,
				Message: "",
				Op:      "connect",
				Err:     nil,
			},
			expected: "timeout_error: connect failed: ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.error.Error()
			if result != tc.expected {
				t.Errorf("Expected error message %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestMonitoringError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")

	testCases := []struct {
		name     string
		error    *MonitoringError
		expected error
	}{
		{
			name: "error with wrapped error",
			error: &MonitoringError{
				Type:    ErrTypeSystem,
				Message: "wrapper message",
				Op:      "test_op",
				Err:     originalErr,
			},
			expected: originalErr,
		},
		{
			name: "error without wrapped error",
			error: &MonitoringError{
				Type:    ErrTypeSystem,
				Message: "no wrapped error",
				Op:      "test_op",
				Err:     nil,
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.error.Unwrap()
			if result != tc.expected {
				t.Errorf("Expected unwrapped error %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestNewSensorError(t *testing.T) {
	op := "read_sensor"
	message := "sensor malfunction"
	originalErr := errors.New("hardware failure")

	err := NewSensorError(op, message, originalErr)

	if err.Type != ErrTypeSensor {
		t.Errorf("Expected error type %s, got %s", ErrTypeSensor, err.Type)
	}
	if err.Op != op {
		t.Errorf("Expected operation %s, got %s", op, err.Op)
	}
	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected wrapped error %v, got %v", originalErr, err.Err)
	}
}

func TestNewSystemError(t *testing.T) {
	op := "system_call"
	message := "system failure"
	originalErr := errors.New("kernel panic")

	err := NewSystemError(op, message, originalErr)

	if err.Type != ErrTypeSystem {
		t.Errorf("Expected error type %s, got %s", ErrTypeSystem, err.Type)
	}
	if err.Op != op {
		t.Errorf("Expected operation %s, got %s", op, err.Op)
	}
	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected wrapped error %v, got %v", originalErr, err.Err)
	}
}

func TestNewIOError(t *testing.T) {
	op := "file_operation"
	message := "read failed"
	originalErr := errors.New("permission denied")

	err := NewIOError(op, message, originalErr)

	if err.Type != ErrTypeIO {
		t.Errorf("Expected error type %s, got %s", ErrTypeIO, err.Type)
	}
	if err.Op != op {
		t.Errorf("Expected operation %s, got %s", op, err.Op)
	}
	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected wrapped error %v, got %v", originalErr, err.Err)
	}
}

func TestNewTimeoutError(t *testing.T) {
	op := "network_request"
	message := "request timed out"

	err := NewTimeoutError(op, message)

	if err.Type != ErrTypeTimeout {
		t.Errorf("Expected error type %s, got %s", ErrTypeTimeout, err.Type)
	}
	if err.Op != op {
		t.Errorf("Expected operation %s, got %s", op, err.Op)
	}
	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}
	if err.Err != nil {
		t.Errorf("Expected no wrapped error, got %v", err.Err)
	}
}

func TestNewPermissionError(t *testing.T) {
	op := "file_access"
	message := "access denied"
	originalErr := errors.New("insufficient privileges")

	err := NewPermissionError(op, message, originalErr)

	if err.Type != ErrTypePermission {
		t.Errorf("Expected error type %s, got %s", ErrTypePermission, err.Type)
	}
	if err.Op != op {
		t.Errorf("Expected operation %s, got %s", op, err.Op)
	}
	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected wrapped error %v, got %v", originalErr, err.Err)
	}
}

func TestIsErrorType(t *testing.T) {
	testCases := []struct {
		name      string
		err       error
		errorType string
		expected  bool
	}{
		{
			name:      "matching monitoring error",
			err:       NewSystemError("test_op", "test message", nil),
			errorType: ErrTypeSystem,
			expected:  true,
		},
		{
			name:      "non-matching monitoring error",
			err:       NewSystemError("test_op", "test message", nil),
			errorType: ErrTypeSensor,
			expected:  false,
		},
		{
			name:      "nil error",
			err:       nil,
			errorType: ErrTypeSystem,
			expected:  false,
		},
		{
			name:      "regular error",
			err:       errors.New("regular error"),
			errorType: ErrTypeSystem,
			expected:  false,
		},
		{
			name:      "different monitoring error type",
			err:       NewIOError("io_op", "io failed", nil),
			errorType: ErrTypeTimeout,
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsErrorType(tc.err, tc.errorType)
			if result != tc.expected {
				t.Errorf("Expected IsErrorType to return %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsTimeout(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      NewTimeoutError("test_op", "timeout occurred"),
			expected: true,
		},
		{
			name:     "non-timeout monitoring error",
			err:      NewSystemError("test_op", "system error", nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsTimeout(tc.err)
			if result != tc.expected {
				t.Errorf("Expected IsTimeout to return %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsPermissionError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "permission error",
			err:      NewPermissionError("test_op", "access denied", nil),
			expected: true,
		},
		{
			name:     "non-permission monitoring error",
			err:      NewSystemError("test_op", "system error", nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPermissionError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected IsPermissionError to return %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsSystemError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "system error",
			err:      NewSystemError("test_op", "system failure", nil),
			expected: true,
		},
		{
			name:     "non-system monitoring error",
			err:      NewTimeoutError("test_op", "timeout occurred"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsSystemError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected IsSystemError to return %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsSensorError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "sensor error",
			err:      NewSensorError("test_op", "sensor malfunction", nil),
			expected: true,
		},
		{
			name:     "non-sensor monitoring error",
			err:      NewSystemError("test_op", "system error", nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsSensorError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected IsSensorError to return %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestMonitoringErrorChaining(t *testing.T) {
	// Test that errors.Is and errors.As work properly with MonitoringError
	rootCause := errors.New("root cause error")
	monitoringErr := NewSystemError("test_op", "system failed", rootCause)

	// Test errors.Is
	if !errors.Is(monitoringErr, rootCause) {
		t.Error("Expected errors.Is to find root cause in monitoring error")
	}

	// Test errors.As with MonitoringError
	var targetMonErr *MonitoringError
	if !errors.As(monitoringErr, &targetMonErr) {
		t.Error("Expected errors.As to extract MonitoringError")
	}
	if targetMonErr.Type != ErrTypeSystem {
		t.Errorf("Expected extracted error type %s, got %s", ErrTypeSystem, targetMonErr.Type)
	}
}

func TestMonitoringErrorFields(t *testing.T) {
	// Test that all fields are properly set and accessible
	err := &MonitoringError{
		Type:    ErrTypeIO,
		Message: "custom message",
		Op:      "custom_operation",
		Err:     errors.New("custom wrapped error"),
	}

	if err.Type != ErrTypeIO {
		t.Errorf("Expected Type %s, got %s", ErrTypeIO, err.Type)
	}
	if err.Message != "custom message" {
		t.Errorf("Expected Message 'custom message', got %s", err.Message)
	}
	if err.Op != "custom_operation" {
		t.Errorf("Expected Op 'custom_operation', got %s", err.Op)
	}
	if err.Err.Error() != "custom wrapped error" {
		t.Errorf("Expected wrapped error 'custom wrapped error', got %s", err.Err.Error())
	}
}

// Benchmark tests
func BenchmarkNewSystemError(b *testing.B) {
	originalErr := errors.New("benchmark error")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = NewSystemError("benchmark_op", "benchmark message", originalErr)
	}
}

func BenchmarkMonitoringError_Error(b *testing.B) {
	err := NewSystemError("benchmark_op", "benchmark message", errors.New("wrapped error"))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkIsErrorType(b *testing.B) {
	err := NewSystemError("benchmark_op", "benchmark message", nil)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = IsErrorType(err, ErrTypeSystem)
	}
}

// Example tests to demonstrate usage
func ExampleNewSystemError() {
	originalErr := errors.New("disk full")
	monErr := NewSystemError("write_file", "failed to write configuration", originalErr)

	fmt.Println(monErr.Error())
	// Output: system_error: write_file failed: disk full
}

func ExampleIsTimeout() {
	timeoutErr := NewTimeoutError("network_call", "request timed out after 30s")
	systemErr := NewSystemError("file_read", "failed to read file", nil)

	fmt.Println("Timeout error is timeout:", IsTimeout(timeoutErr))
	fmt.Println("System error is timeout:", IsTimeout(systemErr))
	// Output:
	// Timeout error is timeout: true
	// System error is timeout: false
}
