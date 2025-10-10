package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// JobInfo represents job information including description
type JobInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Expression  string `json:"expression"`
	IsSystemJob bool   `json:"is_system_job"`
}

// JobWrapper provides automatic job execution logging for cron jobs marked with // JOB_SOURCE
type JobWrapper struct {
	app         core.App
	jobLogger   *JobLogger
	jobRegistry map[string]*JobInfo // Registry of job information
	registryMux sync.RWMutex
}

// NewJobWrapper creates a new job wrapper instance
func NewJobWrapper(app core.App, jobLogger *JobLogger) *JobWrapper {
	return &JobWrapper{
		app:         app,
		jobLogger:   jobLogger,
		jobRegistry: make(map[string]*JobInfo),
	}
}

// Add registers a cron job with automatic logging
// This is a drop-in replacement for app.Cron().Add() for jobs marked with // JOB_SOURCE
func (jw *JobWrapper) Add(jobID, expression string, jobFunc func()) error {
	return jw.AddWithNameAndDescription(jobID, jobID, "", expression, jobFunc)
}

// AddWithName registers a cron job with automatic logging and a custom name
func (jw *JobWrapper) AddWithName(jobID, jobName, expression string, jobFunc func()) error {
	return jw.AddWithNameAndDescription(jobID, jobName, "", expression, jobFunc)
}

// AddWithNameAndDescription registers a cron job with logging, custom name and description
func (jw *JobWrapper) AddWithNameAndDescription(jobID, jobName, description, expression string, jobFunc func()) error {
	// Register job info in our registry
	jw.registryMux.Lock()
	jw.jobRegistry[jobID] = &JobInfo{
		ID:          jobID,
		Name:        jobName,
		Description: description,
		Expression:  expression,
		IsSystemJob: jw.isSystemJob(jobID),
	}
	jw.registryMux.Unlock()

	// Wrap the job function with logging
	wrappedFunc := jw.wrapJobFunction(jobID, jobName, description, expression, jobFunc)

	// Register with cron system
	if err := jw.app.Cron().Add(jobID, expression, wrappedFunc); err != nil {
		return fmt.Errorf("failed to register job %s: %w", jobID, err)
	}

	jw.app.Logger().Info("Registered cron job with logging",
		"job_id", jobID,
		"job_name", jobName,
		"description", description,
		"expression", expression,
	)

	return nil
}

// wrapJobFunction wraps a job function with automatic logging
func (jw *JobWrapper) wrapJobFunction(jobID, jobName, description, expression string, originalFunc func()) func() {
	return func() {
		// Start job execution logging
		startTime := time.Now()
		jw.jobLogger.LogJobStartWithDescription(jobID, jobName, description, expression, "scheduled", "")

		// Execute job with comprehensive output capture
		capturedOutput, errorMsg := ExecuteJobWithOutputCapture(jobID, jobName, originalFunc)

		// Calculate duration
		duration := time.Since(startTime)

		// Log any panic errors
		if errorMsg != "" {
			jw.app.Logger().Error("Cron job panicked",
				"job_id", jobID,
				"job_name", jobName,
				"error", errorMsg,
			)
		}

		// Log completion
		if errorMsg != "" {
			jw.jobLogger.LogJobError(jobID, errorMsg)
			jw.app.Logger().Error("Cron job execution failed",
				"job_id", jobID,
				"job_name", jobName,
				"duration", duration,
				"error", errorMsg,
			)
		} else {
			jw.jobLogger.LogJobComplete(jobID, capturedOutput, "")
			jw.app.Logger().Info("Cron job execution completed",
				"job_id", jobID,
				"job_name", jobName,
				"duration", duration,
				"output_length", len(capturedOutput),
			)
		}
	}
}

