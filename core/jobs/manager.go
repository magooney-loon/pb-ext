package jobs

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/cron"
)

// Manager orchestrates cron job registration, execution, and metadata tracking.
type Manager struct {
	app      core.App
	logger   *Logger
	registry map[string]*JobMetadata
	mu       sync.RWMutex
}

// NewManager creates a Manager. Use Initialize for normal startup.
func NewManager(app core.App, logger *Logger) *Manager {
	return &Manager{
		app:      app,
		logger:   logger,
		registry: make(map[string]*JobMetadata),
	}
}

// Initialize sets up the job logs collection, creates a Logger, and returns a Manager.
func Initialize(app core.App) (*Manager, error) {
	logger, err := InitializeLogger(app)
	if err != nil {
		return nil, err
	}
	m := NewManager(app, logger)

	// Set global singleton
	globalManager = m

	return m, nil
}

// RegisterJob registers a new cron job with automatic logging.
func (m *Manager) RegisterJob(jobID, jobName, description, expression string, fn func(*ExecutionLogger)) error {
	if jobName == "" {
		jobName = jobID
	}

	meta := &JobMetadata{
		ID:          jobID,
		Name:        jobName,
		Description: description,
		Expression:  expression,
		IsSystemJob: isSystemJob(jobID),
		CreatedAt:   time.Now(),
		IsActive:    true,
		Function:    fn,
	}

	m.mu.Lock()
	m.registry[jobID] = meta
	m.mu.Unlock()

	wrapped := m.wrap(jobID, jobName, description, expression, fn)

	if err := m.app.Cron().Add(jobID, expression, wrapped); err != nil {
		m.mu.Lock()
		delete(m.registry, jobID)
		m.mu.Unlock()
		return fmt.Errorf("failed to register job %s: %w", jobID, err)
	}

	m.app.Logger().Info("Registered cron job",
		"job_id", jobID,
		"job_name", jobName,
		"description", description,
		"expression", expression,
		"is_system", meta.IsSystemJob,
	)
	return nil
}

