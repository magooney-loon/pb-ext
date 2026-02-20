package jobs

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Logger manages buffered persistence of job execution logs to PocketBase.
type Logger struct {
	app           core.App
	buffer        []JobLog
	bufferMutex   sync.RWMutex
	flushInterval time.Duration
	batchSize     int
	lastFlushTime time.Time
	flushChan     chan struct{}
	flushTicker   *time.Ticker
	flushActive   sync.Mutex

	activeJobs    map[string]*JobLog
	activeJobsMux sync.RWMutex
}

// NewLogger creates a Logger. Call InitializeLogger instead for normal use.
func NewLogger(app core.App) *Logger {
	return &Logger{
		app:           app,
		buffer:        make([]JobLog, 0),
		flushInterval: LogsFlushWaitTime,
		batchSize:     100,
		lastFlushTime: time.Now(),
		flushChan:     make(chan struct{}, 1),
		activeJobs:    make(map[string]*JobLog),
	}
}

// InitializeLogger sets up the job logs collection and starts background workers.
func InitializeLogger(app core.App) (*Logger, error) {
	l := NewLogger(app)

	if err := SetupCollection(app); err != nil {
		return nil, fmt.Errorf("failed to setup job logs collection: %w", err)
	}

	go l.backgroundFlushWorker()
	go l.startFlushTimer()

	app.Logger().Info("Job logging system initialized")
	return l, nil
}

// LogJobStart records the start of a scheduled or API-triggered execution.
func (l *Logger) LogJobStart(jobID, jobName, expression, triggerType, triggerBy string) {
	l.LogJobStartWithDescription(jobID, jobName, "", expression, triggerType, triggerBy)
}

// LogJobStartWithDescription records the start of a job execution including description.
func (l *Logger) LogJobStartWithDescription(jobID, jobName, description, expression, triggerType, triggerBy string) {
	l.activeJobsMux.Lock()
	defer l.activeJobsMux.Unlock()

	jl := &JobLog{
		JobID:       jobID,
		JobName:     jobName,
		Description: description,
		Expression:  expression,
		StartTime:   time.Now(),
		Status:      StatusStarted,
		TriggerType: triggerType,
		TriggerBy:   triggerBy,
		Metadata:    make(map[string]string),
	}

	if recordID := l.saveRecord(*jl); recordID != "" {
		jl.RecordID = recordID
	}

	l.activeJobs[jobID] = jl
}

// LogJobComplete finalises an active job log on success or failure.
func (l *Logger) LogJobComplete(jobID, output, errorMsg string) {
	l.activeJobsMux.Lock()
	defer l.activeJobsMux.Unlock()

	jl, exists := l.activeJobs[jobID]
	if !exists {
		return
	}

	now := time.Now()
	dur := now.Sub(jl.StartTime)

	jl.EndTime = &now
	jl.Duration = &dur
	jl.Output = output
	jl.Error = errorMsg

	if errorMsg != "" {
		jl.Status = StatusFailed
	} else {
		jl.Status = StatusCompleted
	}

	if jl.RecordID != "" {
		l.updateRecord(*jl)
	} else {
		l.addToBuffer(*jl)
	}

	delete(l.activeJobs, jobID)
}

// LogJobError is a convenience wrapper that marks a job as failed.
func (l *Logger) LogJobError(jobID, errorMsg string) {
	l.LogJobComplete(jobID, "", errorMsg)
}

// ForceFlush immediately flushes buffered logs to the database.
func (l *Logger) ForceFlush() {
	select {
	case l.flushChan <- struct{}{}:
	default:
	}
}

