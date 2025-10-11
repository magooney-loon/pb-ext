package server

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestJobMetadataStruct(t *testing.T) {
	now := time.Now()
	metadata := &JobMetadata{
		ID:          "test-job",
		Name:        "Test Job",
		Description: "A test job",
		Expression:  "0 0 * * *",
		IsSystemJob: false,
		CreatedAt:   now,
		IsActive:    true,
		Function:    nil,
	}

	// Test field values
	if metadata.ID != "test-job" {
		t.Errorf("Expected ID 'test-job', got %s", metadata.ID)
	}
	if metadata.Name != "Test Job" {
		t.Errorf("Expected Name 'Test Job', got %s", metadata.Name)
	}
	if metadata.Description != "A test job" {
		t.Errorf("Expected Description 'A test job', got %s", metadata.Description)
	}
	if metadata.Expression != "0 0 * * *" {
		t.Errorf("Expected Expression '0 0 * * *', got %s", metadata.Expression)
	}
	if metadata.IsSystemJob {
		t.Error("Expected IsSystemJob to be false")
	}
	if metadata.CreatedAt != now {
		t.Errorf("Expected CreatedAt %v, got %v", now, metadata.CreatedAt)
	}
	if !metadata.IsActive {
		t.Error("Expected IsActive to be true")
	}
	if metadata.Function != nil {
		t.Error("Expected Function to be nil")
	}
}

func TestJobMetadataZeroValues(t *testing.T) {
	var metadata JobMetadata

	if metadata.ID != "" {
		t.Error("Expected empty ID in zero JobMetadata")
	}
	if metadata.Name != "" {
		t.Error("Expected empty Name in zero JobMetadata")
	}
	if metadata.Description != "" {
		t.Error("Expected empty Description in zero JobMetadata")
	}
	if metadata.Expression != "" {
		t.Error("Expected empty Expression in zero JobMetadata")
	}
	if metadata.IsSystemJob {
		t.Error("Expected IsSystemJob to be false in zero JobMetadata")
	}
	if !metadata.CreatedAt.IsZero() {
		t.Error("Expected zero CreatedAt in zero JobMetadata")
	}
	if metadata.IsActive {
		t.Error("Expected IsActive to be false in zero JobMetadata")
	}
	if metadata.Function != nil {
		t.Error("Expected Function to be nil in zero JobMetadata")
	}
}

func TestJobExecutionResultStruct(t *testing.T) {
	now := time.Now()
	duration := 5 * time.Second
	result := &JobExecutionResult{
		JobID:       "test-job",
		Success:     true,
		Duration:    duration,
		Output:      "Job completed successfully",
		Error:       "",
		TriggerType: "manual",
		TriggerBy:   "user123",
		ExecutedAt:  now,
	}

	// Test field values
	if result.JobID != "test-job" {
		t.Errorf("Expected JobID 'test-job', got %s", result.JobID)
	}
	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.Duration != duration {
		t.Errorf("Expected Duration %v, got %v", duration, result.Duration)
	}
	if result.Output != "Job completed successfully" {
		t.Errorf("Expected Output 'Job completed successfully', got %s", result.Output)
	}
	if result.Error != "" {
		t.Errorf("Expected empty Error, got %s", result.Error)
	}
	if result.TriggerType != "manual" {
		t.Errorf("Expected TriggerType 'manual', got %s", result.TriggerType)
	}
	if result.TriggerBy != "user123" {
		t.Errorf("Expected TriggerBy 'user123', got %s", result.TriggerBy)
	}
	if result.ExecutedAt != now {
		t.Errorf("Expected ExecutedAt %v, got %v", now, result.ExecutedAt)
	}
}

func TestJobExecutionResultZeroValues(t *testing.T) {
	var result JobExecutionResult

	if result.JobID != "" {
		t.Error("Expected empty JobID in zero JobExecutionResult")
	}
	if result.Success {
		t.Error("Expected Success to be false in zero JobExecutionResult")
	}
	if result.Duration != 0 {
		t.Error("Expected Duration to be 0 in zero JobExecutionResult")
	}
	if result.Output != "" {
		t.Error("Expected empty Output in zero JobExecutionResult")
	}
	if result.Error != "" {
		t.Error("Expected empty Error in zero JobExecutionResult")
	}
	if result.TriggerType != "" {
		t.Error("Expected empty TriggerType in zero JobExecutionResult")
	}
	if result.TriggerBy != "" {
		t.Error("Expected empty TriggerBy in zero JobExecutionResult")
	}
	if !result.ExecutedAt.IsZero() {
		t.Error("Expected zero ExecutedAt in zero JobExecutionResult")
	}
}

