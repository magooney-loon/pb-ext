package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Job logging constants
const (
	JobLogsLookbackDays  = 30
	MaxJobLogsRecords    = 10000
	JobLogsFlushWaitTime = 30 * time.Second
	JobLogsCollection    = "_job_logs"
)

// Job status constants
const (
	JobStatusStarted   = "started"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
	JobStatusTimeout   = "timeout"
)

// JobLog represents a single job execution log entry
type JobLog struct {
	RecordID    string            `json:"record_id,omitempty"` // Database record ID for updates
	JobID       string            `json:"job_id"`
	JobName     string            `json:"job_name"`
	Description string            `json:"description,omitempty"`
	Expression  string            `json:"expression"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time,omitempty"`
	Duration    *time.Duration    `json:"duration,omitempty"`
	Status      string            `json:"status"`
	Output      string            `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	TriggerType string            `json:"trigger_type"`         // "scheduled", "manual", "api"
	TriggerBy   string            `json:"trigger_by,omitempty"` // user ID for manual triggers
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// JobLogger manages job execution logging with buffering
type JobLogger struct {
	app           core.App
	buffer        []JobLog
	bufferMutex   sync.RWMutex
	flushInterval time.Duration
	batchSize     int
	lastFlushTime time.Time
	flushChan     chan struct{}
	flushTicker   *time.Ticker
	flushActive   sync.Mutex

	// Active job tracking
	activeJobs    map[string]*JobLog
	activeJobsMux sync.RWMutex
}

// JobLogsData represents aggregated job logs data for API responses
type JobLogsData struct {
	TotalExecutions     int64            `json:"total_executions"`
	SuccessfulRuns      int64            `json:"successful_runs"`
	FailedRuns          int64            `json:"failed_runs"`
	SuccessRate         float64          `json:"success_rate"`
	AverageRunTime      time.Duration    `json:"average_run_time"`
	RecentExecutions    []JobLogSummary  `json:"recent_executions"`
	JobStats            []JobStatSummary `json:"job_stats"`
	TodayExecutions     int64            `json:"today_executions"`
	YesterdayExecutions int64            `json:"yesterday_executions"`
	HourlyActivity      map[string]int64 `json:"hourly_activity"`
}

// JobLogSummary represents a simplified job log entry
type JobLogSummary struct {
	ID          string         `json:"id"`
	JobID       string         `json:"job_id"`
	JobName     string         `json:"job_name"`
	StartTime   time.Time      `json:"start_time"`
	Duration    *time.Duration `json:"duration,omitempty"`
	Status      string         `json:"status"`
	TriggerType string         `json:"trigger_type"`
	Output      string         `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// JobStatSummary represents statistics for a specific job
type JobStatSummary struct {
	JobID            string        `json:"job_id"`
	JobName          string        `json:"job_name"`
	TotalRuns        int64         `json:"total_runs"`
	SuccessfulRuns   int64         `json:"successful_runs"`
	FailedRuns       int64         `json:"failed_runs"`
	SuccessRate      float64       `json:"success_rate"`
	LastRun          *time.Time    `json:"last_run,omitempty"`
	AverageRunTime   time.Duration `json:"average_run_time"`
	NextScheduledRun *time.Time    `json:"next_scheduled_run,omitempty"`
}

// NewJobLogger creates a new JobLogger instance
func NewJobLogger(app core.App) *JobLogger {
	return &JobLogger{
		app:           app,
		buffer:        make([]JobLog, 0),
		flushInterval: JobLogsFlushWaitTime,
		batchSize:     100,
		lastFlushTime: time.Now(),
		flushChan:     make(chan struct{}, 1),
		activeJobs:    make(map[string]*JobLog),
	}
}

// InitializeJobLogger initializes the job logging system
func InitializeJobLogger(app core.App) (*JobLogger, error) {
	logger := NewJobLogger(app)

	// Setup collections
	if err := SetupJobLogsCollections(app); err != nil {
		return nil, fmt.Errorf("failed to setup job logs collections: %w", err)
	}

	// Start background workers
	go logger.backgroundFlushWorker()
	go logger.startFlushTimer()

	app.Logger().Info("Job logging system initialized")
	return logger, nil
}

// backgroundFlushWorker runs the background flush process
func (jl *JobLogger) backgroundFlushWorker() {
	for range jl.flushChan {
		jl.flushBuffer()
	}
}

// startFlushTimer starts the periodic flush timer
func (jl *JobLogger) startFlushTimer() {
	jl.flushTicker = time.NewTicker(jl.flushInterval)
	defer jl.flushTicker.Stop()

	for range jl.flushTicker.C {
		jl.bufferMutex.RLock()
		bufferLen := len(jl.buffer)
		jl.bufferMutex.RUnlock()

		if bufferLen > 0 {
			select {
			case jl.flushChan <- struct{}{}:
			default:
			}
		}
	}
}

// flushBuffer flushes buffered job logs to the database
func (jl *JobLogger) flushBuffer() {
	jl.flushActive.Lock()
	defer jl.flushActive.Unlock()

	jl.bufferMutex.Lock()
	if len(jl.buffer) == 0 {
		jl.bufferMutex.Unlock()
		return
	}

	toFlush := make([]JobLog, len(jl.buffer))
	copy(toFlush, jl.buffer)
	jl.buffer = jl.buffer[:0]
	jl.bufferMutex.Unlock()

	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		jl.app.Logger().Error("Failed to find job logs collection", "error", err)
		return
	}

	successCount := 0
	for _, jobLog := range toFlush {
		record := core.NewRecord(collection)
		jl.setJobLogRecordFields(record, jobLog)

		if err := jl.app.SaveNoValidate(record); err != nil {
			jl.app.Logger().Error("Failed to save job log", "error", err, "job_id", jobLog.JobID)
		} else {
			successCount++
		}
	}

	jl.lastFlushTime = time.Now()
	jl.app.Logger().Debug("Flushed job logs to database",
		"flushed", successCount,
		"failed", len(toFlush)-successCount,
		"total_attempted", len(toFlush),
	)

	// Cleanup old records
	jl.cleanupOldRecords()
}

