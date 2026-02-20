package jobs_test

import (
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/magooney-loon/pb-ext/core/testutil"
)

// --- Constants ---

func TestStatusConstants(t *testing.T) {
	cases := []struct{ name, val string }{
		{"StatusStarted", jobs.StatusStarted},
		{"StatusCompleted", jobs.StatusCompleted},
		{"StatusFailed", jobs.StatusFailed},
		{"StatusTimeout", jobs.StatusTimeout},
	}
	expected := map[string]string{
		"StatusStarted":   "started",
		"StatusCompleted": "completed",
		"StatusFailed":    "failed",
		"StatusTimeout":   "timeout",
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.val != expected[c.name] {
				t.Errorf("expected %q, got %q", expected[c.name], c.val)
			}
		})
	}
}

func TestCollectionName(t *testing.T) {
	if jobs.Collection != "_job_logs" {
		t.Errorf("expected _job_logs, got %q", jobs.Collection)
	}
}

func TestSystemJobIDs(t *testing.T) {
	required := []string{
		"__pbLogsCleanup__",
		"__pbOTPCleanup__",
		"__pbMFACleanup__",
		"__pbDBOptimize__",
		"__pbExtLogClean__",
	}
	idx := make(map[string]bool, len(jobs.SystemJobIDs))
	for _, id := range jobs.SystemJobIDs {
		idx[id] = true
	}
	for _, id := range required {
		if !idx[id] {
			t.Errorf("SystemJobIDs missing %q", id)
		}
	}
}

// --- Types ---

func TestJobMetadata_ZeroValue(t *testing.T) {
	var m jobs.JobMetadata
	if m.ID != "" || m.Name != "" || m.IsActive || m.IsSystemJob {
		t.Error("zero-value JobMetadata should have empty/false fields")
	}
}

func TestExecutionResult_Fields(t *testing.T) {
	now := time.Now()
	r := jobs.ExecutionResult{
		JobID:       "job1",
		Success:     true,
		Duration:    5 * time.Second,
		Output:      "ok",
		TriggerType: "scheduled",
		TriggerBy:   "cron",
		ExecutedAt:  now,
	}
	if r.JobID != "job1" {
		t.Errorf("JobID mismatch")
	}
	if !r.Success {
		t.Errorf("expected Success=true")
	}
	if r.Duration != 5*time.Second {
		t.Errorf("Duration mismatch")
	}
	if r.ExecutedAt != now {
		t.Errorf("ExecutedAt mismatch")
	}
}

func TestListOptions_Defaults(t *testing.T) {
	var opts jobs.ListOptions
	if opts.IncludeSystemJobs || opts.ActiveOnly {
		t.Error("zero-value ListOptions should have false fields")
	}
}

func TestLogsData_ZeroValue(t *testing.T) {
	var d jobs.LogsData
	if d.TotalExecutions != 0 || d.SuccessRate != 0 {
		t.Error("zero-value LogsData should be empty")
	}
	if d.RecentExecutions != nil {
		t.Error("RecentExecutions should be nil for zero value")
	}
}

func TestPaginationData(t *testing.T) {
	p := jobs.PaginationData{
		Page:       1,
		PerPage:    20,
		Total:      100,
		TotalPages: 5,
		HasNext:    true,
		HasPrev:    false,
	}
	if p.TotalPages != 5 {
		t.Errorf("TotalPages: expected 5, got %d", p.TotalPages)
	}
	if !p.HasNext {
		t.Error("expected HasNext=true")
	}
}

func TestAPIResponse_Fields(t *testing.T) {
	resp := jobs.APIResponse{Message: "ok", Success: true, Data: 42}
	if resp.Message != "ok" {
		t.Errorf("Message mismatch")
	}
	if !resp.Success {
		t.Error("expected Success=true")
	}
}

// --- Collection ---

func TestSetupCollection_CreatesCollection(t *testing.T) {
	app := testutil.NewTestApp(t)

	if err := jobs.SetupCollection(app); err != nil {
		t.Fatalf("SetupCollection: %v", err)
	}

	col, err := app.FindCollectionByNameOrId(jobs.Collection)
	if err != nil {
		t.Fatalf("collection not found after setup: %v", err)
	}
	if col.Name != "_job_logs" {
		t.Errorf("expected _job_logs, got %q", col.Name)
	}
	if !col.System {
		t.Error("_job_logs must be a system collection")
	}
}

