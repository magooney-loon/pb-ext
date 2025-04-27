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

// MonitoringError represents a structured error
type MonitoringError struct {
	Type    string // Error type category
	Message string // Human-readable message
	Op      string // Operation name
	Err     error  // Original error
}

// Error implements error interface
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

// NewSensorError creates a sensor error
func NewSensorError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeSensor,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewSystemError creates a system error
func NewSystemError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeSystem,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewIOError creates an I/O error
func NewIOError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeIO,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(op string, message string) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypeTimeout,
		Message: message,
		Op:      op,
	}
}

// NewPermissionError creates a permission error
func NewPermissionError(op string, message string, err error) *MonitoringError {
	return &MonitoringError{
		Type:    ErrTypePermission,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// IsErrorType checks if error is of a specific type
func IsErrorType(err error, errorType string) bool {
	if err == nil {
		return false
	}

	if monErr, ok := err.(*MonitoringError); ok {
		return monErr.Type == errorType
	}
	return false
}

// IsTimeout checks if error is a timeout error
func IsTimeout(err error) bool {
	return IsErrorType(err, ErrTypeTimeout)
}

// IsPermissionError checks if error is a permission error
func IsPermissionError(err error) bool {
	return IsErrorType(err, ErrTypePermission)
}

// IsSystemError checks if error is a system error
func IsSystemError(err error) bool {
	return IsErrorType(err, ErrTypeSystem)
}

// IsSensorError checks if error is a sensor error
func IsSensorError(err error) bool {
	return IsErrorType(err, ErrTypeSensor)
}