// GetLogsData returns aggregated analytics data for the job logs dashboard.
func (l *Logger) GetLogsData() (*LogsData, error) {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to find job logs collection: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	var total, successful, failed, todayCount, yesterdayCount int64

	if err := l.app.DB().Select("COUNT(*)").From(col.Name).Row(&total); err != nil {
		return nil, fmt.Errorf("failed to count total executions: %w", err)
	}
	if err := l.app.DB().Select("COUNT(*)").From(col.Name).
		Where(dbx.HashExp{"status": StatusCompleted}).Row(&successful); err != nil {
		return nil, fmt.Errorf("failed to count successful runs: %w", err)
	}
	if err := l.app.DB().Select("COUNT(*)").From(col.Name).
		Where(dbx.HashExp{"status": StatusFailed}).Row(&failed); err != nil {
		return nil, fmt.Errorf("failed to count failed runs: %w", err)
	}
	if err := l.app.DB().Select("COUNT(*)").From(col.Name).
		Where(dbx.NewExp("created >= {:today}", dbx.Params{"today": today})).Row(&todayCount); err != nil {
		return nil, fmt.Errorf("failed to count today's executions: %w", err)
	}
	if err := l.app.DB().Select("COUNT(*)").From(col.Name).
		Where(dbx.NewExp("created >= {:yesterday} AND created < {:today}",
			dbx.Params{"yesterday": yesterday, "today": today})).Row(&yesterdayCount); err != nil {
		return nil, fmt.Errorf("failed to count yesterday's executions: %w", err)
	}

	successRate := 0.0
	if total > 0 {
		successRate = float64(successful) / float64(total) * 100
	}

	recent, err := l.getRecentLogs(10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent executions: %w", err)
	}
	stats, err := l.getJobStatistics()
	if err != nil {
		return nil, fmt.Errorf("failed to get job statistics: %w", err)
	}
	avgRunTime, err := l.getAverageRunTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get average run time: %w", err)
	}

	return &LogsData{
		TotalExecutions:     total,
		SuccessfulRuns:      successful,
		FailedRuns:          failed,
		SuccessRate:         successRate,
		AverageRunTime:      avgRunTime,
		RecentExecutions:    recent,
		JobStats:            stats,
		TodayExecutions:     todayCount,
		YesterdayExecutions: yesterdayCount,
		HourlyActivity:      make(map[string]int64),
	}, nil
}

// GetRecentLogs returns the most recent log summaries (used by HTTP handlers).
func (l *Logger) GetRecentLogs(limit int) ([]LogSummary, error) {
	return l.getRecentLogs(limit)
}

// GetRecentLogsWithPagination returns paginated log summaries and total count.
func (l *Logger) GetRecentLogsWithPagination(limit, offset int) ([]LogSummary, int64, error) {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return nil, 0, err
	}

	var totalCount int64
	if err := l.app.DB().Select("COUNT(*)").From(col.Name).Row(&totalCount); err != nil {
		return nil, 0, err
	}

	records, err := l.app.FindRecordsByFilter(col, "", "-created", limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return recordsToSummaries(records), totalCount, nil
}

// GetLogsByJobIDWithPagination returns paginated logs for a specific job.
func (l *Logger) GetLogsByJobIDWithPagination(jobID string, limit, offset int) ([]LogSummary, int64, error) {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return nil, 0, err
	}

	var totalCount int64
	if err := l.app.DB().Select("COUNT(*)").From(col.Name).
		Where(dbx.NewExp("job_id = {:job_id}", dbx.Params{"job_id": jobID})).
		Row(&totalCount); err != nil {
		return nil, 0, err
	}

	records, err := l.app.FindRecordsByFilter(col,
		"job_id = {:job_id}", "-created", limit, offset,
		dbx.Params{"job_id": jobID},
	)
	if err != nil {
		return nil, 0, err
	}

	return recordsToSummaries(records), totalCount, nil
}

// --- internal helpers ---

func (l *Logger) backgroundFlushWorker() {
	for range l.flushChan {
		l.flushBuffer()
	}
}

func (l *Logger) startFlushTimer() {
	l.flushTicker = time.NewTicker(l.flushInterval)
	defer l.flushTicker.Stop()

	for range l.flushTicker.C {
		l.bufferMutex.RLock()
		n := len(l.buffer)
		l.bufferMutex.RUnlock()

		if n > 0 {
			select {
			case l.flushChan <- struct{}{}:
			default:
			}
		}
	}
}

func (l *Logger) flushBuffer() {
	l.flushActive.Lock()
	defer l.flushActive.Unlock()

	l.bufferMutex.Lock()
	if len(l.buffer) == 0 {
		l.bufferMutex.Unlock()
		return
	}
	toFlush := make([]JobLog, len(l.buffer))
	copy(toFlush, l.buffer)
	l.buffer = l.buffer[:0]
	l.bufferMutex.Unlock()

	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		l.app.Logger().Error("Failed to find job logs collection", "error", err)
		return
	}

	ok := 0
	for _, jl := range toFlush {
		rec := core.NewRecord(col)
		setRecordFields(rec, jl)
		if err := l.app.SaveNoValidate(rec); err != nil {
			l.app.Logger().Error("Failed to save job log", "error", err, "job_id", jl.JobID)
		} else {
			ok++
		}
	}

	l.lastFlushTime = time.Now()
	l.app.Logger().Debug("Flushed job logs", "flushed", ok, "failed", len(toFlush)-ok)
	l.cleanupOldRecords()
}

