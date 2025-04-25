package server

import (
	"fmt"
)

// Error types for server operations
const (
	ErrTypeHTTP       = "http_error"
	ErrTypeRouting    = "routing_error"
	ErrTypeAuth       = "auth_error"
	ErrTypeTemplate   = "template_error"
	ErrTypeConfig     = "config_error"
	ErrTypeDatabase   = "database_error"
	ErrTypeMiddleware = "middleware_error"
	ErrTypeInternal   = "internal_error"
)

// ServerError represents a structured error from server operations
type ServerError struct {
	Type       string // Error type category
	Message    string // Human-readable error message
	Op         string // Operation that caused the error
	StatusCode int    // HTTP status code if applicable
	Err        error  // Original error if any
}

// Error implements the error interface
func (e *ServerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s failed: %v", e.Type, e.Op, e.Err)
	}
	return fmt.Sprintf("%s: %s failed: %s", e.Type, e.Op, e.Message)
}

// Unwrap returns the wrapped error
func (e *ServerError) Unwrap() error {
	return e.Err
}

// NewHTTPError creates a new error for HTTP-related issues
func NewHTTPError(op string, message string, statusCode int, err error) *ServerError {
	return &ServerError{
		Type:       ErrTypeHTTP,
		Message:    message,
		Op:         op,
		StatusCode: statusCode,
		Err:        err,
	}
}

// NewRoutingError creates a new error for routing issues
func NewRoutingError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeRouting,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewAuthError creates a new error for authentication issues
func NewAuthError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:       ErrTypeAuth,
		Message:    message,
		Op:         op,
		StatusCode: 401,
		Err:        err,
	}
}

// NewTemplateError creates a new error for template processing issues
func NewTemplateError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeTemplate,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewConfigError creates a new error for configuration issues
func NewConfigError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeConfig,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewDatabaseError creates a new error for database issues
func NewDatabaseError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeDatabase,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewInternalError creates a new error for internal server issues
func NewInternalError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:       ErrTypeInternal,
		Message:    message,
		Op:         op,
		StatusCode: 500,
		Err:        err,
	}
}

// IsErrorType checks if an error is of a specific server error type
func IsErrorType(err error, errorType string) bool {
	if err == nil {
		return false
	}

	if srvErr, ok := err.(*ServerError); ok {
		return srvErr.Type == errorType
	}
	return false
}

// IsHTTPError checks if an error is an HTTP error
func IsHTTPError(err error) bool {
	return IsErrorType(err, ErrTypeHTTP)
}

// IsRoutingError checks if an error is a routing error
func IsRoutingError(err error) bool {
	return IsErrorType(err, ErrTypeRouting)
}

// IsAuthError checks if an error is an authentication error
func IsAuthError(err error) bool {
	return IsErrorType(err, ErrTypeAuth)
}

// IsTemplateError checks if an error is a template error
func IsTemplateError(err error) bool {
	return IsErrorType(err, ErrTypeTemplate)
}

// IsConfigError checks if an error is a configuration error
func IsConfigError(err error) bool {
	return IsErrorType(err, ErrTypeConfig)
}

// IsDatabaseError checks if an error is a database error
func IsDatabaseError(err error) bool {
	return IsErrorType(err, ErrTypeDatabase)
}

// IsInternalError checks if an error is an internal server error
func IsInternalError(err error) bool {
	return IsErrorType(err, ErrTypeInternal)
}
