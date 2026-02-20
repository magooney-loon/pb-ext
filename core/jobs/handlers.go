package jobs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Handlers holds HTTP handlers for the job/cron API.
type Handlers struct {
	manager *Manager
	logger  *Logger
}

// NewHandlers creates a Handlers instance from a Manager.
func NewHandlers(m *Manager) *Handlers {
	return &Handlers{
		manager: m,
		logger:  m.Logger(),
	}
}

// RegisterRoutes attaches all job API endpoints to the router.
func (h *Handlers) RegisterRoutes(e *core.ServeEvent) {
	// Job management
	e.Router.GET("/api/cron/jobs", h.handleGetJobs).Bind(apis.RequireSuperuserAuth())
	e.Router.POST("/api/cron/jobs/{id}/run", h.handleRunJob).Bind(apis.RequireSuperuserAuth())
	e.Router.DELETE("/api/cron/jobs/{id}", h.handleDeleteJob).Bind(apis.RequireSuperuserAuth())

	// System
	e.Router.GET("/api/cron/status", h.handleGetStatus).Bind(apis.RequireSuperuserAuth())
	e.Router.POST("/api/cron/config/timezone", h.handleUpdateTimezone).Bind(apis.RequireSuperuserAuth())

	// Logs (paginated endpoints via JobHandlers)
	e.Router.GET("/api/cron/logs", h.handleGetLogs).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/cron/logs/{job_id}", h.handleGetJobLogs).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/cron/logs/analytics", h.handleGetLogsAnalytics).Bind(apis.RequireSuperuserAuth())

	// Legacy job logs routes (kept for dashboard compatibility)
	e.Router.GET("/api/joblogs/analytics", h.handleGetLogsAnalytics).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/joblogs/recent", h.handleGetRecentLogs).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/joblogs/job/{job_id}", h.handleGetJobLogsLegacy).Bind(apis.RequireSuperuserAuth())
	e.Router.POST("/api/joblogs/flush", h.handleFlush).Bind(apis.RequireSuperuserAuth())
}

// --- job management handlers ---

func (h *Handlers) handleGetJobs(c *core.RequestEvent) error {
	if h.manager == nil {
		return h.errResp(c, 503, "Job manager not initialized", nil)
	}

	showSystem := c.Request.URL.Query().Get("show_system") != "false"
	activeOnly := c.Request.URL.Query().Get("active_only") == "true"

	jobs := h.manager.GetJobs(ListOptions{IncludeSystemJobs: showSystem, ActiveOnly: activeOnly})

	c.App.Logger().Debug("Retrieved jobs", "count", len(jobs), "show_system", showSystem, "active_only", activeOnly)
	return c.JSON(200, jobs)
}