func (l *Logger) cleanupOldRecords() {
	cutoff := time.Now().AddDate(0, 0, -LogsLookbackDays)

	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return
	}

	records, err := l.app.FindRecordsByFilter(col,
		"created < {:cutoff}", "-created", MaxLogsRecords, 0,
		dbx.Params{"cutoff": cutoff},
	)
	if err != nil {
		l.app.Logger().Error("Failed to find old job log records", "error", err)
		return
	}

	deleted := 0
	for _, rec := range records {
		if err := l.app.Delete(rec); err != nil {
			l.app.Logger().Error("Failed to delete old job log record", "error", err, "id", rec.Id)
		} else {
			deleted++
		}
	}
	if deleted > 0 {
		l.app.Logger().Debug("Cleaned up old job log records", "deleted", deleted)
	}
}

func (l *Logger) saveRecord(jl JobLog) string {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		l.app.Logger().Error("Failed to find job logs collection", "error", err)
		return ""
	}

	rec := core.NewRecord(col)
	setRecordFields(rec, jl)

	if err := l.app.SaveNoValidate(rec); err != nil {
		l.app.Logger().Error("Failed to save job log record", "error", err, "job_id", jl.JobID)
		return ""
	}
	return rec.Id
}

func (l *Logger) updateRecord(jl JobLog) {
	if jl.RecordID == "" {
		return
	}

	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		l.app.Logger().Error("Failed to find job logs collection", "error", err)
		return
	}

	rec, err := l.app.FindRecordById(col, jl.RecordID)
	if err != nil {
		l.app.Logger().Error("Failed to find job log record", "error", err, "record_id", jl.RecordID)
		return
	}

	setRecordFields(rec, jl)
	if err := l.app.SaveNoValidate(rec); err != nil {
		l.app.Logger().Error("Failed to update job log record", "error", err, "job_id", jl.JobID)
	}
}

func (l *Logger) addToBuffer(jl JobLog) {
	l.bufferMutex.Lock()
	defer l.bufferMutex.Unlock()

	l.buffer = append(l.buffer, jl)
	if len(l.buffer) >= l.batchSize {
		select {
		case l.flushChan <- struct{}{}:
		default:
		}
	}
}

func (l *Logger) getRecentLogs(limit int) ([]LogSummary, error) {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return nil, err
	}
	records, err := l.app.FindRecordsByFilter(col, "", "-created", limit, 0)
	if err != nil {
		return nil, err
	}
	return recordsToSummaries(records), nil
}

func (l *Logger) getJobStatistics() ([]StatSummary, error) {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return nil, err
	}

	records, err := l.app.FindRecordsByFilter(col, "", "job_id", 0, 0)
	if err != nil {
		return nil, err
	}

	jobMap := make(map[string]*StatSummary)
	for _, rec := range records {
		jobID := rec.GetString("job_id")
		jobName := rec.GetString("job_name")

		if _, exists := jobMap[jobID]; !exists {
			jobMap[jobID] = &StatSummary{JobID: jobID, JobName: jobName}
		}

		stat := jobMap[jobID]
		stat.TotalRuns++

		switch rec.GetString("status") {
		case StatusCompleted:
			stat.SuccessfulRuns++
		case StatusFailed:
			stat.FailedRuns++
		}

		startTime := rec.GetDateTime("start_time").Time()
		if stat.LastRun == nil || startTime.After(*stat.LastRun) {
			stat.LastRun = &startTime
		}

		if dms := rec.GetInt("duration"); dms > 0 {
			d := time.Duration(dms) * time.Millisecond
			stat.AverageRunTime = (stat.AverageRunTime*time.Duration(stat.TotalRuns-1) + d) / time.Duration(stat.TotalRuns)
		}
	}

	result := make([]StatSummary, 0, len(jobMap))
	for _, stat := range jobMap {
		if stat.TotalRuns > 0 {
			stat.SuccessRate = float64(stat.SuccessfulRuns) / float64(stat.TotalRuns) * 100
		}
		result = append(result, *stat)
	}
	return result, nil
}

func (l *Logger) getAverageRunTime() (time.Duration, error) {
	col, err := l.app.FindCollectionByNameOrId(Collection)
	if err != nil {
		return 0, err
	}

	records, err := l.app.FindRecordsByFilter(col,
		"duration > 0 AND status = 'completed'", "", 0, 0,
	)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}

	var total time.Duration
	for _, rec := range records {
		if dms := rec.GetInt("duration"); dms > 0 {
			total += time.Duration(dms) * time.Millisecond
		}
	}
	return total / time.Duration(len(records)), nil
}

// --- shared helpers ---

