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
	"github.com/pocketbase/pocketbase/tools/cron"
)

// JobManager provides unified job management with automatic logging and execution tracking
type JobManager struct {
	app         core.App
	jobLogger   *JobLogger
	jobRegistry map[string]*JobMetadata
	registryMux sync.RWMutex
}

// JobMetadata represents comprehensive job information
type JobMetadata struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Expression  string    `json:"expression"`
	IsSystemJob bool      `json:"is_system_job"`
	CreatedAt   time.Time `json:"created_at"`
	IsActive    bool      `json:"is_active"`
}

// JobExecutionResult represents the result of job execution
type JobExecutionResult struct {
	JobID       string        `json:"job_id"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Output      string        `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	TriggerType string        `json:"trigger_type"`
	TriggerBy   string        `json:"trigger_by,omitempty"`
	ExecutedAt  time.Time     `json:"executed_at"`
}

// JobListOptions provides filtering options for job listings
type JobListOptions struct {
	IncludeSystemJobs bool
	ActiveOnly        bool
}

// System job constants
var SystemJobIDs = []string{
	"__pbLogsCleanup__",
	"__pbOTPCleanup__",
	"__pbMFACleanup__",
	"__pbDBOptimize__",
}

// NewJobManager creates a new job manager instance
func NewJobManager(app core.App, jobLogger *JobLogger) *JobManager {
	return &JobManager{
		app:         app,
		jobLogger:   jobLogger,
		jobRegistry: make(map[string]*JobMetadata),
	}
}

// RegisterJob registers a new cron job with automatic logging and metadata tracking
func (jm *JobManager) RegisterJob(jobID, jobName, description, expression string, jobFunc func()) error {
	if jobName == "" {
		jobName = jobID
	}

	// Create job metadata
	metadata := &JobMetadata{
		ID:          jobID,
		Name:        jobName,
		Description: description,
		Expression:  expression,
		IsSystemJob: jm.isSystemJob(jobID),
		CreatedAt:   time.Now(),
		IsActive:    true,
	}

	// Register in our metadata registry
	jm.registryMux.Lock()
	jm.jobRegistry[jobID] = metadata
	jm.registryMux.Unlock()

	// Wrap the job function with comprehensive logging and execution tracking
	wrappedFunc := jm.wrapJobFunction(jobID, jobName, description, expression, jobFunc)

	// Register with PocketBase cron system
	if err := jm.app.Cron().Add(jobID, expression, wrappedFunc); err != nil {
		// Remove from registry if cron registration fails
		jm.registryMux.Lock()
		delete(jm.jobRegistry, jobID)
		jm.registryMux.Unlock()
		return fmt.Errorf("failed to register job %s: %w", jobID, err)
	}

	jm.app.Logger().Info("Registered cron job",
		"job_id", jobID,
		"job_name", jobName,
		"description", description,
		"expression", expression,
		"is_system", metadata.IsSystemJob,
	)

	return nil
}