// ExecuteJobManually runs a registered job immediately, outside its schedule.
func (m *Manager) ExecuteJobManually(jobID, triggerBy string) (*ExecutionResult, error) {
	jobs := m.app.Cron().Jobs()
	var target *cron.Job
	for _, j := range jobs {
		if j.Id() == jobID {
			target = j
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	m.mu.RLock()
	meta, hasMeta := m.registry[jobID]
	m.mu.RUnlock()

	jobName := jobID
	if hasMeta && meta.Name != "" {
		jobName = meta.Name
	}

	result := &ExecutionResult{
		JobID:       jobID,
		TriggerType: "manual",
		TriggerBy:   triggerBy,
		ExecutedAt:  time.Now(),
	}

	startTime := time.Now()
	m.app.Logger().Info("Starting manual job execution", "job_id", jobID, "job_name", jobName, "trigger_by", triggerBy)

	jobDesc, jobExpr := "", ""
	if hasMeta {
		jobDesc = meta.Description
		jobExpr = meta.Expression
	}

	if m.logger != nil {
		m.logger.LogJobStartWithDescription(jobID, jobName, jobDesc, jobExpr, "manual", triggerBy)
	}

	factory := newLoggerFactory(m.logger)
	execLogger := factory.create(jobID)

	var errorMsg string
	func() {
		defer func() {
			if r := recover(); r != nil {
				errorMsg = fmt.Sprintf("Job panic: %v", r)
				execLogger.Fail(fmt.Errorf("%s", errorMsg))
			}
		}()

		if hasMeta && meta.Function != nil {
			meta.Function(execLogger)
		} else {
			target.Run()
		}
	}()

	capturedOutput := execLogger.GetOutput()
	result.Duration = time.Since(startTime)
	result.Output = capturedOutput
	result.Error = errorMsg
	result.Success = errorMsg == ""

	if m.logger != nil {
		if result.Success {
			m.logger.LogJobComplete(jobID, capturedOutput, "")
		} else {
			m.logger.LogJobError(jobID, errorMsg)
		}
	}

	if result.Success {
		m.app.Logger().Info("Manual job execution completed",
			"job_id", jobID, "job_name", jobName, "duration", result.Duration)
	} else {
		m.app.Logger().Error("Manual job execution failed",
			"job_id", jobID, "job_name", jobName, "duration", result.Duration, "error", errorMsg)
		return result, fmt.Errorf("job execution failed: %s", errorMsg)
	}

	return result, nil
}

// GetJobs returns a filtered list of registered jobs.
func (m *Manager) GetJobs(opts ListOptions) []JobMetadata {
	cronJobs := m.app.Cron().Jobs()
	result := make([]JobMetadata, 0, len(cronJobs))

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, j := range cronJobs {
		id := j.Id()
		if meta, exists := m.registry[id]; exists {
			if !opts.IncludeSystemJobs && meta.IsSystemJob {
				continue
			}
			if opts.ActiveOnly && !meta.IsActive {
				continue
			}
			result = append(result, *meta)
		} else {
			sys := isSystemJob(id)
			if !opts.IncludeSystemJobs && sys {
				continue
			}
			result = append(result, JobMetadata{
				ID:          id,
				Name:        id,
				Expression:  j.Expression(),
				IsSystemJob: sys,
				CreatedAt:   time.Now(),
				IsActive:    true,
			})
		}
	}
	return result
}

// GetJobMetadata returns metadata for a specific job by ID.
func (m *Manager) GetJobMetadata(jobID string) (*JobMetadata, error) {
	m.mu.RLock()
	if meta, exists := m.registry[jobID]; exists {
		copy := *meta
		m.mu.RUnlock()
		return &copy, nil
	}
	m.mu.RUnlock()

	for _, j := range m.app.Cron().Jobs() {
		if j.Id() == jobID {
			return &JobMetadata{
				ID:          jobID,
				Name:        jobID,
				Expression:  j.Expression(),
				IsSystemJob: isSystemJob(jobID),
				CreatedAt:   time.Now(),
				IsActive:    true,
			}, nil
		}
	}
	return nil, fmt.Errorf("job not found: %s", jobID)
}

// RemoveJob removes a job from the cron scheduler and the registry.
func (m *Manager) RemoveJob(jobID string) error {
	m.app.Cron().Remove(jobID)
	m.mu.Lock()
	delete(m.registry, jobID)
	m.mu.Unlock()
	m.app.Logger().Info("Removed job", "job_id", jobID)
	return nil
}

// GetSystemStatus returns a status summary of the cron scheduler.
func (m *Manager) GetSystemStatus() map[string]interface{} {
	cronJobs := m.app.Cron().Jobs()
	started := m.app.Cron().HasStarted()

	status := "stopped"
	if started {
		status = "running"
	}

	systemCount, userCount := 0, 0
	for _, j := range cronJobs {
		if isSystemJob(j.Id()) {
			systemCount++
		} else {
			userCount++
		}
	}

	return map[string]interface{}{
		"total_jobs":   len(cronJobs),
		"system_jobs":  systemCount,
		"user_jobs":    userCount,
		"active_jobs":  len(cronJobs),
		"status":       status,
		"has_started":  started,
		"last_updated": time.Now(),
	}
}

// UpdateTimezone updates the cron scheduler timezone.
func (m *Manager) UpdateTimezone(tz string) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone %s: %w", tz, err)
	}
	m.app.Cron().SetTimezone(loc)
	m.app.Logger().Info("Updated cron timezone", "timezone", tz)
	return nil
}