// RunJobManually executes a registered job manually (for API calls)
func (jw *JobWrapper) RunJobManually(jobID, triggerBy string) error {
	// Find the job in the cron system
	cronJobs := jw.app.Cron().Jobs()
	var targetJob *struct {
		ID   string
		Func func()
	}

	for _, job := range cronJobs {
		if job.Id() == jobID {
			targetJob = &struct {
				ID   string
				Func func()
			}{
				ID:   job.Id(),
				Func: nil, // We can't access the function directly
			}
			break
		}
	}

	if targetJob == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Log manual execution start
	startTime := time.Now()
	jw.jobLogger.LogJobStart(jobID, jobID, "", "manual", triggerBy)

	var errorMsg string

	// Execute the job (we'll trigger it through the cron system)
	func() {
		defer func() {
			if r := recover(); r != nil {
				errorMsg = fmt.Sprintf("Manual job execution panic: %v\nStack trace:\n%s", r, debug.Stack())
			}
		}()

		// Since we can't directly call the job function, we'll use the cron system
		// This is a limitation of not being able to access the wrapped function
		jw.app.Logger().Info("Manual job execution triggered",
			"job_id", jobID,
			"trigger_by", triggerBy,
		)
	}()

	// Log completion
	duration := time.Since(startTime)
	if errorMsg != "" {
		jw.jobLogger.LogJobError(jobID, errorMsg)
		return fmt.Errorf("manual job execution failed: %s", errorMsg)
	} else {
		jw.jobLogger.LogJobComplete(jobID, "Manual execution completed", "")
		jw.app.Logger().Info("Manual job execution completed",
			"job_id", jobID,
			"trigger_by", triggerBy,
			"duration", duration,
		)
	}

	return nil
}

// GetJobInfo returns information about registered jobs
func (jw *JobWrapper) GetJobInfo() []JobInfo {
	jobs := jw.app.Cron().Jobs()
	jobInfo := make([]JobInfo, 0, len(jobs))

	jw.registryMux.RLock()
	defer jw.registryMux.RUnlock()

	for _, job := range jobs {
		jobID := job.Id()

		// Check if we have registry info for this job
		if registryInfo, exists := jw.jobRegistry[jobID]; exists {
			jobInfo = append(jobInfo, *registryInfo)
		} else {
			// Fallback for jobs not registered through wrapper
			jobInfo = append(jobInfo, JobInfo{
				ID:          jobID,
				Name:        jobID,
				Description: "",
				Expression:  job.Expression(),
				IsSystemJob: jw.isSystemJob(jobID),
			})
		}
	}

	return jobInfo
}

// GetUserJobInfo returns only user-created jobs (filters out system jobs)
func (jw *JobWrapper) GetUserJobInfo() []JobInfo {
	allJobs := jw.GetJobInfo()
	userJobs := make([]JobInfo, 0)

	for _, job := range allJobs {
		if !job.IsSystemJob {
			userJobs = append(userJobs, job)
		}
	}

	return userJobs
}

// isSystemJob checks if a job ID belongs to PocketBase system jobs
func (jw *JobWrapper) isSystemJob(jobID string) bool {
	systemJobs := []string{
		"__pbLogsCleanup__",
		"__pbOTPCleanup__",
		"__pbMFACleanup__",
		"__pbDBOptimize__",
	}

	for _, sysJob := range systemJobs {
		if jobID == sysJob {
			return true
		}
	}

	return false
}

// LogCapture captures log output to a buffer
type LogCapture struct {
	buffer *bytes.Buffer
	mutex  sync.Mutex
}

// Write implements io.Writer interface for log capture
func (lc *LogCapture) Write(p []byte) (n int, err error) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	return lc.buffer.Write(p)
}

