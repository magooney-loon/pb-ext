package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// JobAPIResponse represents a standard API response for job operations
type JobAPIResponse struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// PaginationData represents pagination metadata
type PaginationData struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// JobHandlers contains HTTP handlers for job/cron API endpoints
type JobHandlers struct {
	jobManager *JobManager
	jobLogger  *JobLogger
}

// NewJobHandlers creates a new job handlers instance
func NewJobHandlers(jobManager *JobManager, jobLogger *JobLogger) *JobHandlers {
	return &JobHandlers{
		jobManager: jobManager,
		jobLogger:  jobLogger,
	}
}

// RegisterJobRoutes registers all job/cron API endpoints
func (jh *JobHandlers) RegisterJobRoutes(e *core.ServeEvent) {
	// Job management endpoints
	e.Router.GET("/api/cron/jobs", jh.handleGetJobs).Bind(apis.RequireSuperuserAuth())
	e.Router.POST("/api/cron/jobs/{id}/run", jh.handleRunJob).Bind(apis.RequireSuperuserAuth())
	e.Router.DELETE("/api/cron/jobs/{id}", jh.handleDeleteJob).Bind(apis.RequireSuperuserAuth())

	// System endpoints
	e.Router.GET("/api/cron/status", jh.handleGetStatus).Bind(apis.RequireSuperuserAuth())
	e.Router.POST("/api/cron/config/timezone", jh.handleUpdateTimezone).Bind(apis.RequireSuperuserAuth())

	// Logging endpoints
	e.Router.GET("/api/cron/logs", jh.handleGetLogs).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/cron/logs/{job_id}", jh.handleGetJobLogs).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/cron/logs/analytics", jh.handleGetLogsAnalytics).Bind(apis.RequireSuperuserAuth())
}

// Job Management Handlers

// handleGetJobs retrieves all jobs with optional filtering
func (jh *JobHandlers) handleGetJobs(c *core.RequestEvent) error {
	if jh.jobManager == nil {
		return jh.errorResponse(c, 503, "Job manager not initialized", nil)
	}

	// Parse query parameters
	showSystemJobs := c.Request.URL.Query().Get("show_system") != "false"
	activeOnly := c.Request.URL.Query().Get("active_only") == "true"

	// Get jobs using the manager
	options := JobListOptions{
		IncludeSystemJobs: showSystemJobs,
		ActiveOnly:        activeOnly,
	}

	jobs := jh.jobManager.GetJobs(options)

	c.App.Logger().Debug("Retrieved jobs",
		"count", len(jobs),
		"show_system", showSystemJobs,
		"active_only", activeOnly)

	return c.JSON(200, jobs)
}