// ExecuteJobManually executes a registered job manually with comprehensive logging
func (jm *JobManager) ExecuteJobManually(jobID, triggerBy string) (*JobExecutionResult, error) {
	// Find the job in the cron system
	jobs := jm.app.Cron().Jobs()
	var targetJob *cron.Job
	for _, job := range jobs {
		if job.Id() == jobID {
			targetJob = job
			break
		}
	}

	if targetJob == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Get job metadata
	jm.registryMux.RLock()
	metadata, exists := jm.jobRegistry[jobID]
	jm.registryMux.RUnlock()

	jobName := jobID
	if exists && metadata.Name != "" {
		jobName = metadata.Name
	}

	// Execute job with comprehensive tracking
	result := &JobExecutionResult{
		JobID:       jobID,
		TriggerType: "manual",
		TriggerBy:   triggerBy,
		ExecutedAt:  time.Now(),
	}

	startTime := time.Now()

	jm.app.Logger().Info("Starting manual job execution",
		"job_id", jobID,
		"job_name", jobName,
		"trigger_by", triggerBy,
	)

	// Get job metadata for logging
	jm.registryMux.RLock()
	jobDescription := ""
	jobExpression := ""
	if metadata, exists := jm.jobRegistry[jobID]; exists {
		jobDescription = metadata.Description
		jobExpression = metadata.Expression
	}
	jm.registryMux.RUnlock()

	// Log job start directly for manual execution
	if jm.jobLogger != nil {
		jm.jobLogger.LogJobStartWithDescription(jobID, jobName, jobDescription, jobExpression, "manual", triggerBy)
	}

	// Execute job with comprehensive output capture and error handling
	capturedOutput, errorMsg := jm.executeJobWithCapture(jobID, jobName, func() {
		// We need to call the original job function without triggering wrapper logging
		// Since we can't easily extract it, we'll call targetJob.Run() but the wrapper
		// will see this is already being logged and skip its own logging
		targetJob.Run()
	})

	// Calculate execution time
	result.Duration = time.Since(startTime)
	result.Output = capturedOutput
	result.Error = errorMsg
	result.Success = errorMsg == ""

	// Log job completion directly for manual execution
	if jm.jobLogger != nil {
		if result.Success {
			jm.jobLogger.LogJobComplete(jobID, capturedOutput, "")
		} else {
			jm.jobLogger.LogJobError(jobID, errorMsg)
		}
	}

	// Application-level logging only (job logging already done above)
	if result.Success {
		jm.app.Logger().Info("Manual job execution completed",
			"job_id", jobID,
			"job_name", jobName,
			"duration", result.Duration,
			"output_length", len(capturedOutput),
		)
	} else {
		jm.app.Logger().Error("Manual job execution failed",
			"job_id", jobID,
			"job_name", jobName,
			"duration", result.Duration,
			"error", errorMsg,
		)
		return result, fmt.Errorf("job execution failed: %s", errorMsg)
	}

	return result, nil
}

// GetJobs returns a list of jobs based on the provided options
func (jm *JobManager) GetJobs(options JobListOptions) []JobMetadata {
	jobs := jm.app.Cron().Jobs()
	result := make([]JobMetadata, 0, len(jobs))

	jm.registryMux.RLock()
	defer jm.registryMux.RUnlock()

	for _, job := range jobs {
		jobID := job.Id()

		// Check if we have metadata for this job
		if metadata, exists := jm.jobRegistry[jobID]; exists {
			// Apply filters
			if !options.IncludeSystemJobs && metadata.IsSystemJob {
				continue
			}
			if options.ActiveOnly && !metadata.IsActive {
				continue
			}

			result = append(result, *metadata)
		} else {
			// Create fallback metadata for jobs not registered through manager
			isSystem := jm.isSystemJob(jobID)
			if !options.IncludeSystemJobs && isSystem {
				continue
			}

			fallbackMetadata := JobMetadata{
				ID:          jobID,
				Name:        jobID,
				Description: "",
				Expression:  job.Expression(),
				IsSystemJob: isSystem,
				CreatedAt:   time.Now(), // We don't know the actual creation time
				IsActive:    true,
			}

			result = append(result, fallbackMetadata)
		}
	}

	return result
}

// GetJobMetadata returns metadata for a specific job
func (jm *JobManager) GetJobMetadata(jobID string) (*JobMetadata, error) {
	jm.registryMux.RLock()
	defer jm.registryMux.RUnlock()

	if metadata, exists := jm.jobRegistry[jobID]; exists {
		// Return a copy to prevent external modification
		metadataCopy := *metadata
		return &metadataCopy, nil
	}

	// Check if job exists in cron system even if not in our registry
	jobs := jm.app.Cron().Jobs()
	for _, job := range jobs {
		if job.Id() == jobID {
			return &JobMetadata{
				ID:          jobID,
				Name:        jobID,
				Description: "",
				Expression:  job.Expression(),
				IsSystemJob: jm.isSystemJob(jobID),
				CreatedAt:   time.Now(), // Unknown creation time
				IsActive:    true,
			}, nil
		}
	}

	return nil, fmt.Errorf("job not found: %s", jobID)
}