// cleanupOldRecords removes old job log records
func (jl *JobLogger) cleanupOldRecords() {
	cutoffDate := time.Now().AddDate(0, 0, -JobLogsLookbackDays)

	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return
	}

	records, err := jl.app.FindRecordsByFilter(collection,
		"created < {:cutoff}",
		"-created",
		MaxJobLogsRecords, 0,
		dbx.Params{"cutoff": cutoffDate},
	)
	if err != nil {
		jl.app.Logger().Error("Failed to find old job log records for cleanup", "error", err)
		return
	}

	deletedCount := 0
	for _, record := range records {
		if err := jl.app.Delete(record); err != nil {
			jl.app.Logger().Error("Failed to delete old job log record", "error", err, "id", record.Id)
		} else {
			deletedCount++
		}
	}

	if deletedCount > 0 {
		jl.app.Logger().Debug("Cleaned up old job log records", "deleted", deletedCount)
	}
}

// SetupJobLogsCollections creates the job logs system collection
func SetupJobLogsCollections(app core.App) error {
	// Check if collection already exists
	_, err := app.FindCollectionByNameOrId(JobLogsCollection)
	if err == nil {
		app.Logger().Debug("Job logs collection already exists")
		return nil
	}

	// Create job logs collection
	jobLogsCollection := core.NewBaseCollection(JobLogsCollection)
	jobLogsCollection.System = true

	// Add fields
	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "job_id",
		Required: true,
		Max:      255,
	})

	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "job_name",
		Required: true,
		Max:      255,
	})

	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "description",
		Required: false,
		Max:      1000,
	})

	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "expression",
		Required: false,
		Max:      255,
	})

	jobLogsCollection.Fields.Add(&core.DateField{
		Name:     "start_time",
		Required: true,
	})

	jobLogsCollection.Fields.Add(&core.DateField{
		Name:     "end_time",
		Required: false,
	})

	jobLogsCollection.Fields.Add(&core.NumberField{
		Name:     "duration",
		Required: false,
	})

	jobLogsCollection.Fields.Add(&core.SelectField{
		Name:     "status",
		Required: true,
		Values:   []string{JobStatusStarted, JobStatusCompleted, JobStatusFailed, JobStatusTimeout},
	})

	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "output",
		Required: false,
		Max:      10000,
	})

	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "error",
		Required: false,
		Max:      2000,
	})

	jobLogsCollection.Fields.Add(&core.SelectField{
		Name:     "trigger_type",
		Required: true,
		Values:   []string{"scheduled", "manual", "api"},
	})

	jobLogsCollection.Fields.Add(&core.TextField{
		Name:     "trigger_by",
		Required: false,
		Max:      255,
	})

	jobLogsCollection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})
	jobLogsCollection.Fields.Add(&core.AutodateField{
		Name:     "updated",
		OnCreate: true,
		OnUpdate: true,
	})

	if err := app.SaveNoValidate(jobLogsCollection); err != nil {
		return fmt.Errorf("failed to create job logs collection: %w", err)
	}

	app.Logger().Info("Created job logs collection", "name", JobLogsCollection)
	return nil
}