func (h *Handlers) handleRunJob(c *core.RequestEvent) error {
	if h.manager == nil {
		return h.errResp(c, 503, "Job manager not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("id"))
	if jobID == "" {
		return h.errResp(c, 400, "Job ID is required", nil)
	}

	triggerBy := ""
	if c.Auth != nil {
		triggerBy = c.Auth.Id
	}

	result, err := h.manager.ExecuteJobManually(jobID, triggerBy)
	if err != nil {
		return h.errResp(c, 500, "Job execution failed", map[string]interface{}{
			"job_id": jobID,
			"error":  err.Error(),
		})
	}

	return h.okResp(c, fmt.Sprintf("Job '%s' executed successfully", jobID), result)
}

func (h *Handlers) handleDeleteJob(c *core.RequestEvent) error {
	if h.manager == nil {
		return h.errResp(c, 503, "Job manager not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("id"))
	if jobID == "" {
		return h.errResp(c, 400, "Job ID is required", nil)
	}

	if _, err := h.manager.GetJobMetadata(jobID); err != nil {
		return h.errResp(c, 404, "Job not found", map[string]string{"job_id": jobID})
	}

	if err := h.manager.RemoveJob(jobID); err != nil {
		return h.errResp(c, 500, "Failed to delete job", map[string]interface{}{
			"job_id": jobID,
			"error":  err.Error(),
		})
	}

	return h.okResp(c, "Job deleted successfully", map[string]string{"job_id": jobID})
}

// --- system handlers ---

func (h *Handlers) handleGetStatus(c *core.RequestEvent) error {
	if h.manager == nil {
		return h.errResp(c, 503, "Job manager not initialized", nil)
	}
	return c.JSON(200, h.manager.GetSystemStatus())
}

func (h *Handlers) handleUpdateTimezone(c *core.RequestEvent) error {
	if h.manager == nil {
		return h.errResp(c, 503, "Job manager not initialized", nil)
	}

	var body map[string]string
	if err := c.BindBody(&body); err != nil {
		return h.errResp(c, 400, "Invalid timezone data", map[string]string{"error": err.Error()})
	}

	tz, ok := body["timezone"]
	if !ok || strings.TrimSpace(tz) == "" {
		return h.errResp(c, 400, "Timezone is required", nil)
	}

	if err := h.manager.UpdateTimezone(tz); err != nil {
		return h.errResp(c, 400, "Invalid timezone", map[string]interface{}{
			"timezone": tz,
			"error":    err.Error(),
		})
	}

	return h.okResp(c, "Timezone updated successfully", map[string]string{"timezone": tz})
}

// --- log handlers ---

func (h *Handlers) handleGetLogs(c *core.RequestEvent) error {
	if h.logger == nil {
		return h.errResp(c, 503, "Job logging system not initialized", nil)
	}

	page, perPage := parsePagination(c, 1, 50)
	offset := (page - 1) * perPage

	logs, total, err := h.logger.GetRecentLogsWithPagination(perPage, offset)
	if err != nil {
		return h.errResp(c, 500, "Failed to retrieve job logs", map[string]string{"error": err.Error()})
	}

	return h.okResp(c, "Job logs retrieved successfully", map[string]interface{}{
		"logs":       logs,
		"pagination": buildPagination(page, perPage, total),
	})
}

func (h *Handlers) handleGetJobLogs(c *core.RequestEvent) error {
	if h.logger == nil {
		return h.errResp(c, 503, "Job logging system not initialized", nil)
	}

	jobID := strings.TrimSpace(c.Request.PathValue("job_id"))
	if jobID == "" {
		return h.errResp(c, 400, "Job ID is required", nil)
	}

	page, perPage := parsePagination(c, 1, 100)
	offset := (page - 1) * perPage

	logs, total, err := h.logger.GetLogsByJobIDWithPagination(jobID, perPage, offset)
	if err != nil {
		return h.errResp(c, 500, "Failed to retrieve job logs", map[string]string{"error": err.Error()})
	}

	return h.okResp(c, fmt.Sprintf("Job logs for '%s' retrieved successfully", jobID), map[string]interface{}{
		"logs":       logs,
		"pagination": buildPagination(page, perPage, total),
		"job_id":     jobID,
	})
}

func (h *Handlers) handleGetLogsAnalytics(c *core.RequestEvent) error {
	if h.logger == nil {
		return h.errResp(c, 503, "Job logging system not initialized", nil)
	}

	data, err := h.logger.GetLogsData()
	if err != nil {
		return h.errResp(c, 500, "Failed to retrieve job logs analytics", map[string]string{"error": err.Error()})
	}

	return h.okResp(c, "Job logs analytics retrieved successfully", data)
}

// --- legacy log handlers (dashboard compatibility) ---

func (h *Handlers) handleGetRecentLogs(c *core.RequestEvent) error {
	if h.logger == nil {
		return h.errResp(c, 503, "Job logging system not initialized", nil)
	}

	limit := 50
	if l := c.Request.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	logs, err := h.logger.GetRecentLogs(limit)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	return c.JSON(200, logs)
}

func (h *Handlers) handleGetJobLogsLegacy(c *core.RequestEvent) error {
	if h.logger == nil {
		return h.errResp(c, 503, "Job logging system not initialized", nil)
	}

	jobID := c.Request.PathValue("job_id")
	limit := 100
	if l := c.Request.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	logs, _, err := h.logger.GetLogsByJobIDWithPagination(jobID, limit, 0)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	return c.JSON(200, logs)
}

func (h *Handlers) handleFlush(c *core.RequestEvent) error {
	if h.logger == nil {
		return h.errResp(c, 503, "Job logging system not initialized", nil)
	}
	h.logger.ForceFlush()
	return c.JSON(200, map[string]string{"message": "Job logs flushed successfully"})
}

// --- helpers ---

func (h *Handlers) okResp(c *core.RequestEvent, message string, data interface{}) error {
	return c.JSON(200, APIResponse{Message: message, Success: true, Data: data})
}

func (h *Handlers) errResp(c *core.RequestEvent, status int, message string, data interface{}) error {
	c.App.Logger().Error("Job API error", "status", status, "message", message)
	return c.JSON(status, APIResponse{Message: message, Success: false, Data: data})
}

func parsePagination(c *core.RequestEvent, defaultPage, defaultPerPage int) (int, int) {
	page, perPage := defaultPage, defaultPerPage

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

func buildPagination(page, perPage int, total int64) PaginationData {
	totalPages := (total + int64(perPage) - 1) / int64(perPage)
	return PaginationData{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    int64(page) < totalPages,
		HasPrev:    page > 1,
	}
}