// RemoveJob removes a job from both the cron system and our registry
func (jm *JobManager) RemoveJob(jobID string) error {
	// Remove from cron system
	jm.app.Cron().Remove(jobID)

	// Remove from our registry
	jm.registryMux.Lock()
	delete(jm.jobRegistry, jobID)
	jm.registryMux.Unlock()

	jm.app.Logger().Info("Removed job", "job_id", jobID)
	return nil
}

// GetSystemStatus returns the current system status
func (jm *JobManager) GetSystemStatus() map[string]interface{} {
	jobs := jm.app.Cron().Jobs()
	isStarted := jm.app.Cron().HasStarted()

	status := "stopped"
	if isStarted {
		status = "running"
	}

	// Count system vs user jobs
	systemJobs := 0
	userJobs := 0
	for _, job := range jobs {
		if jm.isSystemJob(job.Id()) {
			systemJobs++
		} else {
			userJobs++
		}
	}

	return map[string]interface{}{
		"total_jobs":   len(jobs),
		"system_jobs":  systemJobs,
		"user_jobs":    userJobs,
		"active_jobs":  len(jobs), // All registered PocketBase jobs are active
		"status":       status,
		"has_started":  isStarted,
		"last_updated": time.Now(),
	}
}

// UpdateTimezone updates the cron system timezone
func (jm *JobManager) UpdateTimezone(timezoneStr string) error {
	location, err := time.LoadLocation(timezoneStr)
	if err != nil {
		return fmt.Errorf("invalid timezone %s: %w", timezoneStr, err)
	}

	jm.app.Cron().SetTimezone(location)
	jm.app.Logger().Info("Updated cron timezone", "timezone", timezoneStr)
	return nil
}

// Private methods

// wrapJobFunction wraps a job function with comprehensive logging and execution tracking
func (jm *JobManager) wrapJobFunction(jobID, jobName, description, expression string, originalFunc func()) func() {
	return func() {
		startTime := time.Now()

		// Check if this job is already being logged (e.g., manual execution)
		// by checking if there's already an active job log entry
		jm.jobLogger.activeJobsMux.Lock()
		_, alreadyLogged := jm.jobLogger.activeJobs[jobID]
		jm.jobLogger.activeJobsMux.Unlock()

		// Only log if this is a scheduled execution (not already being logged)
		if !alreadyLogged && jm.jobLogger != nil {
			jm.jobLogger.LogJobStartWithDescription(jobID, jobName, description, expression, "scheduled", "")
		}

		// Execute job with comprehensive output capture
		capturedOutput, errorMsg := jm.executeJobWithCapture(jobID, jobName, originalFunc)
		duration := time.Since(startTime)

		// Only complete logging if we started it (scheduled execution)
		if !alreadyLogged && jm.jobLogger != nil {
			if errorMsg != "" {
				jm.jobLogger.LogJobError(jobID, errorMsg)
			} else {
				jm.jobLogger.LogJobComplete(jobID, capturedOutput, "")
			}
		}

		// Application-level logging
		if errorMsg != "" {
			jm.app.Logger().Error("Job execution failed",
				"job_id", jobID,
				"job_name", jobName,
				"duration", duration,
				"error", errorMsg,
			)
		} else {
			jm.app.Logger().Info("Job execution completed",
				"job_id", jobID,
				"job_name", jobName,
				"duration", duration,
				"output_length", len(capturedOutput),
			)
		}
	}
}

// executeJobWithCapture executes a job function with comprehensive output capture
func (jm *JobManager) executeJobWithCapture(jobID, jobName string, jobFunc func()) (output string, errorMsg string) {
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

// isSystemJob checks if a job ID belongs to PocketBase system jobs
func (jm *JobManager) isSystemJob(jobID string) bool {
	for _, sysJob := range SystemJobIDs {
		if jobID == sysJob {
			return true
		}
	}
	return false
}

// LogCapture captures log output to a buffer (reused from job_wrapper.go)
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

// Global job manager instance
var globalJobManager *JobManager

// InitializeJobManager initializes the global job manager
func InitializeJobManager(app core.App, jobLogger *JobLogger) {
	globalJobManager = NewJobManager(app, jobLogger)
}

// GetJobManager returns the global job manager instance
func GetJobManager() *JobManager {
	return globalJobManager
}