// RegisterRoutes registers job logs API endpoints
func (jl *JobLogger) RegisterRoutes(e *core.ServeEvent) {
	// Get job logs data
	e.Router.GET("/api/joblogs/analytics", func(c *core.RequestEvent) error {
		data, err := jl.GetJobLogsData()
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, data)
	}).Bind(apis.RequireSuperuserAuth())

	// Get recent job executions
	e.Router.GET("/api/joblogs/recent", func(c *core.RequestEvent) error {
		limit := 50
		if l := c.Request.URL.Query().Get("limit"); l != "" {
			if parsed := parseIntWithDefault(l, limit); parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		logs, err := jl.getRecentJobLogs(limit)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, logs)
	}).Bind(apis.RequireSuperuserAuth())

	// Get logs for a specific job
	e.Router.GET("/api/joblogs/job/{job_id}", func(c *core.RequestEvent) error {
		jobID := c.Request.PathValue("job_id")
		limit := 100
		if l := c.Request.URL.Query().Get("limit"); l != "" {
			if parsed := parseIntWithDefault(l, limit); parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		logs, err := jl.getJobLogsByJobID(jobID, limit)
		if err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, logs)
	}).Bind(apis.RequireSuperuserAuth())

	// Force flush logs
	e.Router.POST("/api/joblogs/flush", func(c *core.RequestEvent) error {
		jl.ForceFlush()
		return c.JSON(200, map[string]string{"message": "Job logs flushed successfully"})
	}).Bind(apis.RequireSuperuserAuth())
}

// LogJobStart logs the start of a job execution
func (jl *JobLogger) LogJobStart(jobID, jobName, expression, triggerType, triggerBy string) {
	jl.LogJobStartWithDescription(jobID, jobName, "", expression, triggerType, triggerBy)
}

// LogJobStartWithDescription logs the start of a job execution with description
func (jl *JobLogger) LogJobStartWithDescription(jobID, jobName, description, expression, triggerType, triggerBy string) {
	jl.activeJobsMux.Lock()
	defer jl.activeJobsMux.Unlock()

	jobLog := &JobLog{
		JobID:       jobID,
		JobName:     jobName,
		Description: description,
		Expression:  expression,
		StartTime:   time.Now(),
		Status:      JobStatusStarted,
		TriggerType: triggerType,
		TriggerBy:   triggerBy,
		Metadata:    make(map[string]string),
	}

	// Save immediately to database to get record ID
	if recordID := jl.saveJobLogRecord(*jobLog); recordID != "" {
		jobLog.RecordID = recordID
	}

	jl.activeJobs[jobID] = jobLog
}

// LogJobComplete logs the completion of a job execution
func (jl *JobLogger) LogJobComplete(jobID, output, errorMsg string) {
	jl.activeJobsMux.Lock()
	defer jl.activeJobsMux.Unlock()

	jobLog, exists := jl.activeJobs[jobID]
	if !exists {
		return
	}

	now := time.Now()
	duration := now.Sub(jobLog.StartTime)

	jobLog.EndTime = &now
	jobLog.Duration = &duration
	jobLog.Output = output
	jobLog.Error = errorMsg

	if errorMsg != "" {
		jobLog.Status = JobStatusFailed
	} else {
		jobLog.Status = JobStatusCompleted
	}

	// Update existing record instead of creating new one
	if jobLog.RecordID != "" {
		jl.updateJobLogRecord(*jobLog)
	} else {
		// Fallback: save as new record if no record ID
		jl.addToBuffer(*jobLog)
	}

	delete(jl.activeJobs, jobID)
}