// handleRunJob executes a specific job manually
func (jh *JobHandlers) handleRunJob(c *core.RequestEvent) error {
	if jh.jobManager == nil {
		return jh.errorResponse(c, 503, "Job manager not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("id"))
	if jobID == "" {
		return jh.errorResponse(c, 400, "Job ID is required", nil)
	}

	// Get the trigger user ID
	triggerBy := ""
	if c.Auth != nil {
		triggerBy = c.Auth.Id
	}

	// Execute the job
	result, err := jh.jobManager.ExecuteJobManually(jobID, triggerBy)
	if err != nil {
		return jh.errorResponse(c, 500, "Job execution failed", map[string]interface{}{
			"job_id": jobID,
			"error":  err.Error(),
		})
	}

	return jh.successResponse(c, fmt.Sprintf("Job '%s' executed successfully", jobID), result)
}

// handleDeleteJob removes a job from the system
func (jh *JobHandlers) handleDeleteJob(c *core.RequestEvent) error {
	if jh.jobManager == nil {
		return jh.errorResponse(c, 503, "Job manager not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("id"))
	if jobID == "" {
		return jh.errorResponse(c, 400, "Job ID is required", nil)
	}

	// Verify job exists before deletion
	_, err := jh.jobManager.GetJobMetadata(jobID)
	if err != nil {
		return jh.errorResponse(c, 404, "Job not found", map[string]string{"job_id": jobID})
	}

	// Remove the job
	if err := jh.jobManager.RemoveJob(jobID); err != nil {
		return jh.errorResponse(c, 500, "Failed to delete job", map[string]interface{}{
			"job_id": jobID,
			"error":  err.Error(),
		})
	}

	return jh.successResponse(c, "Job deleted successfully", map[string]string{"job_id": jobID})
}

// System Handlers

// handleGetStatus returns the current cron system status
func (jh *JobHandlers) handleGetStatus(c *core.RequestEvent) error {
	if jh.jobManager == nil {
		return jh.errorResponse(c, 503, "Job manager not initialized", nil)
	}

	status := jh.jobManager.GetSystemStatus()
	return c.JSON(200, status)
}

// handleUpdateTimezone updates the cron system timezone
func (jh *JobHandlers) handleUpdateTimezone(c *core.RequestEvent) error {
	if jh.jobManager == nil {
		return jh.errorResponse(c, 503, "Job manager not initialized", nil)
	}

	var timezoneData map[string]string
	if err := c.BindBody(&timezoneData); err != nil {
		return jh.errorResponse(c, 400, "Invalid timezone data", map[string]string{"error": err.Error()})
	}

	timezoneStr, ok := timezoneData["timezone"]
	if !ok || strings.TrimSpace(timezoneStr) == "" {
		return jh.errorResponse(c, 400, "Timezone is required", nil)
	}

	if err := jh.jobManager.UpdateTimezone(timezoneStr); err != nil {
		return jh.errorResponse(c, 400, "Invalid timezone", map[string]interface{}{
			"timezone": timezoneStr,
			"error":    err.Error(),
		})
	}

	return jh.successResponse(c, "Timezone updated successfully", map[string]string{"timezone": timezoneStr})
}

// Logging Handlers

// handleGetLogs retrieves job execution logs with pagination
func (jh *JobHandlers) handleGetLogs(c *core.RequestEvent) error {
	if jh.jobLogger == nil {
		return jh.errorResponse(c, 503, "Job logging system not initialized", nil)
	}

	// Parse pagination parameters
	page, perPage := jh.parsePaginationParams(c, 1, 50)
	offset := (page - 1) * perPage

	// Get logs with pagination
	logs, totalCount, err := jh.jobLogger.getRecentJobLogsWithPagination(perPage, offset)
	if err != nil {
		return jh.errorResponse(c, 500, "Failed to retrieve job logs", map[string]string{"error": err.Error()})
	}

	// Create pagination metadata
	pagination := jh.createPaginationData(page, perPage, totalCount)

	return jh.successResponse(c, "Job logs retrieved successfully", map[string]interface{}{
		"logs":       logs,
		"pagination": pagination,
	})
}

// handleGetJobLogs retrieves logs for a specific job with pagination
func (jh *JobHandlers) handleGetJobLogs(c *core.RequestEvent) error {
	if jh.jobLogger == nil {
		return jh.errorResponse(c, 503, "Job logging system not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("job_id"))
	if jobID == "" {
		return jh.errorResponse(c, 400, "Job ID is required", nil)
	}

	// Parse pagination parameters
	page, perPage := jh.parsePaginationParams(c, 1, 100)
	offset := (page - 1) * perPage

	// Get job-specific logs with pagination
	logs, totalCount, err := jh.jobLogger.getJobLogsByJobIDWithPagination(jobID, perPage, offset)
	if err != nil {
		return jh.errorResponse(c, 500, "Failed to retrieve job logs", map[string]string{"error": err.Error()})
	}

	// Create pagination metadata
	pagination := jh.createPaginationData(page, perPage, totalCount)

	return jh.successResponse(c, fmt.Sprintf("Job logs for '%s' retrieved successfully", jobID), map[string]interface{}{
		"logs":       logs,
		"pagination": pagination,
		"job_id":     jobID,
	})
}

// handleGetLogsAnalytics retrieves job logs analytics data
func (jh *JobHandlers) handleGetLogsAnalytics(c *core.RequestEvent) error {
	if jh.jobLogger == nil {
		return jh.errorResponse(c, 503, "Job logging system not initialized", nil)
	}

	data, err := jh.jobLogger.GetJobLogsData()
	if err != nil {
		return jh.errorResponse(c, 500, "Failed to retrieve job logs analytics", map[string]string{"error": err.Error()})
	}

	return jh.successResponse(c, "Job logs analytics retrieved successfully", data)
}

// Helper methods

// parsePaginationParams parses page and per_page parameters from request
func (jh *JobHandlers) parsePaginationParams(c *core.RequestEvent, defaultPage, defaultPerPage int) (int, int) {
	page := defaultPage
	perPage := defaultPerPage

	if p := c.Request.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if pp := c.Request.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 1000 {
			perPage = parsed
		}
	}

	return page, perPage
}

// createPaginationData creates pagination metadata
func (jh *JobHandlers) createPaginationData(page, perPage int, totalCount int64) PaginationData {
	totalPages := (totalCount + int64(perPage) - 1) / int64(perPage)
	hasNext := int64(page) < totalPages
	hasPrev := page > 1

	return PaginationData{
		Page:       page,
		PerPage:    perPage,
		Total:      totalCount,
		TotalPages: totalPages,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}
}

// successResponse returns a standardized success response
func (jh *JobHandlers) successResponse(c *core.RequestEvent, message string, data interface{}) error {
	return c.JSON(200, JobAPIResponse{
		Message: message,
		Success: true,
		Data:    data,
	})
}

// errorResponse returns a standardized error response
func (jh *JobHandlers) errorResponse(c *core.RequestEvent, status int, message string, data interface{}) error {
	c.App.Logger().Error("Job API error",
		"status", status,
		"message", message,
		"data", data,
	)

	return c.JSON(status, JobAPIResponse{
		Message: message,
		Success: false,
		Data:    data,
	})
}