func TestSetupCollection_Idempotent(t *testing.T) {
	app := testutil.NewTestApp(t)

	if err := jobs.SetupCollection(app); err != nil {
		t.Fatalf("first SetupCollection: %v", err)
	}
	if err := jobs.SetupCollection(app); err != nil {
		t.Fatalf("second SetupCollection (idempotent): %v", err)
	}
}

func TestSetupCollection_RequiredFields(t *testing.T) {
	app := testutil.NewTestApp(t)

	if err := jobs.SetupCollection(app); err != nil {
		t.Fatal(err)
	}

	col, err := app.FindCollectionByNameOrId(jobs.Collection)
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"job_id", "job_name", "status", "start_time", "trigger_type",
	}
	for _, name := range required {
		if col.Fields.GetByName(name) == nil {
			t.Errorf("required field %q missing from _job_logs", name)
		}
	}
}

func TestSetupCollection_OptionalFields(t *testing.T) {
	app := testutil.NewTestApp(t)

	if err := jobs.SetupCollection(app); err != nil {
		t.Fatal(err)
	}

	col, err := app.FindCollectionByNameOrId(jobs.Collection)
	if err != nil {
		t.Fatal(err)
	}

	optional := []string{
		"description", "expression", "end_time", "duration",
		"output", "error", "trigger_by",
	}
	for _, name := range optional {
		if col.Fields.GetByName(name) == nil {
			t.Errorf("optional field %q missing from _job_logs", name)
		}
	}
}

// --- Logger ---

func TestNewLogger_Defaults(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)

	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestInitializeLogger_SetsUpCollection(t *testing.T) {
	// Use bare app — InitializeLogger should call SetupCollection itself
	app := testutil.NewTestApp(t)

	l, err := jobs.InitializeLogger(app)
	if err != nil {
		t.Fatalf("InitializeLogger: %v", err)
	}
	if l == nil {
		t.Fatal("InitializeLogger returned nil Logger")
	}

	// Collection must exist now
	_, err = app.FindCollectionByNameOrId(jobs.Collection)
	if err != nil {
		t.Fatalf("_job_logs collection not found after InitializeLogger: %v", err)
	}
}

func TestLogger_LogJobStart_CreatesRecord(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)

	l.LogJobStart("test-job", "Test Job", "*/5 * * * *", "scheduled", "")

	// Give the logger time to save (it saves synchronously in saveRecord)
	time.Sleep(50 * time.Millisecond)

	col, _ := app.FindCollectionByNameOrId(jobs.Collection)
	records, err := app.FindAllRecords(col)
	if err != nil {
		t.Fatalf("FindAllRecords: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one record after LogJobStart")
	}

	rec := records[0]
	if rec.GetString("job_id") != "test-job" {
		t.Errorf("job_id: expected %q, got %q", "test-job", rec.GetString("job_id"))
	}
	if rec.GetString("status") != jobs.StatusStarted {
		t.Errorf("status: expected %q, got %q", jobs.StatusStarted, rec.GetString("status"))
	}
}

