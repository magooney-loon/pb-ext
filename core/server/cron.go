package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
)

// CronStatus constants
const (
	CronStatusRunning = "running"
	CronStatusStopped = "stopped"
	CronStatusError   = "error"
)

// CronResponse represents a standard API response for cron operations
type CronResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
}

// CronJobData represents cron job data structure
type CronJobData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Expression  string `json:"expression"`
	Active      bool   `json:"active"`
	IsSystemJob bool   `json:"is_system_job"`
}

// CronSystemStatus represents the system status response
type CronSystemStatus struct {
	TotalJobs  int    `json:"total_jobs"`
	ActiveJobs int    `json:"active_jobs"`
	Status     string `json:"status"`
	HasStarted bool   `json:"has_started"`
}

// RegisterCronRoutes registers cron jobs API endpoints with improved error handling and structure
func (s *Server) RegisterCronRoutes(e *core.ServeEvent) {
	// Get cron jobs list
	e.Router.GET("/api/cron/jobs", func(c *core.RequestEvent) error {
		return s.handleGetCronJobs(c)
	}).Bind(apis.RequireSuperuserAuth())

	// Run a cron job immediately
	e.Router.POST("/api/cron/jobs/{id}/run", func(c *core.RequestEvent) error {
		return s.handleRunCronJob(c)
	}).Bind(apis.RequireSuperuserAuth())

	// Get cron system status
	e.Router.GET("/api/cron/status", func(c *core.RequestEvent) error {
		return s.handleGetCronStatus(c)
	}).Bind(apis.RequireSuperuserAuth())

	// Timezone configuration endpoint
	e.Router.POST("/api/cron/config/timezone", func(c *core.RequestEvent) error {
		return s.handleUpdateTimezone(c)
	}).Bind(apis.RequireSuperuserAuth())

	// Job deletion endpoint
	e.Router.DELETE("/api/cron/jobs/{id}", func(c *core.RequestEvent) error {
		return s.handleDeleteCronJob(c)
	}).Bind(apis.RequireSuperuserAuth())

	// Job logs endpoints
	e.Router.GET("/api/cron/logs", func(c *core.RequestEvent) error {
		return s.handleGetJobLogs(c)
	}).Bind(apis.RequireSuperuserAuth())

	e.Router.GET("/api/cron/logs/{job_id}", func(c *core.RequestEvent) error {
		return s.handleGetJobLogsByJobID(c)
	}).Bind(apis.RequireSuperuserAuth())

	e.Router.GET("/api/cron/logs/analytics", func(c *core.RequestEvent) error {
		return s.handleGetJobLogsAnalytics(c)
	}).Bind(apis.RequireSuperuserAuth())
}

// Handler functions with improved error handling and validation

// handleGetCronJobs retrieves all cron jobs with optional system job filtering
func (s *Server) handleGetCronJobs(c *core.RequestEvent) error {
	// Check for system jobs filter parameter
	showSystemJobs := true
	if filter := c.Request.URL.Query().Get("show_system"); filter == "false" {
		showSystemJobs = false
	}

	var jobsData []CronJobData

	// Use job wrapper if available for enhanced job info
	if globalJobWrapper != nil {
		var jobInfos []JobInfo
		if showSystemJobs {
			jobInfos = globalJobWrapper.GetJobInfo()
		} else {
			jobInfos = globalJobWrapper.GetUserJobInfo()
		}

		jobsData = make([]CronJobData, len(jobInfos))
		for i, jobInfo := range jobInfos {
			jobsData[i] = CronJobData{
				ID:          jobInfo.ID,
				Name:        jobInfo.Name,
				Description: jobInfo.Description,
				Expression:  jobInfo.Expression,
				Active:      true, // PocketBase jobs are always active when registered
				IsSystemJob: jobInfo.IsSystemJob,
			}
		}
	} else {
		// Fallback to basic job info if wrapper not available
		jobs := c.App.Cron().Jobs()
		jobsData = make([]CronJobData, 0, len(jobs))

		for _, job := range jobs {
			jobID := job.Id()
			isSystem := s.isSystemJob(jobID)

			// Filter system jobs if requested
			if !showSystemJobs && isSystem {
				continue
			}

			jobsData = append(jobsData, CronJobData{
				ID:          jobID,
				Name:        jobID, // Use ID as name by default
				Description: "",
				Expression:  job.Expression(),
				Active:      true,
				IsSystemJob: isSystem,
			})
		}
	}

	c.App.Logger().Debug("Retrieved cron jobs",
		"count", len(jobsData),
		"show_system", showSystemJobs)
	return c.JSON(200, jobsData)
}

