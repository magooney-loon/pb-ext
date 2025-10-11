package server

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewJobExecutionLogger(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	if logger.jobID != "test-job" {
		t.Errorf("Expected jobID to be 'test-job', got %s", logger.jobID)
	}

	if logger.executionID != "exec-123" {
		t.Errorf("Expected executionID to be 'exec-123', got %s", logger.executionID)
	}

	if logger.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}

	if logger.mutex == nil {
		t.Error("Expected mutex to be initialized")
	}

	if logger.startTime.IsZero() {
		t.Error("Expected startTime to be set")
	}
}

func TestJobExecutionLoggerInfo(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Info("This is an info message")

	output := logger.GetOutput()
	if !strings.Contains(output, "[INFO]") {
		t.Error("Expected output to contain [INFO] level")
	}

	if !strings.Contains(output, "[test-job]") {
		t.Error("Expected output to contain job ID")
	}

	if !strings.Contains(output, "This is an info message") {
		t.Error("Expected output to contain the logged message")
	}
}

func TestJobExecutionLoggerError(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Error("This is an error: %s", "something went wrong")

	output := logger.GetOutput()
	if !strings.Contains(output, "[ERROR]") {
		t.Error("Expected output to contain [ERROR] level")
	}

	if !strings.Contains(output, "This is an error: something went wrong") {
		t.Error("Expected output to contain formatted error message")
	}
}

func TestJobExecutionLoggerMultipleLevels(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Info("Info message")
	logger.Error("Error message")
	logger.Debug("Debug message")
	logger.Warn("Warning message")

	output := logger.GetOutput()

	levels := []string{"[INFO]", "[ERROR]", "[DEBUG]", "[WARN]"}
	for _, level := range levels {
		if !strings.Contains(output, level) {
			t.Errorf("Expected output to contain %s level", level)
		}
	}

	messages := []string{"Info message", "Error message", "Debug message", "Warning message"}
	for _, message := range messages {
		if !strings.Contains(output, message) {
			t.Errorf("Expected output to contain message: %s", message)
		}
	}
}

func TestJobExecutionLoggerSuccess(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Success("Task completed successfully")

	output := logger.GetOutput()
	if !strings.Contains(output, "✅") {
		t.Error("Expected output to contain success emoji")
	}

	if !strings.Contains(output, "Task completed successfully") {
		t.Error("Expected output to contain success message")
	}
}

func TestJobExecutionLoggerProgress(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Progress("Processing items")

	output := logger.GetOutput()
	if !strings.Contains(output, "🔄") {
		t.Error("Expected output to contain progress emoji")
	}

	if !strings.Contains(output, "Processing items") {
		t.Error("Expected output to contain progress message")
	}
}

func TestJobExecutionLoggerStart(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Start("My Test Job")

	output := logger.GetOutput()
	if !strings.Contains(output, "🚀") {
		t.Error("Expected output to contain start emoji")
	}

	if !strings.Contains(output, "Starting job: My Test Job") {
		t.Error("Expected output to contain start message with job name")
	}
}

func TestJobExecutionLoggerComplete(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	// Wait a bit to have some duration
	time.Sleep(10 * time.Millisecond)

	logger.Complete("All tasks finished")

	output := logger.GetOutput()
	if !strings.Contains(output, "✅") {
		t.Error("Expected output to contain completion emoji")
	}

	if !strings.Contains(output, "Job completed successfully") {
		t.Error("Expected output to contain completion message")
	}

	if !strings.Contains(output, "All tasks finished") {
		t.Error("Expected output to contain custom completion message")
	}

	// Check that duration is included
	if !strings.Contains(output, "in ") {
		t.Error("Expected output to contain duration information")
	}
}

func TestJobExecutionLoggerFail(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	// Wait a bit to have some duration
	time.Sleep(10 * time.Millisecond)

	logger.Fail(fmt.Errorf("database connection failed"))

	output := logger.GetOutput()
	if !strings.Contains(output, "❌") {
		t.Error("Expected output to contain failure emoji")
	}

	if !strings.Contains(output, "Job failed after") {
		t.Error("Expected output to contain failure message with duration")
	}

	if !strings.Contains(output, "database connection failed") {
		t.Error("Expected output to contain error details")
	}
}

func TestJobExecutionLoggerStatistics(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	stats := map[string]interface{}{
		"processed":    100,
		"failed":       5,
		"success_rate": 95.0,
	}

	logger.Statistics(stats)

	output := logger.GetOutput()
	if !strings.Contains(output, "📊") {
		t.Error("Expected output to contain statistics emoji")
	}

	if !strings.Contains(output, "Statistics:") {
		t.Error("Expected output to contain statistics header")
	}

	// Check that all stats are logged
	expectedStats := []string{"processed: 100", "failed: 5", "success_rate: 95"}
	for _, stat := range expectedStats {
		if !strings.Contains(output, stat) {
			t.Errorf("Expected output to contain statistic: %s", stat)
		}
	}
}

func TestJobExecutionLoggerGetDuration(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	// Wait a bit to have measurable duration
	time.Sleep(50 * time.Millisecond)

	duration := logger.GetDuration()
	if duration < 50*time.Millisecond {
		t.Errorf("Expected duration to be at least 50ms, got %v", duration)
	}
}

func TestJobExecutionLoggerWithContext(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	contextLogger := logger.WithContext("user_id", "123")
	contextLogger.Info("Processing user data")

	output := logger.GetOutput() // Should be shared buffer
	if !strings.Contains(output, "[test-job[user_id=123]]") {
		t.Error("Expected output to contain contextualized job ID")
	}

	if !strings.Contains(output, "Processing user data") {
		t.Error("Expected output to contain contextual message")
	}
}

