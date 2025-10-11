package server

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
)

// JobManager provides unified job management with automatic logging and execution tracking
type JobManager struct {
	app         core.App
	JobLogger   *JobLogger // Made public so factory can access it
	jobRegistry map[string]*JobMetadata
	registryMux sync.RWMutex
}

// JobMetadata represents comprehensive job information
type JobMetadata struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Expression  string                    `json:"expression"`
	IsSystemJob bool                      `json:"is_system_job"`
	CreatedAt   time.Time                 `json:"created_at"`
	IsActive    bool                      `json:"is_active"`
	Function    func(*JobExecutionLogger) `json:"-"` // Store original function, exclude from JSON
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
		JobLogger:   jobLogger,
		jobRegistry: make(map[string]*JobMetadata),
	}
}

// RegisterJob registers a new cron job with automatic logging and metadata tracking
func (jm *JobManager) RegisterJob(jobID, jobName, description, expression string, jobFunc func(*JobExecutionLogger)) error {
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
		Function:    jobFunc, // Store the original function
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
	if jm.JobLogger != nil {
		jm.JobLogger.LogJobStartWithDescription(jobID, jobName, jobDescription, jobExpression, "manual", triggerBy)
	}

	// Create job-specific logger for this execution
	jobLoggerFactory := NewJobLoggerFactory(jm.JobLogger)
	jobExecutionLogger := jobLoggerFactory.CreateLogger(jobID)

	// Execute job with error handling
	var errorMsg string
	capturedOutput := ""
	func() {
		defer func() {
			if r := recover(); r != nil {
				errorMsg = fmt.Sprintf("Job panic: %v", r)
				jobExecutionLogger.Fail(fmt.Errorf("%s", errorMsg))
			}
		}()

		// Get the original job function from metadata and execute it with the logger
		if metadata, exists := jm.jobRegistry[jobID]; exists && metadata.Function != nil {
			metadata.Function(jobExecutionLogger)
		} else {
			// Fallback to running the wrapped job if original function not available
			targetJob.Run()
		}

		// Get the captured output from the job execution logger
		capturedOutput = jobExecutionLogger.GetOutput()
	}()

	// Calculate execution time
	result.Duration = time.Since(startTime)
	result.Output = capturedOutput
	result.Error = errorMsg
	result.Success = errorMsg == ""

	// Log job completion directly for manual execution
	if jm.JobLogger != nil {
		if result.Success {
			jm.JobLogger.LogJobComplete(jobID, capturedOutput, "")
		} else {
			jm.JobLogger.LogJobError(jobID, errorMsg)
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
func (jm *JobManager) wrapJobFunction(jobID, jobName, description, expression string, originalFunc func(*JobExecutionLogger)) func() {
	return func() {
		startTime := time.Now()

		// Check if this job is already being logged (e.g., manual execution)
		// by checking if there's already an active job log entry
		jm.JobLogger.activeJobsMux.Lock()
		_, alreadyLogged := jm.JobLogger.activeJobs[jobID]
		jm.JobLogger.activeJobsMux.Unlock()

		// Only log if this is a scheduled execution (not already being logged)
		if !alreadyLogged && jm.JobLogger != nil {
			jm.JobLogger.LogJobStartWithDescription(jobID, jobName, description, expression, "scheduled", "")
		}

		// Create job-specific logger for this execution
		jobLoggerFactory := NewJobLoggerFactory(jm.JobLogger)
		jobExecutionLogger := jobLoggerFactory.CreateLogger(jobID)

		// Execute job with error handling
		var errorMsg string
		func() {
			defer func() {
				if r := recover(); r != nil {
					errorMsg = fmt.Sprintf("Job panic: %v", r)
					jobExecutionLogger.Fail(fmt.Errorf("%s", errorMsg))
				}
			}()

			// Execute the original job function with the job-specific logger
			originalFunc(jobExecutionLogger)
		}()

		// Get the captured output from the job execution logger
		capturedOutput := jobExecutionLogger.GetOutput()
		duration := time.Since(startTime)

		// Only complete logging if we started it (scheduled execution)
		if !alreadyLogged && jm.JobLogger != nil {
			if errorMsg != "" {
				jm.JobLogger.LogJobError(jobID, errorMsg)
			} else {
				jm.JobLogger.LogJobComplete(jobID, capturedOutput, "")
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

// executeJobWithCapture is simplified since we use job-specific loggers
func (jm *JobManager) executeJobWithCapture(jobID, jobName string, jobFunc func()) (output string, errorMsg string) {
	// Create job-specific logger for this execution
	jobLoggerFactory := NewJobLoggerFactory(jm.JobLogger)
	jobExecutionLogger := jobLoggerFactory.CreateLogger(jobID)

	defer func() {
		if r := recover(); r != nil {
			errorMsg = fmt.Sprintf("Job panic: %v", r)
			jobExecutionLogger.Fail(fmt.Errorf("%s", errorMsg))
		}
	}()

	// Execute the job function
	jobFunc()

	// Get the output from the job execution logger
	output = jobExecutionLogger.GetOutput()
	if output == "" {
		output = fmt.Sprintf("Job %s executed successfully", jobName)
	}

	return output, errorMsg
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

// LogCapture is kept for compatibility but simplified
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