// handleRunCronJob executes a specific job immediately with logging
func (s *Server) handleRunCronJob(c *core.RequestEvent) error {
	jobId := strings.TrimSpace(c.Request.PathValue("id"))
	if jobId == "" {
		return s.cronError(c, 400, "Job ID is required", nil)
	}

	// Get the user ID for logging (if available)
	triggerBy := ""
	if c.Auth != nil {
		triggerBy = c.Auth.Id
	}

	jobs := c.App.Cron().Jobs()
	var targetJob *cron.Job
	for _, job := range jobs {
		if job.Id() == jobId {
			targetJob = job
			break
		}
	}

	if targetJob == nil {
		return s.cronError(c, 404, "Job not found", map[string]string{"job_id": jobId})
	}

	// Execute job with logging if job logger is available
	if s.jobLogger != nil {
		// Log job start
		s.jobLogger.LogJobStart(jobId, jobId, targetJob.Expression(), "manual", triggerBy)

		startTime := time.Now()

		c.App.Logger().Info("Executing cron job manually",
			"job_id", jobId,
			"trigger_by", triggerBy,
			"start_time", startTime)

		// Execute job with comprehensive output capture
		capturedOutput, errorMsg := ExecuteJobWithOutputCapture(jobId, jobId, func() {
			targetJob.Run()
		})

		// Log completion
		duration := time.Since(startTime)
		if errorMsg != "" {
			s.jobLogger.LogJobError(jobId, errorMsg)
			c.App.Logger().Error("Manual cron job execution failed",
				"job_id", jobId,
				"duration", duration,
				"error", errorMsg)

			return s.cronError(c, 500, "Job execution failed", map[string]string{
				"job_id": jobId,
				"error":  errorMsg,
			})
		} else {
			s.jobLogger.LogJobComplete(jobId, capturedOutput, "")
			c.App.Logger().Info("Manual cron job execution completed",
				"job_id", jobId,
				"duration", duration,
				"output_length", len(capturedOutput))
		}
	} else {
		// Fallback to regular execution without logging
		c.App.Logger().Info("Executing cron job on demand (no logging)", "job_id", jobId)
		go targetJob.Run()
	}

	return c.JSON(200, CronResponse{
		Message: fmt.Sprintf("Job '%s' executed successfully", jobId),
		Success: true,
		Data:    map[string]string{"job_id": jobId},
	})
}

// handleGetCronStatus returns the current system status
func (s *Server) handleGetCronStatus(c *core.RequestEvent) error {
	jobs := c.App.Cron().Jobs()
	isStarted := c.App.Cron().HasStarted()

	status := CronStatusStopped
	if isStarted {
		status = CronStatusRunning
	}

	statusData := CronSystemStatus{
		TotalJobs:  len(jobs),
		ActiveJobs: len(jobs), // All registered PocketBase jobs are active
		Status:     status,
		HasStarted: isStarted,
	}

	c.App.Logger().Debug("Retrieved cron system status", "status", status, "jobs", len(jobs))
	return c.JSON(200, statusData)
}

// handleUpdateTimezone updates the cron system timezone
func (s *Server) handleUpdateTimezone(c *core.RequestEvent) error {
	var timezoneData map[string]string
	if err := c.BindBody(&timezoneData); err != nil {
		return s.cronError(c, 400, "Invalid timezone data", map[string]string{"error": err.Error()})
	}

	timezoneStr, ok := timezoneData["timezone"]
	if !ok || strings.TrimSpace(timezoneStr) == "" {
		return s.cronError(c, 400, "Timezone is required", nil)
	}

	location, err := time.LoadLocation(timezoneStr)
	if err != nil {
		return s.cronError(c, 400, "Invalid timezone", map[string]string{"timezone": timezoneStr, "error": err.Error()})
	}

	c.App.Logger().Info("Updating cron timezone", "timezone", timezoneStr)
	c.App.Cron().SetTimezone(location)

	return c.JSON(200, CronResponse{
		Message: "Timezone updated successfully",
		Success: true,
		Data:    map[string]string{"timezone": timezoneStr},
	})
}

// handleDeleteCronJob deletes an existing cron job
func (s *Server) handleDeleteCronJob(c *core.RequestEvent) error {
	jobId := strings.TrimSpace(c.Request.PathValue("id"))
	if jobId == "" {
		return s.cronError(c, 400, "Job ID is required", nil)
	}

	// Verify job exists before deletion
	jobs := c.App.Cron().Jobs()
	found := false
	for _, job := range jobs {
		if job.Id() == jobId {
			found = true
			break
		}
	}

	if !found {
		return s.cronError(c, 404, "Job not found", map[string]string{"job_id": jobId})
	}

	c.App.Cron().Remove(jobId)
	c.App.Logger().Info("Deleted cron job", "job_id", jobId)

	return c.JSON(200, CronResponse{
		Message: "Cron job deleted successfully",
		Success: true,
		Data:    map[string]string{"job_id": jobId},
	})
}

