package monitoring

import (
	"fmt"
)

// Error types for monitoring operations
const (
	ErrTypeSensor       = "sensor_error"
	ErrTypeSystem       = "system_error"
	ErrTypeIO           = "io_error"
	ErrTypeTimeout      = "timeout_error"
	ErrTypePermission   = "permission_error"
	ErrTypeUnsupported  = "unsupported_error"
	ErrTypeNetworkStats = "network_stats_error"
	ErrTypeProcessStats = "process_stats_error"
)

// MonitoringError represents a structured error from monitoring operations
type MonitoringError struct {
	Type    string // Error type category
	Message string // Human-readable error message
	Op      string // Operation that caused the error
	Err     error  // Original error if any
}

// Error implements the error interface
func (e *MonitoringError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s failed: %v", e.Type, e.Op, e.Err)
	}
	return fmt.Sprintf("%s: %s failed: %s", e.Type, e.Op, e.Message)
}

// Unwrap returns the wrapped error
func (e *MonitoringError) Unwrap() error {
	return e.Err
}

// NewSensorError creates a new error for sensor-related issues
func NewSensorError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeSensor,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewSystemError creates a new error for system information issues
func NewSystemError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeSystem,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewIOError creates a new error for I/O related issues
func NewIOError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeIO,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewTimeoutError creates a new error for timeout issues
func NewTimeoutError(op string, message string) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeTimeout,
		Message: message,
		Op:      op,
	}
}

// NewPermissionError creates a new error for permission issues
func NewPermissionError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypePermission,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// IsErrorType checks if an error is of a specific monitoring error type
func IsErrorType(err error, errorType string) bool {
	if err == nil {
		return false
	}

	if monErr, ok := err.(*MonitoringError); ok {
		return monErr.Type == errorType
	}
	return false
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	return IsErrorType(err, ErrTypeTimeout)
}

// IsPermissionError checks if an error is a permission error
func IsPermissionError(err error) bool {
	return IsErrorType(err, ErrTypePermission)
}

// IsSystemError checks if an error is a system error
func IsSystemError(err error) bool {
	return IsErrorType(err, ErrTypeSystem)
}

// IsSensorError checks if an error is a sensor error
func IsSensorError(err error) bool {
	return IsErrorType(err, ErrTypeSensor)
}
