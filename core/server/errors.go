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

// ServerError represents a structured error
type ServerError struct {
	Type       string // Error type category
	Message    string // Human-readable message
	Op         string // Operation name
	StatusCode int    // HTTP status code
	Err        error  // Original error
}

// Error implements error interface
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

// NewHTTPError creates an HTTP error
func NewHTTPError(op string, message string, statusCode int, err error) *ServerError {
	return &ServerError{
		Type:       ErrTypeHTTP,
		Message:    message,
		Op:         op,
		StatusCode: statusCode,
		Err:        err,
	}
}

// NewRoutingError creates a routing error
func NewRoutingError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeRouting,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewAuthError creates an authentication error
func NewAuthError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:       ErrTypeAuth,
		Message:    message,
		Op:         op,
		StatusCode: 401,
		Err:        err,
	}
}

// NewTemplateError creates a template error
func NewTemplateError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeTemplate,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewConfigError creates a configuration error
func NewConfigError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeConfig,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewDatabaseError creates a database error
func NewDatabaseError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:    ErrTypeDatabase,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// NewInternalError creates an internal server error
func NewInternalError(op string, message string, err error) *ServerError {
	return &ServerError{
		Type:       ErrTypeInternal,
		Message:    message,
		Op:         op,
		StatusCode: 500,
		Err:        err,
	}
}

// IsErrorType checks if error is of a specific type
func IsErrorType(err error, errorType string) bool {
	if err == nil {
		return false
	}

	if srvErr, ok := err.(*ServerError); ok {
		return srvErr.Type == errorType
	}
	return false
}

// IsHTTPError checks if error is an HTTP error
func IsHTTPError(err error) bool {
	return IsErrorType(err, ErrTypeHTTP)
}

// IsRoutingError checks if error is a routing error
func IsRoutingError(err error) bool {
	return IsErrorType(err, ErrTypeRouting)
}

// IsAuthError checks if error is an auth error
func IsAuthError(err error) bool {
	return IsErrorType(err, ErrTypeAuth)
}

// IsTemplateError checks if error is a template error
func IsTemplateError(err error) bool {
	return IsErrorType(err, ErrTypeTemplate)
}

// IsConfigError checks if error is a config error
func IsConfigError(err error) bool {
	return IsErrorType(err, ErrTypeConfig)
}

// IsDatabaseError checks if error is a database error
func IsDatabaseError(err error) bool {
	return IsErrorType(err, ErrTypeDatabase)
}

// IsInternalError checks if error is an internal error
func IsInternalError(err error) bool {
	return IsErrorType(err, ErrTypeInternal)
}
