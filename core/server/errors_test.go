package server

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorConstants(t *testing.T) {
	expectedConstants := map[string]string{
		"ErrTypeHTTP":       "http_error",
		"ErrTypeRouting":    "routing_error",
		"ErrTypeAuth":       "auth_error",
		"ErrTypeTemplate":   "template_error",
		"ErrTypeConfig":     "config_error",
		"ErrTypeDatabase":   "database_error",
		"ErrTypeMiddleware": "middleware_error",
		"ErrTypeInternal":   "internal_error",
	}

	constants := map[string]string{
		"ErrTypeHTTP":       ErrTypeHTTP,
		"ErrTypeRouting":    ErrTypeRouting,
		"ErrTypeAuth":       ErrTypeAuth,
		"ErrTypeTemplate":   ErrTypeTemplate,
		"ErrTypeConfig":     ErrTypeConfig,
		"ErrTypeDatabase":   ErrTypeDatabase,
		"ErrTypeMiddleware": ErrTypeMiddleware,
		"ErrTypeInternal":   ErrTypeInternal,
	}

	for name, actual := range constants {
		expected := expectedConstants[name]
		if actual != expected {
			t.Errorf("Expected %s to be %s, got %s", name, expected, actual)
		}
	}
}

func TestServerErrorStruct(t *testing.T) {
	originalErr := errors.New("original error")
	serverErr := &ServerError{
		Type:       ErrTypeHTTP,
		Message:    "Test message",
		Op:         "test_operation",
		StatusCode: 404,
		Err:        originalErr,
	}

	if serverErr.Type != ErrTypeHTTP {
		t.Errorf("Expected Type %s, got %s", ErrTypeHTTP, serverErr.Type)
	}
	if serverErr.Message != "Test message" {
		t.Errorf("Expected Message 'Test message', got %s", serverErr.Message)
	}
	if serverErr.Op != "test_operation" {
		t.Errorf("Expected Op 'test_operation', got %s", serverErr.Op)
	}
	if serverErr.StatusCode != 404 {
		t.Errorf("Expected StatusCode 404, got %d", serverErr.StatusCode)
	}
	if serverErr.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", serverErr.Err)
	}
}