// LogJobError logs a job execution error
func (jl *JobLogger) LogJobError(jobID, errorMsg string) {
	jl.LogJobComplete(jobID, "", errorMsg)
}

// saveJobLogRecord saves a job log record immediately to database and returns record ID
func (jl *JobLogger) saveJobLogRecord(jobLog JobLog) string {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		jl.app.Logger().Error("Failed to find job logs collection", "error", err)
		return ""
	}

	record := core.NewRecord(collection)
	jl.setJobLogRecordFields(record, jobLog)

	if err := jl.app.SaveNoValidate(record); err != nil {
		jl.app.Logger().Error("Failed to save job log record", "error", err, "job_id", jobLog.JobID)
		return ""
	}

	return record.Id
}

// updateJobLogRecord updates an existing job log record
func (jl *JobLogger) updateJobLogRecord(jobLog JobLog) {
	if jobLog.RecordID == "" {
		return
	}

	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		jl.app.Logger().Error("Failed to find job logs collection", "error", err)
		return
	}

	record, err := jl.app.FindRecordById(collection, jobLog.RecordID)
	if err != nil {
		jl.app.Logger().Error("Failed to find job log record", "error", err, "record_id", jobLog.RecordID)
		return
	}

	jl.setJobLogRecordFields(record, jobLog)

	if err := jl.app.SaveNoValidate(record); err != nil {
		jl.app.Logger().Error("Failed to update job log record", "error", err, "job_id", jobLog.JobID)
	}
}

// setJobLogRecordFields sets the fields on a record from a JobLog
func (jl *JobLogger) setJobLogRecordFields(record *core.Record, jobLog JobLog) {
	record.Set("job_id", jobLog.JobID)
	record.Set("job_name", jobLog.JobName)
	record.Set("description", jobLog.Description)
	record.Set("expression", jobLog.Expression)
	record.Set("start_time", jobLog.StartTime)

	if jobLog.EndTime != nil {
		record.Set("end_time", *jobLog.EndTime)
	}

	if jobLog.Duration != nil {
		record.Set("duration", int64(*jobLog.Duration/time.Millisecond))
	}

	record.Set("status", jobLog.Status)
	record.Set("output", jobLog.Output)
	record.Set("error", jobLog.Error)
	record.Set("trigger_type", jobLog.TriggerType)
	record.Set("trigger_by", jobLog.TriggerBy)
}

// addToBuffer adds a job log to the buffer (fallback method)
func (jl *JobLogger) addToBuffer(jobLog JobLog) {
	jl.bufferMutex.Lock()
	defer jl.bufferMutex.Unlock()

	jl.buffer = append(jl.buffer, jobLog)

	if len(jl.buffer) >= jl.batchSize {
		select {
		case jl.flushChan <- struct{}{}:
		default:
		}
	}
}

// ForceFlush forces immediate flush of buffered logs
func (jl *JobLogger) ForceFlush() {
	select {
	case jl.flushChan <- struct{}{}:
	default:
	}
}

// GetJobLogsData returns aggregated job logs data
func (jl *JobLogger) GetJobLogsData() (*JobLogsData, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to find job logs collection: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	// Get total executions
	var totalRecords int64
	err = jl.app.DB().Select("COUNT(*)").
		From(collection.Name).
		Row(&totalRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to count total executions: %w", err)
	}

	// Get successful runs
	var successfulRecords int64
	err = jl.app.DB().Select("COUNT(*)").
		From(collection.Name).
		Where(dbx.HashExp{"status": "completed"}).
		Row(&successfulRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to count successful runs: %w", err)
	}

	// Get failed runs
	var failedRecords int64
	err = jl.app.DB().Select("COUNT(*)").
		From(collection.Name).
		Where(dbx.HashExp{"status": "failed"}).
		Row(&failedRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed runs: %w", err)
	}

	// Calculate success rate
	successRate := 0.0
	if totalRecords > 0 {
		successRate = float64(successfulRecords) / float64(totalRecords) * 100
	}

	// Get today's executions
	var todayRecords int64
	err = jl.app.DB().Select("COUNT(*)").
		From(collection.Name).
		Where(dbx.NewExp("created >= {:today}", dbx.Params{"today": today})).
		Row(&todayRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to count today's executions: %w", err)
	}

	// Get yesterday's executions
	var yesterdayRecords int64
	err = jl.app.DB().Select("COUNT(*)").
		From(collection.Name).
		Where(dbx.NewExp("created >= {:yesterday} AND created < {:today}", dbx.Params{"yesterday": yesterday, "today": today})).
		Row(&yesterdayRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to count yesterday's executions: %w", err)
	}

	// Get recent executions
	recentLogs, err := jl.getRecentJobLogs(10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent executions: %w", err)
	}

	// Get job statistics
	jobStats, err := jl.getJobStatistics()
	if err != nil {
		return nil, fmt.Errorf("failed to get job statistics: %w", err)
	}

	// Get average run time
	avgRunTime, err := jl.getAverageRunTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get average run time: %w", err)
	}

	return &JobLogsData{
		TotalExecutions:     totalRecords,
		SuccessfulRuns:      successfulRecords,
		FailedRuns:          failedRecords,
		SuccessRate:         successRate,
		AverageRunTime:      avgRunTime,
		RecentExecutions:    recentLogs,
		JobStats:            jobStats,
		TodayExecutions:     todayRecords,
		YesterdayExecutions: yesterdayRecords,
		HourlyActivity:      make(map[string]int64),
	}, nil
}