func TestJobListOptionsStruct(t *testing.T) {
	options := JobListOptions{
		IncludeSystemJobs: true,
		ActiveOnly:        false,
	}

	if !options.IncludeSystemJobs {
		t.Error("Expected IncludeSystemJobs to be true")
	}
	if options.ActiveOnly {
		t.Error("Expected ActiveOnly to be false")
	}
}

func TestJobListOptionsZeroValues(t *testing.T) {
	var options JobListOptions

	if options.IncludeSystemJobs {
		t.Error("Expected IncludeSystemJobs to be false in zero JobListOptions")
	}
	if options.ActiveOnly {
		t.Error("Expected ActiveOnly to be false in zero JobListOptions")
	}
}

func TestSystemJobIDsConstant(t *testing.T) {
	expectedSystemJobs := []string{
		"__pbLogsCleanup__",
		"__pbOTPCleanup__",
		"__pbMFACleanup__",
		"__pbDBOptimize__",
	}

	if len(SystemJobIDs) != len(expectedSystemJobs) {
		t.Errorf("Expected %d system jobs, got %d", len(expectedSystemJobs), len(SystemJobIDs))
	}

	for i, expected := range expectedSystemJobs {
		if i >= len(SystemJobIDs) {
			t.Errorf("Missing system job ID: %s", expected)
			continue
		}
		if SystemJobIDs[i] != expected {
			t.Errorf("Expected system job ID %s, got %s", expected, SystemJobIDs[i])
		}
	}
}

func TestIsSystemJobLogic(t *testing.T) {
	// Mock job manager to test isSystemJob method
	jm := &JobManager{}

	// Test system job IDs
	systemJobs := []string{
		"__pbLogsCleanup__",
		"__pbOTPCleanup__",
		"__pbMFACleanup__",
		"__pbDBOptimize__",
	}

	for _, jobID := range systemJobs {
		if !jm.isSystemJob(jobID) {
			t.Errorf("Expected %s to be identified as system job", jobID)
		}
	}

	// Test non-system job IDs
	userJobs := []string{
		"user-job",
		"daily-cleanup",
		"weekly-stats",
		"custom-task",
		"helloWorld",
		"dailyCleanup",
		"weeklyStats",
	}

	for _, jobID := range userJobs {
		if jm.isSystemJob(jobID) {
			t.Errorf("Expected %s to NOT be identified as system job", jobID)
		}
	}
}

func TestJobManagerZeroValues(t *testing.T) {
	var jm JobManager

	if jm.app != nil {
		t.Error("Expected nil app in zero JobManager")
	}
	if jm.JobLogger != nil {
		t.Error("Expected nil JobLogger in zero JobManager")
	}
	if jm.jobRegistry != nil {
		t.Error("Expected nil jobRegistry in zero JobManager")
	}
}

func TestJobMetadataWithFunction(t *testing.T) {
	executed := false
	jobFunc := func(logger *JobExecutionLogger) {
		executed = true
		logger.Info("Test function executed")
	}

	metadata := &JobMetadata{
		ID:          "func-test",
		Name:        "Function Test",
		Description: "Test job with function",
		Expression:  "0 0 * * *",
		IsSystemJob: false,
		CreatedAt:   time.Now(),
		IsActive:    true,
		Function:    jobFunc,
	}

	if metadata.Function == nil {
		t.Error("Expected Function to be set")
	}

	// Test that the function can be executed
	if metadata.Function != nil {
		logger := NewJobExecutionLogger("test", "test", nil)
		metadata.Function(logger)
		if !executed {
			t.Error("Expected function to be executed")
		}
	}
}

func TestJobExecutionResultSuccess(t *testing.T) {
	result := &JobExecutionResult{
		JobID:    "success-job",
		Success:  true,
		Duration: 2 * time.Second,
		Output:   "Job completed successfully",
		Error:    "",
	}

	if !result.Success {
		t.Error("Expected successful result")
	}
	if result.Error != "" {
		t.Error("Expected no error for successful job")
	}
	if result.Output == "" {
		t.Error("Expected output for successful job")
	}
}

func TestJobExecutionResultFailure(t *testing.T) {
	result := &JobExecutionResult{
		JobID:    "failed-job",
		Success:  false,
		Duration: 1 * time.Second,
		Output:   "",
		Error:    "Job failed: database connection error",
	}

	if result.Success {
		t.Error("Expected failed result")
	}
	if result.Error == "" {
		t.Error("Expected error message for failed job")
	}
}