func TestServerErrorError(t *testing.T) {
	testCases := []struct {
		name     string
		err      *ServerError
		expected string
	}{
		{
			name: "with wrapped error",
			err: &ServerError{
				Type:    ErrTypeHTTP,
				Op:      "test_op",
				Message: "test message",
				Err:     errors.New("wrapped error"),
			},
			expected: "http_error: test_op failed: wrapped error",
		},
		{
			name: "without wrapped error",
			err: &ServerError{
				Type:    ErrTypeDatabase,
				Op:      "db_operation",
				Message: "database connection failed",
				Err:     nil,
			},
			expected: "database_error: db_operation failed: database connection failed",
		},
		{
			name: "empty message without wrapped error",
			err: &ServerError{
				Type:    ErrTypeInternal,
				Op:      "internal_op",
				Message: "",
				Err:     nil,
			},
			expected: "internal_error: internal_op failed: ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.err.Error()
			if result != tc.expected {
				t.Errorf("Expected error message '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestServerErrorUnwrap(t *testing.T) {
	testCases := []struct {
		name     string
		err      *ServerError
		expected error
	}{
		{
			name: "with wrapped error",
			err: &ServerError{
				Type: ErrTypeHTTP,
				Op:   "test_op",
				Err:  errors.New("wrapped error"),
			},
			expected: errors.New("wrapped error"),
		},
		{
			name: "without wrapped error",
			err: &ServerError{
				Type: ErrTypeDatabase,
				Op:   "db_op",
				Err:  nil,
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.err.Unwrap()
			if tc.expected == nil {
				if result != nil {
					t.Errorf("Expected nil error, got %v", result)
				}
			} else {
				if result == nil || result.Error() != tc.expected.Error() {
					t.Errorf("Expected error %v, got %v", tc.expected, result)
				}
			}
		})
	}
}

func TestNewHTTPError(t *testing.T) {
	originalErr := errors.New("http error")
	err := NewHTTPError("http_op", "HTTP request failed", 404, originalErr)

	if err.Type != ErrTypeHTTP {
		t.Errorf("Expected Type %s, got %s", ErrTypeHTTP, err.Type)
	}
	if err.Op != "http_op" {
		t.Errorf("Expected Op 'http_op', got %s", err.Op)
	}
	if err.Message != "HTTP request failed" {
		t.Errorf("Expected Message 'HTTP request failed', got %s", err.Message)
	}
	if err.StatusCode != 404 {
		t.Errorf("Expected StatusCode 404, got %d", err.StatusCode)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
	}
}

func TestNewRoutingError(t *testing.T) {
	originalErr := errors.New("route not found")
	err := NewRoutingError("routing_op", "Route resolution failed", originalErr)

	if err.Type != ErrTypeRouting {
		t.Errorf("Expected Type %s, got %s", ErrTypeRouting, err.Type)
	}
	if err.Op != "routing_op" {
		t.Errorf("Expected Op 'routing_op', got %s", err.Op)
	}
	if err.Message != "Route resolution failed" {
		t.Errorf("Expected Message 'Route resolution failed', got %s", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
	}
	// Routing errors don't set StatusCode by default
	if err.StatusCode != 0 {
		t.Errorf("Expected StatusCode 0, got %d", err.StatusCode)
	}
}

func TestNewAuthError(t *testing.T) {
	originalErr := errors.New("invalid credentials")
	err := NewAuthError("auth_op", "Authentication failed", originalErr)

	if err.Type != ErrTypeAuth {
		t.Errorf("Expected Type %s, got %s", ErrTypeAuth, err.Type)
	}
	if err.Op != "auth_op" {
		t.Errorf("Expected Op 'auth_op', got %s", err.Op)
	}
	if err.Message != "Authentication failed" {
		t.Errorf("Expected Message 'Authentication failed', got %s", err.Message)
	}
	if err.StatusCode != 401 {
		t.Errorf("Expected StatusCode 401, got %d", err.StatusCode)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
	}
}

func TestNewTemplateError(t *testing.T) {
	originalErr := errors.New("template parse error")
	err := NewTemplateError("template_op", "Template rendering failed", originalErr)

	if err.Type != ErrTypeTemplate {
		t.Errorf("Expected Type %s, got %s", ErrTypeTemplate, err.Type)
	}
	if err.Op != "template_op" {
		t.Errorf("Expected Op 'template_op', got %s", err.Op)
	}
	if err.Message != "Template rendering failed" {
		t.Errorf("Expected Message 'Template rendering failed', got %s", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
	}
}

func TestNewConfigError(t *testing.T) {
	originalErr := errors.New("config validation failed")
	err := NewConfigError("config_op", "Configuration error", originalErr)

	if err.Type != ErrTypeConfig {
		t.Errorf("Expected Type %s, got %s", ErrTypeConfig, err.Type)
	}
	if err.Op != "config_op" {
		t.Errorf("Expected Op 'config_op', got %s", err.Op)
	}
	if err.Message != "Configuration error" {
		t.Errorf("Expected Message 'Configuration error', got %s", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
	}
}

func TestNewDatabaseError(t *testing.T) {
	originalErr := errors.New("connection timeout")
	err := NewDatabaseError("db_op", "Database operation failed", originalErr)

	if err.Type != ErrTypeDatabase {
		t.Errorf("Expected Type %s, got %s", ErrTypeDatabase, err.Type)
	}
	if err.Op != "db_op" {
		t.Errorf("Expected Op 'db_op', got %s", err.Op)
	}
	if err.Message != "Database operation failed" {
		t.Errorf("Expected Message 'Database operation failed', got %s", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
	}
}

func TestNewInternalError(t *testing.T) {
	originalErr := errors.New("internal system failure")
	err := NewInternalError("internal_op", "Internal server error", originalErr)

	if err.Type != ErrTypeInternal {
		t.Errorf("Expected Type %s, got %s", ErrTypeInternal, err.Type)
	}
	if err.Op != "internal_op" {
		t.Errorf("Expected Op 'internal_op', got %s", err.Op)
	}
	if err.Message != "Internal server error" {
		t.Errorf("Expected Message 'Internal server error', got %s", err.Message)
	}
	if err.StatusCode != 500 {
		t.Errorf("Expected StatusCode 500, got %d", err.StatusCode)
	}
	if err.Err != originalErr {
		t.Errorf("Expected Err to be original error, got %v", err.Err)
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
			name:      "matching server error",
			err:       &ServerError{Type: ErrTypeHTTP},
			errorType: ErrTypeHTTP,
			expected:  true,
		},
		{
			name:      "non-matching server error",
			err:       &ServerError{Type: ErrTypeHTTP},
			errorType: ErrTypeDatabase,
			expected:  false,
		},
		{
			name:      "nil error",
			err:       nil,
			errorType: ErrTypeHTTP,
			expected:  false,
		},
		{
			name:      "regular error",
			err:       errors.New("regular error"),
			errorType: ErrTypeHTTP,
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsErrorType(tc.err, tc.errorType)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsHTTPError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "HTTP error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: true,
		},
		{
			name:     "non-HTTP server error",
			err:      NewDatabaseError("op", "msg", nil),
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsHTTPError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsRoutingError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "routing error",
			err:      NewRoutingError("op", "msg", nil),
			expected: true,
		},
		{
			name:     "non-routing server error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRoutingError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "auth error",
			err:      NewAuthError("op", "msg", nil),
			expected: true,
		},
		{
			name:     "non-auth server error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsAuthError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsTemplateError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "template error",
			err:      NewTemplateError("op", "msg", nil),
			expected: true,
		},
		{
			name:     "non-template server error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsTemplateError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsConfigError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "config error",
			err:      NewConfigError("op", "msg", nil),
			expected: true,
		},
		{
			name:     "non-config server error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsConfigError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsDatabaseError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "database error",
			err:      NewDatabaseError("op", "msg", nil),
			expected: true,
		},
		{
			name:     "non-database server error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDatabaseError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestIsInternalError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "internal error",
			err:      NewInternalError("op", "msg", nil),
			expected: true,
		},
		{
			name:     "non-internal server error",
			err:      NewHTTPError("op", "msg", 404, nil),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsInternalError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, result)
			}
		})
	}
}

func TestServerErrorChaining(t *testing.T) {
	originalErr := errors.New("root cause")
	serverErr := NewHTTPError("http_op", "HTTP failed", 500, originalErr)

	// Test that errors.Is works with wrapped errors
	if !errors.Is(serverErr, originalErr) {
		t.Error("Expected errors.Is to find wrapped error")
	}

	// Test error unwrapping chain
	unwrapped := errors.Unwrap(serverErr)
	if unwrapped != originalErr {
		t.Errorf("Expected unwrapped error to be %v, got %v", originalErr, unwrapped)
	}
}

func TestServerErrorEdgeCases(t *testing.T) {
	// Test with empty strings
	err := &ServerError{
		Type:    "",
		Op:      "",
		Message: "",
		Err:     nil,
	}

	result := err.Error()
	expected := ":  failed: "
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test unwrap with nil
	if err.Unwrap() != nil {
		t.Error("Expected Unwrap to return nil")
	}
}

// Benchmark tests
func BenchmarkNewHTTPError(b *testing.B) {
	err := errors.New("benchmark error")
	for i := 0; i < b.N; i++ {
		_ = NewHTTPError("bench_op", "benchmark message", 500, err)
	}
}

func BenchmarkServerErrorError(b *testing.B) {
	err := NewHTTPError("bench_op", "benchmark message", 500, errors.New("wrapped"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkIsErrorType(b *testing.B) {
	err := NewHTTPError("bench_op", "benchmark message", 500, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsErrorType(err, ErrTypeHTTP)
	}
}

func BenchmarkIsHTTPError(b *testing.B) {
	err := NewHTTPError("bench_op", "benchmark message", 500, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsHTTPError(err)
	}
}

// Example usage tests
func ExampleNewHTTPError() {
	err := NewHTTPError("api_request", "Resource not found", 404, nil)
	fmt.Println(err.Error())
	// Output: http_error: api_request failed: Resource not found
}

func ExampleIsHTTPError() {
	err := NewHTTPError("api_request", "Bad request", 400, nil)
	if IsHTTPError(err) {
		fmt.Println("This is an HTTP error")
	}
	// Output: This is an HTTP error
}

func TestServerErrorImplementsError(t *testing.T) {
	var err error = &ServerError{}
	_ = err // Test that ServerError implements error interface
}

func TestAllErrorTypesHaveConstructors(t *testing.T) {
	// Test that we have constructor functions for all error types
	constructors := map[string]func(string, string, error) *ServerError{
		ErrTypeRouting:  NewRoutingError,
		ErrTypeTemplate: NewTemplateError,
		ErrTypeConfig:   NewConfigError,
		ErrTypeDatabase: NewDatabaseError,
	}

	// Special constructors with different signatures
	_ = NewHTTPError("op", "msg", 404, nil) // HTTP errors have status code
	_ = NewAuthError("op", "msg", nil)      // Auth errors default to 401
	_ = NewInternalError("op", "msg", nil)  // Internal errors default to 500

	// Test regular constructors
	for errorType, constructor := range constructors {
		err := constructor("test_op", "test message", nil)
		if err.Type != errorType {
			t.Errorf("Constructor for %s returned wrong type: %s", errorType, err.Type)
		}
	}
}
