package server

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

// JobExecutionLogger provides job-specific logging for individual job runs
type JobExecutionLogger struct {
	jobID       string
	executionID string
	buffer      *bytes.Buffer
	mutex       *sync.Mutex
	startTime   time.Time
	jobLogger   *JobLogger // Reference to main job logging system
}

// NewJobExecutionLogger creates a new job-specific logger instance
func NewJobExecutionLogger(jobID, executionID string, jobLogger *JobLogger) *JobExecutionLogger {
	return &JobExecutionLogger{
		jobID:       jobID,
		executionID: executionID,
		buffer:      &bytes.Buffer{},
		mutex:       &sync.Mutex{},
		startTime:   time.Now(),
		jobLogger:   jobLogger,
	}
}

// Info logs an info-level message for this specific job execution
func (jel *JobExecutionLogger) Info(format string, args ...interface{}) {
	jel.log("INFO", format, args...)
}

// Error logs an error-level message for this specific job execution
func (jel *JobExecutionLogger) Error(format string, args ...interface{}) {
	jel.log("ERROR", format, args...)
}

// Debug logs a debug-level message for this specific job execution
func (jel *JobExecutionLogger) Debug(format string, args ...interface{}) {
	jel.log("DEBUG", format, args...)
}

// Warn logs a warning-level message for this specific job execution
func (jel *JobExecutionLogger) Warn(format string, args ...interface{}) {
	jel.log("WARN", format, args...)
}

// Success logs a success message with emoji for better visibility
func (jel *JobExecutionLogger) Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	jel.log("INFO", "✅ %s", message)
}

// Progress logs a progress message with emoji
func (jel *JobExecutionLogger) Progress(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	jel.log("INFO", "🔄 %s", message)
}

// Start logs the beginning of a job execution
func (jel *JobExecutionLogger) Start(jobName string) {
	jel.log("INFO", "🚀 Starting job: %s", jobName)
}

// Complete logs successful completion of a job
func (jel *JobExecutionLogger) Complete(message string) {
	duration := time.Since(jel.startTime)
	jel.log("INFO", "✅ Job completed successfully in %v: %s", duration, message)
}

// Fail logs job failure with error details
func (jel *JobExecutionLogger) Fail(err error) {
	duration := time.Since(jel.startTime)
	jel.log("ERROR", "❌ Job failed after %v: %v", duration, err)
}

// Statistics logs statistical information
func (jel *JobExecutionLogger) Statistics(stats map[string]interface{}) {
	jel.log("INFO", "📊 Statistics:")
	for key, value := range stats {
		jel.log("INFO", "   • %s: %v", key, value)
	}
}

// log writes a formatted log entry to the buffer with timestamp and job context
func (jel *JobExecutionLogger) log(level, format string, args ...interface{}) {
	jel.mutex.Lock()
	defer jel.mutex.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logEntry := fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level, jel.jobID, message)

	jel.buffer.WriteString(logEntry)
}

// GetOutput returns all accumulated log output for this job execution
func (jel *JobExecutionLogger) GetOutput() string {
	jel.mutex.Lock()
	defer jel.mutex.Unlock()
	return jel.buffer.String()
}

// GetDuration returns the elapsed time since this job execution started
func (jel *JobExecutionLogger) GetDuration() time.Duration {
	return time.Since(jel.startTime)
}

// Flush forces any buffered logs to be written to the main job logging system
func (jel *JobExecutionLogger) Flush() {
	if jel.jobLogger != nil {
		// This would update the job log record with the current output
		// The actual implementation depends on how you want to integrate with the existing JobLogger
	}
}

// WithContext adds contextual information to subsequent log entries
func (jel *JobExecutionLogger) WithContext(key, value string) *JobExecutionLogger {
	// Create a new logger that shares the same underlying resources
	// but has a different job ID for context
	contextLogger := &JobExecutionLogger{
		jobID:       fmt.Sprintf("%s[%s=%s]", jel.jobID, key, value),
		executionID: jel.executionID,
		buffer:      jel.buffer, // Share the same buffer
		mutex:       jel.mutex,  // Share the same mutex reference
		startTime:   jel.startTime,
		jobLogger:   jel.jobLogger,
	}
	return contextLogger
}

// JobLoggerFactory creates job-specific loggers
type JobLoggerFactory struct {
	mainJobLogger *JobLogger
}

// NewJobLoggerFactory creates a new factory for job-specific loggers
func NewJobLoggerFactory(mainJobLogger *JobLogger) *JobLoggerFactory {
	return &JobLoggerFactory{
		mainJobLogger: mainJobLogger,
	}
}

// CreateLogger creates a new job-specific logger for a job execution
func (jlf *JobLoggerFactory) CreateLogger(jobID string) *JobExecutionLogger {
	executionID := fmt.Sprintf("%s_%d", jobID, time.Now().UnixNano())
	return NewJobExecutionLogger(jobID, executionID, jlf.mainJobLogger)
}