func setRecordFields(rec *core.Record, jl JobLog) {
	rec.Set("job_id", jl.JobID)
	rec.Set("job_name", jl.JobName)
	rec.Set("description", jl.Description)
	rec.Set("expression", jl.Expression)
	rec.Set("start_time", jl.StartTime)
	if jl.EndTime != nil {
		rec.Set("end_time", *jl.EndTime)
	}
	if jl.Duration != nil {
		rec.Set("duration", int64(*jl.Duration/time.Millisecond))
	}
	rec.Set("status", jl.Status)
	rec.Set("output", jl.Output)
	rec.Set("error", jl.Error)
	rec.Set("trigger_type", jl.TriggerType)
	rec.Set("trigger_by", jl.TriggerBy)
}

func recordsToSummaries(records []*core.Record) []LogSummary {
	summaries := make([]LogSummary, len(records))
	for i, rec := range records {
		var dur *time.Duration
		if dms := rec.GetInt("duration"); dms > 0 {
			d := time.Duration(dms) * time.Millisecond
			dur = &d
		}
		summaries[i] = LogSummary{
			ID:          rec.Id,
			JobID:       rec.GetString("job_id"),
			JobName:     rec.GetString("job_name"),
			StartTime:   rec.GetDateTime("start_time").Time(),
			Duration:    dur,
			Status:      rec.GetString("status"),
			TriggerType: rec.GetString("trigger_type"),
			Output:      rec.GetString("output"),
			Error:       rec.GetString("error"),
		}
	}
	return summaries
}

// ExecutionLogger is the per-job structured logger passed to job functions.
type ExecutionLogger struct {
	jobID       string
	executionID string
	buffer      *bytes.Buffer
	mutex       *sync.Mutex
	startTime   time.Time
	logger      *Logger
}

// NewExecutionLogger creates a logger for a single job execution.
func NewExecutionLogger(jobID, executionID string, l *Logger) *ExecutionLogger {
	return &ExecutionLogger{
		jobID:       jobID,
		executionID: executionID,
		buffer:      &bytes.Buffer{},
		mutex:       &sync.Mutex{},
		startTime:   time.Now(),
		logger:      l,
	}
}

func (el *ExecutionLogger) Info(format string, args ...interface{})  { el.log("INFO", format, args...) }
func (el *ExecutionLogger) Error(format string, args ...interface{}) { el.log("ERROR", format, args...) }
func (el *ExecutionLogger) Debug(format string, args ...interface{}) { el.log("DEBUG", format, args...) }
func (el *ExecutionLogger) Warn(format string, args ...interface{})  { el.log("WARN", format, args...) }

func (el *ExecutionLogger) Success(format string, args ...interface{}) {
	el.log("INFO", "✅ "+format, args...)
}
func (el *ExecutionLogger) Progress(format string, args ...interface{}) {
	el.log("INFO", "🔄 "+format, args...)
}
func (el *ExecutionLogger) Start(jobName string) {
	el.log("INFO", "🚀 Starting job: %s", jobName)
}
func (el *ExecutionLogger) Complete(message string) {
	el.log("INFO", "✅ Job completed successfully in %v: %s", time.Since(el.startTime), message)
}
func (el *ExecutionLogger) Fail(err error) {
	el.log("ERROR", "❌ Job failed after %v: %v", time.Since(el.startTime), err)
}
func (el *ExecutionLogger) Statistics(stats map[string]interface{}) {
	el.log("INFO", "📊 Statistics:")
	for k, v := range stats {
		el.log("INFO", "   • %s: %v", k, v)
	}
}

func (el *ExecutionLogger) GetOutput() string {
	el.mutex.Lock()
	defer el.mutex.Unlock()
	return el.buffer.String()
}

func (el *ExecutionLogger) GetDuration() time.Duration {
	return time.Since(el.startTime)
}

func (el *ExecutionLogger) WithContext(key, value string) *ExecutionLogger {
	return &ExecutionLogger{
		jobID:       fmt.Sprintf("%s[%s=%s]", el.jobID, key, value),
		executionID: el.executionID,
		buffer:      el.buffer,
		mutex:       el.mutex,
		startTime:   el.startTime,
		logger:      el.logger,
	}
}

func (el *ExecutionLogger) log(level, format string, args ...interface{}) {
	el.mutex.Lock()
	defer el.mutex.Unlock()

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	el.buffer.WriteString(fmt.Sprintf("[%s] [%s] [%s] %s\n", ts, level, el.jobID, msg))
}

// loggerFactory creates ExecutionLoggers (used internally by Manager).
type loggerFactory struct {
	logger *Logger
}

func newLoggerFactory(l *Logger) *loggerFactory {
	return &loggerFactory{logger: l}
}

func (f *loggerFactory) create(jobID string) *ExecutionLogger {
	execID := fmt.Sprintf("%s_%d", jobID, time.Now().UnixNano())
	return NewExecutionLogger(jobID, execID, f.logger)
}