// Helper functions

// cronError returns a standardized error response
func (s *Server) cronError(c *core.RequestEvent, status int, message string, data any) error {
	c.App.Logger().Error("Cron API error", "status", status, "message", message, "data", data)

	return c.JSON(status, CronResponse{
		Message: message,
		Success: false,
		Data:    data,
	})
}

// isSystemJob checks if a job ID belongs to PocketBase system jobs
func (s *Server) isSystemJob(jobID string) bool {
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

// handleGetJobLogs retrieves job execution logs with pagination
func (s *Server) handleGetJobLogs(c *core.RequestEvent) error {
	if s.jobLogger == nil {
		return s.cronError(c, 503, "Job logging system not initialized", nil)
	}

	// Parse pagination parameters
	page := 1
	if p := c.Request.URL.Query().Get("page"); p != "" {
		if parsed := parseIntWithDefault(p, 1); parsed > 0 {
			page = parsed
		}
	}

	perPage := 50
	if pp := c.Request.URL.Query().Get("per_page"); pp != "" {
		if parsed := parseIntWithDefault(pp, 50); parsed > 0 && parsed <= 1000 {
			perPage = parsed
		}
	}

	// Calculate offset
	offset := (page - 1) * perPage

	logs, totalCount, err := s.jobLogger.getRecentJobLogsWithPagination(perPage, offset)
	if err != nil {
		return s.cronError(c, 500, "Failed to retrieve job logs", map[string]string{"error": err.Error()})
	}

	// Calculate pagination metadata
	totalPages := (totalCount + int64(perPage) - 1) / int64(perPage)
	hasNext := int64(page) < totalPages
	hasPrev := page > 1

	return c.JSON(200, CronResponse{
		Message: "Job logs retrieved successfully",
		Success: true,
		Data: map[string]interface{}{
			"logs": logs,
			"pagination": map[string]interface{}{
				"page":        page,
				"per_page":    perPage,
				"total":       totalCount,
				"total_pages": totalPages,
				"has_next":    hasNext,
				"has_prev":    hasPrev,
			},
		},
	})
}

// handleGetJobLogsByJobID retrieves logs for a specific job with pagination
func (s *Server) handleGetJobLogsByJobID(c *core.RequestEvent) error {
	if s.jobLogger == nil {
		return s.cronError(c, 503, "Job logging system not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("job_id"))
	if jobID == "" {
		return s.cronError(c, 400, "Job ID is required", nil)
	}

	// Parse pagination parameters
	page := 1
	if p := c.Request.URL.Query().Get("page"); p != "" {
		if parsed := parseIntWithDefault(p, 1); parsed > 0 {
			page = parsed
		}
	}

	perPage := 100
	if pp := c.Request.URL.Query().Get("per_page"); pp != "" {
		if parsed := parseIntWithDefault(pp, 100); parsed > 0 && parsed <= 1000 {
			perPage = parsed
		}
	}

	// Calculate offset
	offset := (page - 1) * perPage

	logs, totalCount, err := s.jobLogger.getJobLogsByJobIDWithPagination(jobID, perPage, offset)
	if err != nil {
		return s.cronError(c, 500, "Failed to retrieve job logs", map[string]string{"error": err.Error()})
	}

	// Calculate pagination metadata
	totalPages := (totalCount + int64(perPage) - 1) / int64(perPage)
	hasNext := int64(page) < totalPages
	hasPrev := page > 1

	return c.JSON(200, CronResponse{
		Message: fmt.Sprintf("Job logs for '%s' retrieved successfully", jobID),
		Success: true,
		Data: map[string]interface{}{
			"logs": logs,
			"pagination": map[string]interface{}{
				"page":        page,
				"per_page":    perPage,
				"total":       totalCount,
				"total_pages": totalPages,
				"has_next":    hasNext,
				"has_prev":    hasPrev,
				"job_id":      jobID,
			},
		},
	})
}

// handleGetJobLogsAnalytics retrieves job logs analytics data
func (s *Server) handleGetJobLogsAnalytics(c *core.RequestEvent) error {
	if s.jobLogger == nil {
		return s.cronError(c, 503, "Job logging system not initialized", nil)
	}

	data, err := s.jobLogger.GetJobLogsData()
	if err != nil {
		return s.cronError(c, 500, "Failed to retrieve job logs analytics", map[string]string{"error": err.Error()})
	}

	return c.JSON(200, CronResponse{
		Message: "Job logs analytics retrieved successfully",
		Success: true,
		Data:    data,
	})
}