// RegisterInternalSystemJobs registers the built-in pb-ext maintenance job.
func (m *Manager) RegisterInternalSystemJobs() error {
	if err := m.RegisterJob(
		"__pbExtLogClean__",
		"__pbExtLogClean__",
		"Clean up pb-ext job logs older than 72 hours",
		"0 0 * * *",
		func(el *ExecutionLogger) {
			el.Start("Log Cleanup Job")
			el.Info("Log cleanup job started at: %s", time.Now().Format("2006-01-02 15:04:05"))

			m.app.Logger().Info("Running log cleanup job", "time", time.Now())

			cutoff := time.Now().Add(-72 * time.Hour)
			el.Info("Cleaning up job logs older than: %s", cutoff.Format("2006-01-02 15:04:05"))

			col, err := m.app.FindCollectionByNameOrId(Collection)
			if err != nil {
				el.Error("Failed to find job logs collection: %v", err)
				el.Fail(err)
				return
			}

			el.Success("Found job logs collection, proceeding with cleanup...")

			records, err := m.app.FindRecordsByFilter(col,
				"created < {:cutoff}", "-created", 10000, 0,
				map[string]any{"cutoff": cutoff.Format("2006-01-02 15:04:05.000Z")},
			)
			if err != nil {
				el.Error("Failed to find old job log records: %v", err)
				el.Fail(err)
				return
			}

			el.Info("Found %d old job log records to clean up", len(records))

			deleted, failed := 0, 0
			for _, rec := range records {
				if err := m.app.Delete(rec); err != nil {
					el.Error("Failed to delete job log record %s: %v", rec.Id, err)
					failed++
				} else {
					deleted++
				}
			}

			el.Statistics(map[string]interface{}{
				"total_found":  len(records),
				"deleted":      deleted,
				"failed":       failed,
				"cutoff_hours": 72,
				"cutoff_date":  cutoff.Format("2006-01-02 15:04:05"),
			})

			if failed > 0 {
				el.Warn("Cleanup completed with some failures: deleted %d/%d records", deleted, len(records))
			} else {
				el.Success("Cleanup completed successfully: deleted %d records", deleted)
			}

			el.Complete(fmt.Sprintf("Log cleanup finished - deleted %d/%d records", deleted, len(records)))
			m.app.Logger().Info("Log cleanup completed", "deleted_records", deleted, "failed_deletions", failed)
		},
	); err != nil {
		return fmt.Errorf("failed to register log cleanup job: %w", err)
	}

	if err := m.RegisterJob(
		"__pbExtAnalyticsClean__",
		"__pbExtAnalyticsClean__",
		"Delete _analytics records older than 90 days",
		"0 3 * * *",
		func(el *ExecutionLogger) {
			el.Start("Analytics Cleanup Job")

			cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
			el.Info("Deleting _analytics records with date < %s", cutoff)

			res, err := m.app.NonconcurrentDB().
				NewQuery("DELETE FROM _analytics WHERE date < {:cutoff}").
				Bind(dbx.Params{"cutoff": cutoff}).
				Execute()
			if err != nil {
				el.Error("Failed to delete old analytics records: %v", err)
				el.Fail(err)
				return
			}

			affected, _ := res.RowsAffected()

			el.Statistics(map[string]interface{}{
				"deleted":        affected,
				"retention_days": 90,
				"cutoff_date":    cutoff,
			})
			el.Success("Cleanup completed: deleted %d _analytics rows", affected)
			el.Complete(fmt.Sprintf("Analytics cleanup finished - deleted %d rows", affected))
			m.app.Logger().Info("Analytics cleanup completed", "deleted", affected)
		},
	); err != nil {
		return fmt.Errorf("failed to register analytics cleanup job: %w", err)
	}

	m.app.Logger().Info("✅ Internal system jobs registered")
	return nil
}

// Logger returns the underlying job Logger (needed for HTTP handlers).
func (m *Manager) Logger() *Logger {
	return m.logger
}

// --- private ---

func (m *Manager) wrap(jobID, jobName, description, expression string, fn func(*ExecutionLogger)) func() {
	return func() {
		startTime := time.Now()

		m.logger.activeJobsMux.Lock()
		_, alreadyLogged := m.logger.activeJobs[jobID]
		m.logger.activeJobsMux.Unlock()

		if !alreadyLogged && m.logger != nil {
			m.logger.LogJobStartWithDescription(jobID, jobName, description, expression, "scheduled", "")
		}

		factory := newLoggerFactory(m.logger)
		execLogger := factory.create(jobID)

		var errorMsg string
		func() {
			defer func() {
				if r := recover(); r != nil {
					errorMsg = fmt.Sprintf("Job panic: %v", r)
					execLogger.Fail(fmt.Errorf("%s", errorMsg))
				}
			}()
			fn(execLogger)
		}()

		capturedOutput := execLogger.GetOutput()
		duration := time.Since(startTime)

		if !alreadyLogged && m.logger != nil {
			if errorMsg != "" {
				m.logger.LogJobError(jobID, errorMsg)
			} else {
				m.logger.LogJobComplete(jobID, capturedOutput, "")
			}
		}

		if errorMsg != "" {
			m.app.Logger().Error("Job execution failed",
				"job_id", jobID, "job_name", jobName, "duration", duration, "error", errorMsg)
		} else {
			m.app.Logger().Info("Job execution completed",
				"job_id", jobID, "job_name", jobName, "duration", duration, "output_length", len(capturedOutput))
		}
	}
}

func isSystemJob(jobID string) bool {
	return slices.Contains(SystemJobIDs, jobID)
}

// --- global singleton ---

var globalManager *Manager

// GetManager returns the global Manager instance (set by Initialize).
func GetManager() *Manager {
	return globalManager
}