func TestJobListOptionsVariations(t *testing.T) {
	testCases := []struct {
		name              string
		includeSystemJobs bool
		activeOnly        bool
	}{
		{"All jobs", true, false},
		{"Active jobs only", true, true},
		{"User jobs only", false, false},
		{"Active user jobs", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			options := JobListOptions{
				IncludeSystemJobs: tc.includeSystemJobs,
				ActiveOnly:        tc.activeOnly,
			}

			if options.IncludeSystemJobs != tc.includeSystemJobs {
				t.Errorf("Expected IncludeSystemJobs %v, got %v", tc.includeSystemJobs, options.IncludeSystemJobs)
			}
			if options.ActiveOnly != tc.activeOnly {
				t.Errorf("Expected ActiveOnly %v, got %v", tc.activeOnly, options.ActiveOnly)
			}
		})
	}
}

func TestJobMetadataStringFields(t *testing.T) {
	metadata := &JobMetadata{
		ID:          "string-test",
		Name:        "String Test Job",
		Description: "A job to test string fields handling",
		Expression:  "*/15 * * * *",
	}

	// Test that string fields handle various cases
	if !strings.Contains(metadata.Description, "string fields") {
		t.Error("Expected description to contain 'string fields'")
	}
	if !strings.HasPrefix(metadata.Expression, "*/15") {
		t.Error("Expected expression to start with '*/15'")
	}
}

func TestJobExecutionResultTriggerTypes(t *testing.T) {
	triggerTypes := []string{"manual", "scheduled", "api", "webhook"}

	for _, triggerType := range triggerTypes {
		t.Run(fmt.Sprintf("TriggerType_%s", triggerType), func(t *testing.T) {
			result := &JobExecutionResult{
				JobID:       "trigger-test",
				TriggerType: triggerType,
			}

			if result.TriggerType != triggerType {
				t.Errorf("Expected TriggerType %s, got %s", triggerType, result.TriggerType)
			}
		})
	}
}

func TestJobMetadataTimeFields(t *testing.T) {
	now := time.Now()
	metadata := &JobMetadata{
		ID:        "time-test",
		CreatedAt: now,
	}

	// Test time field precision
	if metadata.CreatedAt != now {
		t.Error("Expected exact time match")
	}

	// Test time is not zero
	if metadata.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt")
	}
}

// Benchmark tests
func BenchmarkIsSystemJob(b *testing.B) {
	jm := &JobManager{}
	jobIDs := []string{"user-job", "__pbLogsCleanup__", "another-user-job", "__pbOTPCleanup__"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobID := jobIDs[i%len(jobIDs)]
		jm.isSystemJob(jobID)
	}
}

func BenchmarkJobMetadataCreation(b *testing.B) {
	jobFunc := func(logger *JobExecutionLogger) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metadata := &JobMetadata{
			ID:          fmt.Sprintf("bench-job-%d", i),
			Name:        "Benchmark Job",
			Description: "A benchmark job",
			Expression:  "0 0 * * *",
			IsSystemJob: false,
			CreatedAt:   time.Now(),
			IsActive:    true,
			Function:    jobFunc,
		}
		_ = metadata
	}
}

func BenchmarkJobExecutionResultCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &JobExecutionResult{
			JobID:       fmt.Sprintf("bench-job-%d", i),
			Success:     true,
			Duration:    time.Duration(i) * time.Millisecond,
			Output:      "Benchmark output",
			Error:       "",
			TriggerType: "manual",
			TriggerBy:   "benchmark",
			ExecutedAt:  time.Now(),
		}
		_ = result
	}
}

// Example usage
func ExampleJobMetadata() {
	jobFunc := func(logger *JobExecutionLogger) {
		logger.Info("Example job running")
	}

	metadata := &JobMetadata{
		ID:          "example-job",
		Name:        "Example Job",
		Description: "An example job for documentation",
		Expression:  "0 0 * * *",
		IsSystemJob: false,
		CreatedAt:   time.Now(),
		IsActive:    true,
		Function:    jobFunc,
	}

	fmt.Printf("Job: %s (%s)", metadata.Name, metadata.Expression)
	// Output: Job: Example Job (0 0 * * *)
}

func ExampleJobListOptions() {
	// Get all jobs including system jobs
	allJobs := JobListOptions{
		IncludeSystemJobs: true,
		ActiveOnly:        false,
	}

	// Get only active user jobs
	activeUserJobs := JobListOptions{
		IncludeSystemJobs: false,
		ActiveOnly:        true,
	}

	fmt.Printf("All jobs: %t, Active user jobs: %t",
		allJobs.IncludeSystemJobs, !activeUserJobs.IncludeSystemJobs && activeUserJobs.ActiveOnly)
	// Output: All jobs: true, Active user jobs: true
}
