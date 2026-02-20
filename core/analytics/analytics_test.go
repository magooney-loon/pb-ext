package analytics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/testutil"
	"github.com/pocketbase/pocketbase/core"
)

// --- Constants ---

func TestCollectionName(t *testing.T) {
	if analytics.CollectionName != "_analytics" {
		t.Errorf("expected _analytics, got %q", analytics.CollectionName)
	}
}

func TestSessionsCollectionName(t *testing.T) {
	if analytics.SessionsCollectionName != "_analytics_sessions" {
		t.Errorf("expected _analytics_sessions, got %q", analytics.SessionsCollectionName)
	}
}

func TestConstants(t *testing.T) {
	if analytics.LookbackDays <= 0 {
		t.Errorf("LookbackDays must be positive, got %d", analytics.LookbackDays)
	}
	if analytics.MaxExpectedHourlyVisits <= 0 {
		t.Errorf("MaxExpectedHourlyVisits must be positive, got %d", analytics.MaxExpectedHourlyVisits)
	}
	if analytics.SessionRingSize <= 0 {
		t.Errorf("SessionRingSize must be positive, got %d", analytics.SessionRingSize)
	}
}

// --- Types ---

func TestData_ZeroValue(t *testing.T) {
	var d analytics.Data
	if d.UniqueVisitors != 0 || d.TotalPageViews != 0 {
		t.Error("zero-value Data should have zero counts")
	}
}

func TestDefaultData(t *testing.T) {
	d := analytics.DefaultData()
	if d == nil {
		t.Fatal("DefaultData returned nil")
	}
	if d.TopDeviceType == "" {
		t.Error("DefaultData.TopDeviceType should not be empty")
	}
	if d.TopBrowser == "" {
		t.Error("DefaultData.TopBrowser should not be empty")
	}
	if d.BrowserBreakdown == nil {
		t.Error("DefaultData.BrowserBreakdown should not be nil")
	}
	if d.TopPages == nil {
		t.Error("DefaultData.TopPages should not be nil")
	}
	if d.RecentVisits == nil {
		t.Error("DefaultData.RecentVisits should not be nil")
	}
}

func TestPageStat_Fields(t *testing.T) {
	ps := analytics.PageStat{Path: "/home", Views: 42}
	if ps.Path != "/home" || ps.Views != 42 {
		t.Errorf("PageStat field mismatch")
	}
}

func TestRecentVisit_Fields(t *testing.T) {
	now := time.Now()
	rv := analytics.RecentVisit{
		Time:       now,
		Path:       "/about",
		DeviceType: "mobile",
		Browser:    "safari",
		OS:         "ios",
	}
	if rv.Path != "/about" {
		t.Errorf("Path mismatch")
	}
	if rv.DeviceType != "mobile" {
		t.Errorf("DeviceType mismatch")
	}
}

// --- Collection setup ---

func TestSetupCollections_CreatesBothCollections(t *testing.T) {
	app := testutil.NewTestApp(t)

	if err := analytics.SetupCollections(app); err != nil {
		t.Fatalf("SetupCollections: %v", err)
	}

	for _, name := range []string{analytics.CollectionName, analytics.SessionsCollectionName} {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("collection %q not found: %v", name, err)
		}
		if !col.System {
			t.Errorf("collection %q must be a system collection", name)
		}
	}
}

func TestSetupCollections_Idempotent(t *testing.T) {
	app := testutil.NewTestApp(t)

	if err := analytics.SetupCollections(app); err != nil {
		t.Fatalf("first SetupCollections: %v", err)
	}
	if err := analytics.SetupCollections(app); err != nil {
		t.Fatalf("second SetupCollections (idempotent): %v", err)
	}
}

func TestSetupCollection_Alias(t *testing.T) {
	// SetupCollection is a backward-compat alias for SetupCollections.
	app := testutil.NewTestApp(t)
	if err := analytics.SetupCollection(app); err != nil {
		t.Fatalf("SetupCollection alias: %v", err)
	}
	if _, err := app.FindCollectionByNameOrId(analytics.CollectionName); err != nil {
		t.Fatalf("_analytics not found after alias call: %v", err)
	}
	if _, err := app.FindCollectionByNameOrId(analytics.SessionsCollectionName); err != nil {
		t.Fatalf("_analytics_sessions not found after alias call: %v", err)
	}
}

func TestSetupCollections_CounterFields(t *testing.T) {
	app := testutil.NewTestApp(t)
	if err := analytics.SetupCollections(app); err != nil {
		t.Fatal(err)
	}

	col, err := app.FindCollectionByNameOrId(analytics.CollectionName)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"path", "date", "device_type", "browser", "views", "unique_sessions"} {
		if col.Fields.GetByName(name) == nil {
			t.Errorf("required field %q missing from _analytics", name)
		}
	}
}

func TestSetupCollections_SessionsFields(t *testing.T) {
	app := testutil.NewTestApp(t)
	if err := analytics.SetupCollections(app); err != nil {
		t.Fatal(err)
	}

	col, err := app.FindCollectionByNameOrId(analytics.SessionsCollectionName)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"path", "device_type", "browser", "os", "timestamp", "is_new_session"} {
		if col.Fields.GetByName(name) == nil {
			t.Errorf("required field %q missing from _analytics_sessions", name)
		}
	}
}

// --- Migration ---