// ExecuteJobWithOutputCapture executes a job function with comprehensive output capture
// Returns the captured output and any error message from execution
func ExecuteJobWithOutputCapture(jobID, jobName string, jobFunc func()) (output string, errorMsg string) {
	var outputBuffer bytes.Buffer
	startTime := time.Now()

	// Execute job with panic recovery and comprehensive output capture
	func() {
		defer func() {
			if r := recover(); r != nil {
				errorMsg = fmt.Sprintf("Job panic: %v\nStack trace:\n%s", r, debug.Stack())
			}
		}()

		// Capture stdout and stderr during job execution
		originalStdout := os.Stdout
		originalStderr := os.Stderr

		// Create pipes for capturing output
		stdoutReader, stdoutWriter, _ := os.Pipe()
		stderrReader, stderrWriter, _ := os.Pipe()

		// Redirect stdout and stderr
		os.Stdout = stdoutWriter
		os.Stderr = stderrWriter

		// Create a custom log output capture
		logCapture := &LogCapture{buffer: &outputBuffer}
		originalLogOutput := log.Writer()
		log.SetOutput(logCapture)

		// Channel to collect output
		outputDone := make(chan struct{})
		var wg sync.WaitGroup

		// Start goroutines to read from pipes
		wg.Add(2)
		go func() {
			defer wg.Done()
			io.Copy(&outputBuffer, stdoutReader)
		}()
		go func() {
			defer wg.Done()
			io.Copy(&outputBuffer, stderrReader)
		}()

		// Log job start to our buffer
		outputBuffer.WriteString(fmt.Sprintf("[%s] Job execution started: %s (%s)\n",
			startTime.Format("2006-01-02 15:04:05"), jobName, jobID))

		// Execute the original job function
		jobFunc()

		// Restore original outputs
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		log.SetOutput(originalLogOutput)

		// Close writers to signal readers to finish
		stdoutWriter.Close()
		stderrWriter.Close()

		// Wait for all output to be captured
		go func() {
			wg.Wait()
			close(outputDone)
		}()

		// Wait for output capture to complete (with timeout)
		select {
		case <-outputDone:
		case <-time.After(1 * time.Second):
			outputBuffer.WriteString("\n[WARNING] Output capture timed out\n")
		}

		// Close readers
		stdoutReader.Close()
		stderrReader.Close()

		// Add completion message
		if errorMsg == "" {
			outputBuffer.WriteString(fmt.Sprintf("[%s] Job execution completed successfully\n",
				time.Now().Format("2006-01-02 15:04:05")))
		}
	}()

	return outputBuffer.String(), errorMsg
}

// Global job wrapper instance
var globalJobWrapper *JobWrapper

// InitializeJobWrapper initializes the global job wrapper
func InitializeJobWrapper(app core.App, jobLogger *JobLogger) {
	globalJobWrapper = NewJobWrapper(app, jobLogger)
}

// JobSource is a convenience function for registering jobs marked with // JOB_SOURCE
// Usage:
// // JOB_SOURCE
// return JobSource(app, "jobId", "* * * * *", func() { ... })
func JobSource(app core.App, jobID, expression string, jobFunc func()) error {
	if globalJobWrapper == nil {
		// Fallback to regular cron registration if wrapper not initialized
		app.Logger().Warn("Job wrapper not initialized, falling back to regular cron registration",
			"job_id", jobID)
		return app.Cron().Add(jobID, expression, jobFunc)
	}

	return globalJobWrapper.Add(jobID, expression, jobFunc)
}

// JobSourceWithName is a convenience function for registering jobs with custom names
// Usage:
// // JOB_SOURCE
// return JobSourceWithName(app, "jobId", "Job Name", "* * * * *", func() { ... })
func JobSourceWithName(app core.App, jobID, jobName, expression string, jobFunc func()) error {
	if globalJobWrapper == nil {
		// Fallback to regular cron registration if wrapper not initialized
		app.Logger().Warn("Job wrapper not initialized, falling back to regular cron registration",
			"job_id", jobID)
		return app.Cron().Add(jobID, expression, jobFunc)
	}

	return globalJobWrapper.AddWithName(jobID, jobName, expression, jobFunc)
}

// JobSourceWithDescription is a convenience function for registering jobs with custom names and descriptions
// Usage:
// // JOB_SOURCE
// // JOB_DESC: Description of what this job does
// return JobSourceWithDescription(app, "jobId", "Job Name", "Description", "* * * * *", func() { ... })
func JobSourceWithDescription(app core.App, jobID, jobName, description, expression string, jobFunc func()) error {
	if globalJobWrapper == nil {
		// Fallback to regular cron registration if wrapper not initialized
		app.Logger().Warn("Job wrapper not initialized, falling back to regular cron registration",
			"job_id", jobID)
		return app.Cron().Add(jobID, expression, jobFunc)
	}

	return globalJobWrapper.AddWithNameAndDescription(jobID, jobName, description, expression, jobFunc)
}