// getRecentJobLogs retrieves recent job execution logs
func (jl *JobLogger) getRecentJobLogs(limit int) ([]JobLogSummary, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return nil, err
	}

	records, err := jl.app.FindRecordsByFilter(collection, "", "-created", limit, 0)
	if err != nil {
		return nil, err
	}

	logs := make([]JobLogSummary, len(records))
	for i, record := range records {
		var duration *time.Duration
		if durationMs := record.GetInt("duration"); durationMs > 0 {
			d := time.Duration(durationMs) * time.Millisecond
			duration = &d
		}

		logs[i] = JobLogSummary{
			ID:          record.Id,
			JobID:       record.GetString("job_id"),
			JobName:     record.GetString("job_name"),
			StartTime:   record.GetDateTime("start_time").Time(),
			Duration:    duration,
			Status:      record.GetString("status"),
			TriggerType: record.GetString("trigger_type"),
			Output:      record.GetString("output"),
			Error:       record.GetString("error"),
		}
	}

	return logs, nil
}

// getRecentJobLogsWithPagination retrieves recent job execution logs with pagination
func (jl *JobLogger) getRecentJobLogsWithPagination(limit, offset int) ([]JobLogSummary, int64, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return nil, 0, err
	}

	// Get total count
	var totalCount int64
	err = jl.app.DB().Select("COUNT(*)").From(collection.Name).Row(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated records
	records, err := jl.app.FindRecordsByFilter(collection, "", "-created", limit, offset)
	if err != nil {
		return nil, 0, err
	}

	logs := make([]JobLogSummary, len(records))
	for i, record := range records {
		var duration *time.Duration
		if durationMs := record.GetInt("duration"); durationMs > 0 {
			d := time.Duration(durationMs) * time.Millisecond
			duration = &d
		}

		logs[i] = JobLogSummary{
			ID:          record.Id,
			JobID:       record.GetString("job_id"),
			JobName:     record.GetString("job_name"),
			StartTime:   record.GetDateTime("start_time").Time(),
			Duration:    duration,
			Status:      record.GetString("status"),
			TriggerType: record.GetString("trigger_type"),
			Output:      record.GetString("output"),
			Error:       record.GetString("error"),
		}
	}

	return logs, totalCount, nil
}

// getJobLogsByJobID retrieves logs for a specific job
func (jl *JobLogger) getJobLogsByJobID(jobID string, limit int) ([]JobLogSummary, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return nil, err
	}

	records, err := jl.app.FindRecordsByFilter(collection,
		"job_id = {:job_id}",
		"-created",
		limit, 0,
		dbx.Params{"job_id": jobID},
	)
	if err != nil {
		return nil, err
	}

	logs := make([]JobLogSummary, len(records))
	for i, record := range records {
		var duration *time.Duration
		if durationMs := record.GetInt("duration"); durationMs > 0 {
			d := time.Duration(durationMs) * time.Millisecond
			duration = &d
		}

		logs[i] = JobLogSummary{
			ID:          record.Id,
			JobID:       record.GetString("job_id"),
			JobName:     record.GetString("job_name"),
			StartTime:   record.GetDateTime("start_time").Time(),
			Duration:    duration,
			Status:      record.GetString("status"),
			TriggerType: record.GetString("trigger_type"),
			Output:      record.GetString("output"),
			Error:       record.GetString("error"),
		}
	}

	return logs, nil
}

