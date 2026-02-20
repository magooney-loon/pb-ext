package jobs

import "time"

// Job logging constants
const (
	LogsLookbackDays  = 30
	MaxLogsRecords    = 10000
	LogsFlushWaitTime = 30 * time.Second
	Collection        = "_job_logs"
)

// Job status constants
const (
	StatusStarted   = "started"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusTimeout   = "timeout"
)

// SystemJobIDs lists the built-in PocketBase job IDs that are treated as system jobs.
var SystemJobIDs = []string{
	"__pbLogsCleanup__",
	"__pbOTPCleanup__",
	"__pbMFACleanup__",
	"__pbDBOptimize__",
	"__pbExtLogClean__",
}

// JobMetadata holds registration info for a cron job.
type JobMetadata struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Expression  string                    `json:"expression"`
	IsSystemJob bool                      `json:"is_system_job"`
	CreatedAt   time.Time                 `json:"created_at"`
	IsActive    bool                      `json:"is_active"`
	Function    func(*ExecutionLogger)    `json:"-"`
}

// ExecutionResult is the outcome of a single job run.
type ExecutionResult struct {
	JobID       string        `json:"job_id"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Output      string        `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	TriggerType string        `json:"trigger_type"`
	TriggerBy   string        `json:"trigger_by,omitempty"`
	ExecutedAt  time.Time     `json:"executed_at"`
}

// ListOptions filters job listings.
type ListOptions struct {
	IncludeSystemJobs bool
	ActiveOnly        bool
}

// JobLog is a single execution log entry stored in PocketBase.
type JobLog struct {
	RecordID    string            `json:"record_id,omitempty"`
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
	TriggerType string            `json:"trigger_type"`
	TriggerBy   string            `json:"trigger_by,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// LogsData is the aggregated analytics payload for job logs.
type LogsData struct {
	TotalExecutions     int64            `json:"total_executions"`
	SuccessfulRuns      int64            `json:"successful_runs"`
	FailedRuns          int64            `json:"failed_runs"`
	SuccessRate         float64          `json:"success_rate"`
	AverageRunTime      time.Duration    `json:"average_run_time"`
	RecentExecutions    []LogSummary     `json:"recent_executions"`
	JobStats            []StatSummary    `json:"job_stats"`
	TodayExecutions     int64            `json:"today_executions"`
	YesterdayExecutions int64            `json:"yesterday_executions"`
	HourlyActivity      map[string]int64 `json:"hourly_activity"`
}

// LogSummary is a condensed view of one execution log.
type LogSummary struct {
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

// StatSummary holds per-job aggregate statistics.
type StatSummary struct {
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

// APIResponse is the standard envelope for job HTTP responses.
type APIResponse struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// PaginationData carries pagination metadata in list responses.
type PaginationData struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}