func TestLogger_LogJobComplete_UpdatesStatus(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)

	l.LogJobStart("complete-job", "Complete Job", "*/5 * * * *", "scheduled", "")
	time.Sleep(50 * time.Millisecond)

	l.LogJobComplete("complete-job", "job done", "")
	time.Sleep(50 * time.Millisecond)

	col, _ := app.FindCollectionByNameOrId(jobs.Collection)
	records, err := app.FindAllRecords(col)
	if err != nil {
		t.Fatalf("FindAllRecords: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("no records found")
	}

	// Find the updated record
	var found bool
	for _, rec := range records {
		if rec.GetString("job_id") == "complete-job" {
			status := rec.GetString("status")
			if status == jobs.StatusCompleted {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a record with status=completed for complete-job")
	}
}

func TestLogger_ForceFlush(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)

	// ForceFlush on empty buffer should not panic
	l.ForceFlush()
}

// --- Manager ---

func TestNewManager_NotNil(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManager_RegisterJob(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	called := make(chan struct{}, 1)
	err := m.RegisterJob("test-job", "Test Job", "A test job", "*/5 * * * *",
		func(el *jobs.ExecutionLogger) { called <- struct{}{} })
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	// Job should be in registry
	meta, err := m.GetJobMetadata("test-job")
	if err != nil {
		t.Fatalf("GetJobMetadata: %v", err)
	}
	if meta.ID != "test-job" {
		t.Errorf("ID: expected test-job, got %q", meta.ID)
	}
	if meta.Name != "Test Job" {
		t.Errorf("Name: expected 'Test Job', got %q", meta.Name)
	}
	if meta.Expression != "*/5 * * * *" {
		t.Errorf("Expression mismatch")
	}
	if !meta.IsActive {
		t.Error("expected IsActive=true")
	}
}

func TestManager_RegisterJob_FallbackName(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	// Empty name — should fall back to jobID
	err := m.RegisterJob("fallback-id", "", "", "*/10 * * * *", func(_ *jobs.ExecutionLogger) {})
	if err != nil {
		t.Fatalf("RegisterJob: %v", err)
	}

	meta, err := m.GetJobMetadata("fallback-id")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name != "fallback-id" {
		t.Errorf("expected name fallback to id, got %q", meta.Name)
	}
}

func TestManager_RegisterJob_InvalidExpression(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	err := m.RegisterJob("bad-job", "Bad Job", "", "not-a-cron-expr", func(_ *jobs.ExecutionLogger) {})
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestManager_GetJobs(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	if err := m.RegisterJob("job-a", "Job A", "", "*/5 * * * *", func(_ *jobs.ExecutionLogger) {}); err != nil {
		t.Fatal(err)
	}
	if err := m.RegisterJob("job-b", "Job B", "", "*/10 * * * *", func(_ *jobs.ExecutionLogger) {}); err != nil {
		t.Fatal(err)
	}

	all := m.GetJobs(jobs.ListOptions{IncludeSystemJobs: false})
	ids := make(map[string]bool)
	for _, j := range all {
		ids[j.ID] = true
	}
	if !ids["job-a"] || !ids["job-b"] {
		t.Errorf("GetJobs missing registered jobs; got: %v", all)
	}
}

func TestManager_RemoveJob(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	if err := m.RegisterJob("rm-job", "Remove Me", "", "*/5 * * * *", func(_ *jobs.ExecutionLogger) {}); err != nil {
		t.Fatal(err)
	}

	if err := m.RemoveJob("rm-job"); err != nil {
		t.Fatalf("RemoveJob: %v", err)
	}

	_, err := m.GetJobMetadata("rm-job")
	if err == nil {
		t.Error("expected error after removing job, got nil")
	}
}

func TestManager_GetSystemStatus(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	status := m.GetSystemStatus()
	if status == nil {
		t.Fatal("GetSystemStatus returned nil")
	}
	if _, ok := status["total_jobs"]; !ok {
		t.Error("GetSystemStatus missing 'total_jobs' key")
	}
	if _, ok := status["status"]; !ok {
		t.Error("GetSystemStatus missing 'status' key")
	}
}

func TestManager_UpdateTimezone_Valid(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	if err := m.UpdateTimezone("America/New_York"); err != nil {
		t.Errorf("UpdateTimezone(valid): %v", err)
	}
}

func TestManager_UpdateTimezone_Invalid(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	if err := m.UpdateTimezone("Not/AReal/Timezone"); err == nil {
		t.Error("expected error for invalid timezone")
	}
}

func TestManager_IsSystemJob(t *testing.T) {
	app := testutil.NewTestAppWithJobs(t)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)

	// Register a system-id job — IsSystemJob should be true
	if err := m.RegisterJob("__pbLogsCleanup__", "", "", "0 * * * *", func(_ *jobs.ExecutionLogger) {}); err != nil {
		t.Fatalf("RegisterJob system: %v", err)
	}

	meta, err := m.GetJobMetadata("__pbLogsCleanup__")
	if err != nil {
		t.Fatal(err)
	}
	if !meta.IsSystemJob {
		t.Error("expected IsSystemJob=true for __pbLogsCleanup__")
	}
}

// --- Initialize / GetManager ---

func TestInitialize_SetsGlobal(t *testing.T) {
	app := testutil.NewTestApp(t)

	m, err := jobs.Initialize(app)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if m == nil {
		t.Fatal("Initialize returned nil Manager")
	}

	got := jobs.GetManager()
	if got == nil {
		t.Fatal("GetManager returned nil after Initialize")
	}
	if got != m {
		t.Error("GetManager did not return the initialized Manager")
	}
}

// --- Benchmarks ---

func BenchmarkRegisterJob(b *testing.B) {
	app := testutil.NewTestAppWithJobs(b)
	l := jobs.NewLogger(app)
	m := jobs.NewManager(app, l)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := "bench-job"
		_ = m.RegisterJob(id, "Bench", "", "*/5 * * * *", func(_ *jobs.ExecutionLogger) {})
		_ = m.RemoveJob(id)
	}
}