// getJobLogsByJobIDWithPagination retrieves logs for a specific job with pagination
func (jl *JobLogger) getJobLogsByJobIDWithPagination(jobID string, limit, offset int) ([]JobLogSummary, int64, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return nil, 0, err
	}

	// Get total count for this job
	var totalCount int64
	err = jl.app.DB().Select("COUNT(*)").
		From(collection.Name).
		Where(dbx.NewExp("job_id = {:job_id}", dbx.Params{"job_id": jobID})).
		Row(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated records
	records, err := jl.app.FindRecordsByFilter(collection,
		"job_id = {:job_id}",
		"-created",
		limit, offset,
		dbx.Params{"job_id": jobID},
	)
	if err != nil {
		return nil, 0, err
	}

	logs := make([]JobLogSummary, len(records))
	for i, record := range records {
		var duration *time.Duration
		if durationMs := record.GetInt("duration"); durationMs > 0 {
			d := time.Duration(durationMs) * time.Millisecond
			duration = &d
		}

		logs[i] = JobLogSummary{
			ID:          record.Id,
			JobID:       record.GetString("job_id"),
			JobName:     record.GetString("job_name"),
			StartTime:   record.GetDateTime("start_time").Time(),
			Duration:    duration,
			Status:      record.GetString("status"),
			TriggerType: record.GetString("trigger_type"),
			Output:      record.GetString("output"),
			Error:       record.GetString("error"),
		}
	}

	return logs, totalCount, nil
}

// getJobStatistics retrieves statistics for all jobs
func (jl *JobLogger) getJobStatistics() ([]JobStatSummary, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return nil, err
	}

	// Get all records
	records, err := jl.app.FindRecordsByFilter(collection, "", "job_id", 0, 0)
	if err != nil {
		return nil, err
	}

	jobMap := make(map[string]*JobStatSummary)

	for _, record := range records {
		jobID := record.GetString("job_id")
		jobName := record.GetString("job_name")

		if _, exists := jobMap[jobID]; !exists {
			jobMap[jobID] = &JobStatSummary{
				JobID:   jobID,
				JobName: jobName,
			}
		}

		stat := jobMap[jobID]
		stat.TotalRuns++

		if record.GetString("status") == JobStatusCompleted {
			stat.SuccessfulRuns++
		} else if record.GetString("status") == JobStatusFailed {
			stat.FailedRuns++
		}

		// Update last run time
		startTime := record.GetDateTime("start_time").Time()
		if stat.LastRun == nil || startTime.After(*stat.LastRun) {
			stat.LastRun = &startTime
		}

		// Calculate average run time
		if durationMs := record.GetInt("duration"); durationMs > 0 {
			duration := time.Duration(durationMs) * time.Millisecond
			stat.AverageRunTime = (stat.AverageRunTime*time.Duration(stat.TotalRuns-1) + duration) / time.Duration(stat.TotalRuns)
		}
	}

	// Calculate success rates
	stats := make([]JobStatSummary, 0, len(jobMap))
	for _, stat := range jobMap {
		if stat.TotalRuns > 0 {
			stat.SuccessRate = float64(stat.SuccessfulRuns) / float64(stat.TotalRuns) * 100
		}
		stats = append(stats, *stat)
	}

	return stats, nil
}

// getAverageRunTime calculates the average run time across all jobs
func (jl *JobLogger) getAverageRunTime() (time.Duration, error) {
	collection, err := jl.app.FindCollectionByNameOrId(JobLogsCollection)
	if err != nil {
		return 0, err
	}

	records, err := jl.app.FindRecordsByFilter(collection,
		"duration > 0 AND status = 'completed'",
		"", 0, 0,
	)
	if err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, nil
	}

	var totalDuration time.Duration
	for _, record := range records {
		if durationMs := record.GetInt("duration"); durationMs > 0 {
			totalDuration += time.Duration(durationMs) * time.Millisecond
		}
	}

	return totalDuration / time.Duration(len(records)), nil
}

// Helper function to parse int with default value
func parseIntWithDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}

	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultVal
	}

	return result
}
