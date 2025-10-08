package api

// =============================================================================
// Common Interfaces
// =============================================================================

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// DefaultLogger provides a default logger implementation
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) { /* no-op or implement */ }
func (l *DefaultLogger) Info(msg string, args ...interface{})  { /* no-op or implement */ }
func (l *DefaultLogger) Warn(msg string, args ...interface{})  { /* no-op or implement */ }
func (l *DefaultLogger) Error(msg string, args ...interface{}) { /* no-op or implement */ }
func (l *DefaultLogger) Log(msg string)                        { /* no-op or implement */ }