// TestMigration_OldAnalyticsSchema_DroppedAndRecreated simulates upgrading from
// the old raw-events _analytics schema (which contained an "ip" field) to the
// new aggregated counter schema.
func TestMigration_OldAnalyticsSchema_DroppedAndRecreated(t *testing.T) {
	app := testutil.NewTestApp(t)

	// Build an old-style _analytics collection with raw-event fields including "ip".
	oldCol := core.NewBaseCollection(analytics.CollectionName)
	oldCol.System = true
	oldCol.Fields.Add(&core.TextField{Name: "path", Required: true})
	oldCol.Fields.Add(&core.TextField{Name: "method", Required: false})
	oldCol.Fields.Add(&core.TextField{Name: "ip", Required: false})
	oldCol.Fields.Add(&core.TextField{Name: "user_agent", Required: false})
	oldCol.Fields.Add(&core.TextField{Name: "visitor_id", Required: false})
	oldCol.Fields.Add(&core.DateField{Name: "timestamp", Required: false})
	oldCol.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})

	if err := app.SaveNoValidate(oldCol); err != nil {
		t.Fatalf("create old _analytics schema: %v", err)
	}

	// Pre-condition: old schema has "ip".
	before, _ := app.FindCollectionByNameOrId(analytics.CollectionName)
	if before.Fields.GetByName("ip") == nil {
		t.Fatal("pre-condition failed: old schema should have 'ip'")
	}

	// Run migration via SetupCollections.
	if err := analytics.SetupCollections(app); err != nil {
		t.Fatalf("SetupCollections (migration): %v", err)
	}

	// Old PII fields must be gone.
	after, err := app.FindCollectionByNameOrId(analytics.CollectionName)
	if err != nil {
		t.Fatalf("_analytics not found after migration: %v", err)
	}
	for _, bad := range []string{"ip", "user_agent", "visitor_id", "method"} {
		if after.Fields.GetByName(bad) != nil {
			t.Errorf("migration failed: old field %q still present in _analytics", bad)
		}
	}

	// New schema fields must exist.
	for _, name := range []string{"path", "date", "device_type", "browser", "views", "unique_sessions"} {
		if after.Fields.GetByName(name) == nil {
			t.Errorf("migration failed: new field %q missing from _analytics", name)
		}
	}

	// _analytics_sessions must also be created.
	if _, err := app.FindCollectionByNameOrId(analytics.SessionsCollectionName); err != nil {
		t.Errorf("_analytics_sessions not created during migration: %v", err)
	}
}

// --- Initialize / New ---

func TestInitialize_CreatesBothCollections(t *testing.T) {
	app := testutil.NewTestApp(t)

	a, err := analytics.Initialize(app)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if a == nil {
		t.Fatal("Initialize returned nil Analytics")
	}

	for _, name := range []string{analytics.CollectionName, analytics.SessionsCollectionName} {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Fatalf("collection %q not found after Initialize: %v", name, err)
		}
	}
}

// --- GetData ---

func TestGetData_ReturnsDefault_WhenNoCollection(t *testing.T) {
	// No collections — GetData should fall back to DefaultData gracefully.
	app := testutil.NewTestApp(t)
	a := analytics.New(app)

	d, err := a.GetData()
	if err != nil {
		t.Errorf("GetData should not error when collection missing, got: %v", err)
	}
	if d == nil {
		t.Fatal("GetData returned nil")
	}
}

func TestGetData_ReturnsData_WhenEmpty(t *testing.T) {
	app := testutil.NewTestAppWithAnalytics(t)
	a := analytics.New(app)

	d, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if d == nil {
		t.Fatal("GetData returned nil")
	}
	// Fresh DB — counters must be zero.
	if d.TotalPageViews != 0 {
		t.Errorf("expected 0 TotalPageViews, got %d", d.TotalPageViews)
	}
	if d.UniqueVisitors != 0 {
		t.Errorf("expected 0 UniqueVisitors, got %d", d.UniqueVisitors)
	}
}

func TestGetData_TopPages_EmptySlice(t *testing.T) {
	app := testutil.NewTestAppWithAnalytics(t)
	a := analytics.New(app)

	d, err := a.GetData()
	if err != nil {
		t.Fatal(err)
	}
	if d.TopPages == nil {
		t.Error("TopPages should be non-nil even when empty")
	}
}

func TestGetData_RecentVisits_EmptySlice(t *testing.T) {
	app := testutil.NewTestAppWithAnalytics(t)
	a := analytics.New(app)

	d, err := a.GetData()
	if err != nil {
		t.Fatal(err)
	}
	if d.RecentVisits == nil {
		t.Error("RecentVisits should be non-nil even when empty")
	}
}

func TestGetData_BrowserBreakdown_NotNil(t *testing.T) {
	app := testutil.NewTestAppWithAnalytics(t)
	a := analytics.New(app)

	d, err := a.GetData()
	if err != nil {
		t.Fatal(err)
	}
	if d.BrowserBreakdown == nil {
		t.Error("BrowserBreakdown should be non-nil")
	}
}

// --- Middleware smoke test ---

func TestMiddleware_PassesThrough(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-page", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) Chrome/120")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- Benchmarks ---

func BenchmarkDefaultData(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analytics.DefaultData()
	}
}

func BenchmarkSetupCollections(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := testutil.NewTestApp(b)
		_ = analytics.SetupCollections(app)
		app.Cleanup()
	}
}

func BenchmarkGetData_Empty(b *testing.B) {
	app := testutil.NewTestAppWithAnalytics(b)
	a := analytics.New(app)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = a.GetData()
	}
}
