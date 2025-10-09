package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
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
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Active     bool   `json:"active"`
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
}

// Handler functions with improved error handling and validation

// handleGetCronJobs retrieves all cron jobs
func (s *Server) handleGetCronJobs(c *core.RequestEvent) error {
	jobs := c.App.Cron().Jobs()
	jobsData := make([]CronJobData, len(jobs))

	for i, job := range jobs {
		jobsData[i] = CronJobData{
			ID:         job.Id(),
			Name:       job.Id(), // Use ID as name by default
			Expression: job.Expression(),
			Active:     true, // PocketBase jobs are always active when registered
		}
	}

	c.App.Logger().Debug("Retrieved cron jobs", "count", len(jobsData))
	return c.JSON(200, jobsData)
}

// handleRunCronJob executes a specific job immediately
func (s *Server) handleRunCronJob(c *core.RequestEvent) error {
	jobId := strings.TrimSpace(c.Request.PathValue("id"))
	if jobId == "" {
		return s.cronError(c, 400, "Job ID is required", nil)
	}

	jobs := c.App.Cron().Jobs()
	for _, job := range jobs {
		if job.Id() == jobId {
			c.App.Logger().Info("Executing cron job on demand", "job_id", jobId)
			go job.Run() // Run asynchronously

			return c.JSON(200, CronResponse{
				Message: fmt.Sprintf("Job '%s' executed successfully", jobId),
				Success: true,
				Data:    map[string]string{"job_id": jobId},
			})
		}
	}

	return s.cronError(c, 404, "Job not found", map[string]string{"job_id": jobId})
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