func TestJobExecutionLoggerMultipleContexts(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	userLogger := logger.WithContext("user_id", "123")
	actionLogger := userLogger.WithContext("action", "update")

	actionLogger.Info("Updating user profile")

	output := logger.GetOutput()
	if !strings.Contains(output, "[test-job[user_id=123][action=update]]") {
		t.Error("Expected output to contain nested context in job ID")
	}
}

func TestJobExecutionLoggerThreadSafety(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	// Run multiple goroutines concurrently writing to the logger
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("Message from goroutine %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	output := logger.GetOutput()

	// Check that all messages are present
	for i := 0; i < 10; i++ {
		expectedMsg := fmt.Sprintf("Message from goroutine %d", i)
		if !strings.Contains(output, expectedMsg) {
			t.Errorf("Expected output to contain message: %s", expectedMsg)
		}
	}
}

func TestJobExecutionLoggerTimestampFormat(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	logger.Info("Test message")

	output := logger.GetOutput()

	// Check that timestamp format is correct (YYYY-MM-DD HH:MM:SS.mmm)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("Expected at least one log line")
	}

	firstLine := lines[0]
	if !strings.HasPrefix(firstLine, "[20") { // Year should start with 20
		t.Error("Expected timestamp to start with year in 20XX format")
	}

	// Check for timestamp pattern: [YYYY-MM-DD HH:MM:SS.mmm]
	timestampPattern := `\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}\]`
	matched, _ := regexp.MatchString(timestampPattern, firstLine)
	if !matched {
		t.Errorf("Expected timestamp format to match pattern, got: %s", firstLine)
	}
}

func TestJobLoggerFactory(t *testing.T) {
	factory := NewJobLoggerFactory(nil)

	if factory.mainJobLogger != nil {
		t.Error("Expected mainJobLogger to be nil")
	}

	logger := factory.CreateLogger("test-job")

	if logger.jobID != "test-job" {
		t.Errorf("Expected jobID to be 'test-job', got %s", logger.jobID)
	}

	if !strings.HasPrefix(logger.executionID, "test-job_") {
		t.Error("Expected executionID to start with job ID")
	}
}

func TestJobExecutionLoggerZeroValues(t *testing.T) {
	var logger JobExecutionLogger

	if logger.jobID != "" {
		t.Error("Expected empty jobID in zero logger")
	}
	if logger.executionID != "" {
		t.Error("Expected empty executionID in zero logger")
	}
	if logger.buffer != nil {
		t.Error("Expected nil buffer in zero logger")
	}
	if logger.mutex != nil {
		t.Error("Expected nil mutex in zero logger")
	}
	if !logger.startTime.IsZero() {
		t.Error("Expected zero startTime in zero logger")
	}
}

func TestJobExecutionLoggerEmptyOutput(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	output := logger.GetOutput()
	if output != "" {
		t.Errorf("Expected empty output initially, got: %s", output)
	}
}

func TestJobExecutionLoggerFormattedMessages(t *testing.T) {
	logger := NewJobExecutionLogger("test-job", "exec-123", nil)

	// Test formatted messages
	logger.Info("Processing %d items", 42)
	logger.Error("Failed to connect to %s:%d", "localhost", 5432)

	output := logger.GetOutput()
	if !strings.Contains(output, "Processing 42 items") {
		t.Error("Expected formatted info message")
	}
	if !strings.Contains(output, "Failed to connect to localhost:5432") {
		t.Error("Expected formatted error message")
	}
}

// Benchmark tests
func BenchmarkJobExecutionLoggerInfo(b *testing.B) {
	logger := NewJobExecutionLogger("benchmark-job", "exec-123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message %d", i)
	}
}

func BenchmarkJobExecutionLoggerConcurrent(b *testing.B) {
	logger := NewJobExecutionLogger("benchmark-job", "exec-123", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			logger.Info("Concurrent message %d", i)
			i++
		}
	})
}

func BenchmarkJobExecutionLoggerWithContext(b *testing.B) {
	logger := NewJobExecutionLogger("benchmark-job", "exec-123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contextLogger := logger.WithContext("user", fmt.Sprintf("user-%d", i))
		contextLogger.Info("Context message")
	}
}

func BenchmarkNewJobExecutionLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		logger := NewJobExecutionLogger("test-job", "exec-123", nil)
		_ = logger
	}
}

// Example usage
func ExampleJobExecutionLogger() {
	logger := NewJobExecutionLogger("example-job", "exec-001", nil)

	logger.Start("Example Job")
	logger.Info("Processing data...")
	logger.Progress("50%% complete")
	logger.Success("Data processed successfully")
	logger.Complete("Job finished")

	output := logger.GetOutput()
	fmt.Printf("Output contains logs: %t", len(output) > 0)
	// Output: Output contains logs: true
}

func ExampleJobExecutionLogger_WithContext() {
	logger := NewJobExecutionLogger("user-job", "exec-002", nil)

	userLogger := logger.WithContext("user_id", "12345")
	userLogger.Info("Starting user processing")

	actionLogger := userLogger.WithContext("action", "update")
	actionLogger.Success("User profile updated")

	// All output goes to the same buffer
	output := logger.GetOutput()
	fmt.Printf("Context preserved: %t", strings.Contains(output, "[user-job[user_id=12345][action=update]]"))
	// Output: Context preserved: true
}
