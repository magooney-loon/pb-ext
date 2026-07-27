package analytics_test

import (
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const uaChrome = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36"

// neverFlush keeps the background worker out of the way so tests can observe
// buffering and flush explicitly.
func neverFlush() analytics.Option { return analytics.WithFlushInterval(time.Hour) }

func trackVisit(a *analytics.Analytics, ip, path string) {
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("User-Agent", uaChrome)
	a.Track(ip, r)
}

// --- schema setup & migrations ---

func TestMigration_CreatesCounterTable(t *testing.T) {
	// NewTestApp runs RunAllMigrations, which applies pb-ext's registered
	// migrations alongside PocketBase's own.
	app := testutil.NewTestApp(t)

	if !app.AuxHasTable(analytics.TableName) {
		t.Fatalf("table %s not found in auxiliary.db", analytics.TableName)
	}

	var columns []string
	err := app.AuxDB().NewQuery(
		"SELECT name FROM PRAGMA_TABLE_INFO({:t})",
	).Bind(dbx.Params{"t": analytics.TableName}).Column(&columns)
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}

	got := map[string]bool{}
	for _, c := range columns {
		got[c] = true
	}
	for _, column := range []string{"path", "date", "device_type", "browser", "views", "unique_sessions", "returning_sessions", "created", "updated"} {
		if !got[column] {
			t.Errorf("missing column %q", column)
		}
	}
}

// TestMigration_LeavesNothingInDataDB pins the move: the counters must not be a
// data.db collection any more, or flushes would be back on the application's
// single writer connection.
func TestMigration_LeavesNothingInDataDB(t *testing.T) {
	app := testutil.NewTestApp(t)

	if _, err := app.FindCollectionByNameOrId(analytics.TableName); err == nil {
		t.Error("_analytics still exists as a data.db collection")
	}
	if app.HasTable(analytics.TableName) {
		t.Error("_analytics table still exists in data.db")
	}
}

// indexNames returns the names of the indexes SQLite actually created for the
// _analytics table, which is what the query planner can use.
func indexNames(t *testing.T, app core.App) map[string]bool {
	t.Helper()

	var names []string
	err := app.AuxDB().NewQuery(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name={:t}",
	).Bind(dbx.Params{"t": analytics.TableName}).Column(&names)
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}

	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return set
}

var wantIndexes = []string{
	"idx_analytics_upsert",
	"idx_analytics_totals",
	"idx_analytics_pages",
	"idx_analytics_devices",
	"idx_analytics_browsers",
}

func TestMigration_CreatesDashboardIndexes(t *testing.T) {
	app := testutil.NewTestApp(t)

	got := indexNames(t, app)
	for _, name := range wantIndexes {
		if !got[name] {
			t.Errorf("missing index %q", name)
		}
	}
}

// TestGetData_QueriesUseCoveringIndexes guards the dashboard against silently
// regressing into per-row table lookups as the table grows.
func TestGetData_QueriesUseCoveringIndexes(t *testing.T) {
	app := testutil.NewTestApp(t)

	cutoff := time.Now().AddDate(0, 0, -analytics.LookbackDays).Format("2006-01-02")
	queries := map[string]string{
		"totals":   "SELECT SUM(views), SUM(unique_sessions), SUM(returning_sessions) FROM _analytics WHERE date >= {:c}",
		"devices":  "SELECT device_type, SUM(views) FROM _analytics WHERE date >= {:c} GROUP BY device_type",
		"browsers": "SELECT browser, SUM(views) v FROM _analytics WHERE date >= {:c} GROUP BY browser ORDER BY v DESC LIMIT 5",
		"pages":    "SELECT path, SUM(views) v FROM _analytics WHERE date >= {:c} GROUP BY path ORDER BY v DESC LIMIT 10",
	}

	for name, q := range queries {
		type planRow struct {
			ID      int    `db:"id"`
			Parent  int    `db:"parent"`
			Notused int    `db:"notused"`
			Detail  string `db:"detail"`
		}
		var rows []planRow
		if err := app.AuxDB().NewQuery("EXPLAIN QUERY PLAN " + q).
			Bind(dbx.Params{"c": cutoff}).All(&rows); err != nil {
			t.Fatalf("explain %s: %v", name, err)
		}

		covering := false
		for _, r := range rows {
			if strings.Contains(r.Detail, "COVERING INDEX") {
				covering = true
			}
		}
		if !covering {
			for _, r := range rows {
				t.Logf("[%s] %s", name, r.Detail)
			}
			t.Errorf("%s query does not use a covering index", name)
		}
	}
}

func TestMigration_IsRecordedInHistory(t *testing.T) {
	app := testutil.NewTestApp(t)

	var applied int
	err := app.DB().NewQuery(
		"SELECT COUNT(*) FROM _migrations WHERE file = {:f}",
	).Bind(dbx.Params{"f": analytics.MigrationFile}).Row(&applied)
	if err != nil {
		t.Fatalf("query _migrations: %v", err)
	}
	if applied != 1 {
		t.Fatalf("migration %q recorded %d times, want 1", analytics.MigrationFile, applied)
	}
}

// TestMigrationFile_IsNamespaced guards the property that keeps pb-ext from
// colliding with an app's own migrations: PocketBase keys applied migrations on
// the base file name alone, so the name must be both timestamped (for ordering)
// and namespaced (for uniqueness).
func TestMigrationFile_IsNamespaced(t *testing.T) {
	if !strings.Contains(analytics.MigrationFile, "_pbext_") {
		t.Errorf("MigrationFile %q must contain _pbext_ to avoid colliding with app migrations", analytics.MigrationFile)
	}
	ts, _, found := strings.Cut(analytics.MigrationFile, "_")
	if !found {
		t.Fatalf("MigrationFile %q has no timestamp prefix", analytics.MigrationFile)
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		t.Errorf("MigrationFile %q must start with a unix timestamp: %v", analytics.MigrationFile, err)
	}
}

// TestMigrationFile_SortsAfterPocketBaseSystemMigrations keeps pb-ext's
// migrations ordered after PocketBase's own in the combined list, and before
// anything an app generates today.
func TestMigrationFile_SortsAfterPocketBaseSystemMigrations(t *testing.T) {
	ts, _, _ := strings.Cut(analytics.MigrationFile, "_")
	stamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	// Newest PocketBase system migration as of v0.39.
	const newestSystemMigration = 1778828400
	if stamp <= newestSystemMigration {
		t.Errorf("timestamp %d must be greater than PocketBase's newest system migration %d",
			stamp, newestSystemMigration)
	}
	if stamp >= time.Now().Unix() {
		t.Errorf("timestamp %d must be in the past so app migrations sort after it", stamp)
	}
}

// runMigration executes one direction of a migration the way MigrationsRunner
// does: an auxiliary transaction wrapped around a data one, so the migration can
// touch either database.
func runMigration(t *testing.T, app core.App, direction func(core.App) error) error {
	t.Helper()
	return app.AuxRunInTransaction(func(txApp core.App) error {
		return txApp.RunInTransaction(direction)
	})
}

// TestMigration_DownRemovesTable exercises the down path, which is what
// `pocketbase migrate down` would run.
func TestMigration_DownRemovesTable(t *testing.T) {
	app := testutil.NewTestApp(t)

	migration := findMigration(t, analytics.MigrationFile)

	if err := runMigration(t, app, migration.Down); err != nil {
		t.Fatalf("migration down: %v", err)
	}
	if app.AuxHasTable(analytics.TableName) {
		t.Fatal("table still exists after migration down")
	}

	// Down must be safe to run when the table is already gone.
	if err := runMigration(t, app, migration.Down); err != nil {
		t.Fatalf("second migration down: %v", err)
	}

	// And up must restore it, indexes included.
	if err := runMigration(t, app, migration.Up); err != nil {
		t.Fatalf("migration up after down: %v", err)
	}
	got := indexNames(t, app)
	for _, name := range wantIndexes {
		if !got[name] {
			t.Errorf("index %q missing after down/up round trip", name)
		}
	}
}

// findMigration locates a registered migration by its file name.
func findMigration(t *testing.T, file string) *core.Migration {
	t.Helper()
	for _, m := range core.AppMigrations.Items() {
		if m.File == file {
			return m
		}
	}
	t.Fatalf("migration %q is not registered in core.AppMigrations", file)
	return nil
}

// TestInitialize_FailsWithoutMigration verifies the collector reports a clear
// error instead of silently collecting into a missing table.
func TestInitialize_FailsWithoutMigration(t *testing.T) {
	app := testutil.NewTestApp(t)

	migration := findMigration(t, analytics.MigrationFile)
	if err := runMigration(t, app, migration.Down); err != nil {
		t.Fatalf("migration down: %v", err)
	}

	_, err := analytics.Initialize(app)
	if err == nil {
		t.Fatal("Initialize succeeded without the _analytics table")
	}
	if !strings.Contains(err.Error(), analytics.MigrationFile) {
		t.Errorf("error %q should name the missing migration %q", err, analytics.MigrationFile)
	}
}

// --- request path does no database work ---

func TestTrack_BuffersWithoutWriting(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush())

	for i := 0; i < 500; i++ {
		trackVisit(a, "1.2.3.4", "/pricing")
	}

	if rows, _, _, _ := testutil.AnalyticsTotals(t, app); rows != 0 {
		t.Fatalf("%d rows written during tracking, want 0 (request path must not touch the database)", rows)
	}
	if got := a.PendingCounters(); got != 1 {
		t.Fatalf("PendingCounters = %d, want 1", got)
	}
}

func TestFlush_PersistsAggregatedCounters(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush())

	for i := 0; i < 250; i++ {
		trackVisit(a, "1.2.3.4", "/pricing")
	}
	for i := 0; i < 100; i++ {
		trackVisit(a, "1.2.3.4", "/docs")
	}

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	rows, views, newSessions, _ := testutil.AnalyticsTotals(t, app)
	if rows != 2 {
		t.Fatalf("rows = %d, want 2 (one per path)", rows)
	}
	if views != 350 {
		t.Fatalf("views = %d, want 350", views)
	}
	if newSessions != 1 {
		t.Fatalf("unique_sessions = %d, want 1 (single visitor, single session)", newSessions)
	}
	if got := a.PendingCounters(); got != 0 {
		t.Fatalf("PendingCounters after flush = %d, want 0", got)
	}
}

func TestFlush_AccumulatesAcrossFlushes(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush())

	for round := 0; round < 5; round++ {
		for i := 0; i < 10; i++ {
			trackVisit(a, "1.2.3.4", "/pricing")
		}
		if err := a.Flush(); err != nil {
			t.Fatalf("Flush round %d: %v", round, err)
		}
	}

	rows, views, _, _ := testutil.AnalyticsTotals(t, app)
	if rows != 1 {
		t.Fatalf("rows = %d, want 1 (upsert must not duplicate)", rows)
	}
	if views != 50 {
		t.Fatalf("views = %d, want 50", views)
	}
}

func TestFlush_NoopWhenEmpty(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush())

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush on empty: %v", err)
	}
	if rows, _, _, _ := testutil.AnalyticsTotals(t, app); rows != 0 {
		t.Fatalf("rows = %d, want 0", rows)
	}
}

// TestFlush_DoesNotWaitOnTheDataDBWriter is the regression test for the reason
// counters live in auxiliary.db at all.
//
// data.db's nonconcurrent pool is capped at one connection, so an open
// application transaction owns the only writer. While counters were a data.db
// collection, a flush landing in that window queued behind it — and the request
// path kept accumulating. Writing to auxiliary.db, which has its own writer and
// its own WAL, makes the two independent.
func TestFlush_DoesNotWaitOnTheDataDBWriter(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush())

	for i := 0; i < 50; i++ {
		trackVisit(a, "1.2.3.4", "/pricing")
	}

	held := make(chan struct{})
	release := make(chan struct{})
	txDone := make(chan error, 1)

	go func() {
		txDone <- app.RunInTransaction(func(txApp core.App) error {
			// A write is what actually takes the lock; BEGIN alone is deferred.
			if _, err := txApp.NonconcurrentDB().
				NewQuery("CREATE TABLE writer_lock_probe (x)").Execute(); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()

	<-held

	flushed := make(chan error, 1)
	go func() { flushed <- a.Flush() }()

	select {
	case err := <-flushed:
		if err != nil {
			t.Fatalf("Flush while data.db writer is held: %v", err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("Flush blocked on the data.db writer — counters must be written to auxiliary.db")
	}

	close(release)
	if err := <-txDone; err != nil {
		t.Fatalf("holding transaction: %v", err)
	}

	if rows, views, _, _ := testutil.AnalyticsTotals(t, app); rows != 1 || views != 50 {
		t.Fatalf("rows = %d, views = %d; want 1 and 50", rows, views)
	}
}

func TestFlush_ChunksLargeBatches(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithMaxDistinctPaths(1000))

	// Comfortably more than one INSERT chunk.
	const paths = 750
	for i := 0; i < paths; i++ {
		trackVisit(a, "1.2.3.4", fmt.Sprintf("/p%d", i))
	}

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	rows, views, _, _ := testutil.AnalyticsTotals(t, app)
	if rows != paths {
		t.Fatalf("rows = %d, want %d", rows, paths)
	}
	if views != paths {
		t.Fatalf("views = %d, want %d", views, paths)
	}
}

func TestBackgroundWorkerFlushes(t *testing.T) {
	app, a := testutil.NewAnalytics(t, analytics.WithFlushInterval(25*time.Millisecond))

	trackVisit(a, "1.2.3.4", "/pricing")

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rows, _, _, _ := testutil.AnalyticsTotals(t, app); rows == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background worker did not flush within 5s")
}

func TestEarlyFlushTriggeredAtCeiling(t *testing.T) {
	app, a := testutil.NewAnalytics(t,
		neverFlush(),
		analytics.WithMaxPendingCounters(10),
		analytics.WithMaxDistinctPaths(1000),
	)

	for i := 0; i < 50; i++ {
		trackVisit(a, "1.2.3.4", fmt.Sprintf("/p%d", i))
	}

	// The ceiling signals the worker, which flushes even though the timer is an
	// hour away.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rows, _, _, _ := testutil.AnalyticsTotals(t, app); rows > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("pending ceiling did not trigger an early flush")
}

func TestClose_FlushesPending(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush())

	for i := 0; i < 20; i++ {
		trackVisit(a, "1.2.3.4", "/pricing")
	}

	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, views, _, _ := testutil.AnalyticsTotals(t, app)
	if views != 20 {
		t.Fatalf("views after Close = %d, want 20 (shutdown must not lose counts)", views)
	}

	// Close is idempotent — testutil cleanup will call it again.
	if err := a.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// --- session classification ---

func TestSessionClassification_NewVisitorsAndContinuedViews(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithSessionWindow(time.Hour))

	// Three distinct visitors, each viewing two pages.
	for v := 0; v < 3; v++ {
		ip := fmt.Sprintf("10.0.0.%d", v)
		trackVisit(a, ip, "/")
		trackVisit(a, ip, "/pricing")
	}

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	_, views, newSessions, returning := testutil.AnalyticsTotals(t, app)
	if views != 6 {
		t.Fatalf("views = %d, want 6", views)
	}
	if newSessions != 3 {
		t.Fatalf("unique_sessions = %d, want 3 (one per visitor)", newSessions)
	}
	if returning != 0 {
		t.Fatalf("returning_sessions = %d, want 0", returning)
	}
}

func TestSessionClassification_LapsedSessionCountsAsReturning(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithSessionWindow(50*time.Millisecond))

	trackVisit(a, "1.2.3.4", "/")
	time.Sleep(120 * time.Millisecond) // outlive the session window
	trackVisit(a, "1.2.3.4", "/")

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	_, views, newSessions, returning := testutil.AnalyticsTotals(t, app)
	if views != 2 {
		t.Fatalf("views = %d, want 2", views)
	}
	if newSessions != 1 {
		t.Fatalf("unique_sessions = %d, want 1", newSessions)
	}
	if returning != 1 {
		t.Fatalf("returning_sessions = %d, want 1", returning)
	}
}

// --- cardinality and memory bounds ---

func TestJunkURLFloodDoesNotGrowTable(t *testing.T) {
	const maxPaths = 200
	app, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithMaxDistinctPaths(maxPaths))

	const flood = 25000
	for i := 0; i < flood; i++ {
		trackVisit(a, "1.2.3.4", fmt.Sprintf("/does-not-exist-%d", i))
	}
	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	rows, views, _, _ := testutil.AnalyticsTotals(t, app)
	if rows > maxPaths+1 {
		t.Fatalf("%d junk requests created %d rows, want <= %d", flood, rows, maxPaths+1)
	}
	if views != flood {
		t.Fatalf("views = %d, want %d (bounding paths must not lose views)", views, flood)
	}
	t.Logf("%d junk URLs -> %d rows, %d views retained", flood, rows, views)
}

func TestForgedVisitorFloodStaysWithinMemoryBound(t *testing.T) {
	_, a := testutil.NewAnalytics(t,
		neverFlush(),
		analytics.WithVisitorMemory(3, 1000),
		analytics.WithMaxDistinctPaths(10),
	)

	// Every request appears to come from a different client.
	for i := 0; i < 100000; i++ {
		trackVisit(a, fmt.Sprintf("203.0.113.%d.%d", i%255, i/255), "/")
	}

	current, max := a.VisitorMemory()
	if max != 3000 {
		t.Fatalf("VisitorMemory max = %d, want 3000", max)
	}
	if current > max {
		t.Fatalf("tracked visitors = %d, exceeds bound %d", current, max)
	}
	t.Logf("100000 forged clients -> %d entries retained (cap %d)", current, max)
}

// --- dashboard aggregation ---

func TestGetData_EmptyCollection(t *testing.T) {
	_, a := testutil.NewAnalytics(t, neverFlush())

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}

	if data.TotalPageViews != 0 || data.UniqueVisitors != 0 {
		t.Errorf("expected zeroed totals, got views=%d visitors=%d", data.TotalPageViews, data.UniqueVisitors)
	}
	if data.TopPages == nil || len(data.TopPages) != 0 {
		t.Errorf("TopPages = %v, want empty non-nil slice", data.TopPages)
	}
	if data.RecentVisits == nil || len(data.RecentVisits) != 0 {
		t.Errorf("RecentVisits = %v, want empty non-nil slice", data.RecentVisits)
	}
	if data.BrowserBreakdown == nil {
		t.Error("BrowserBreakdown must not be nil")
	}
}

func TestGetData_MissingCollectionFallsBackToDefaults(t *testing.T) {
	app := testutil.NewTestApp(t)
	a := analytics.New(app)

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if data.TotalPageViews != 0 {
		t.Errorf("TotalPageViews = %d, want 0", data.TotalPageViews)
	}
	if data.TopPages == nil || data.RecentVisits == nil {
		t.Error("fallback payload must have non-nil slices")
	}
}

func TestGetData_ReflectsTrackedTraffic(t *testing.T) {
	_, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithCacheTTL(0))

	// /pricing is the most popular page; three distinct visitors.
	for v := 0; v < 3; v++ {
		ip := fmt.Sprintf("10.0.0.%d", v)
		for i := 0; i < 4; i++ {
			trackVisit(a, ip, "/pricing")
		}
		trackVisit(a, ip, "/docs")
	}

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}

	if data.TotalPageViews != 15 {
		t.Errorf("TotalPageViews = %d, want 15", data.TotalPageViews)
	}
	if data.UniqueVisitors != 3 {
		t.Errorf("UniqueVisitors = %d, want 3", data.UniqueVisitors)
	}
	if data.NewVisitors != 3 {
		t.Errorf("NewVisitors = %d, want 3", data.NewVisitors)
	}
	if data.TodayPageViews != 15 {
		t.Errorf("TodayPageViews = %d, want 15", data.TodayPageViews)
	}
	if data.ViewsPerVisitor != 5 {
		t.Errorf("ViewsPerVisitor = %v, want 5", data.ViewsPerVisitor)
	}
	if len(data.TopPages) != 2 || data.TopPages[0].Path != "/pricing" || data.TopPages[0].Views != 12 {
		t.Errorf("TopPages = %+v, want /pricing with 12 views first", data.TopPages)
	}
	if data.TopDeviceType != "desktop" || data.DesktopPercentage != 100 {
		t.Errorf("device breakdown = %s/%v, want desktop/100", data.TopDeviceType, data.DesktopPercentage)
	}
	if data.TopBrowser != "chrome" {
		t.Errorf("TopBrowser = %q, want chrome", data.TopBrowser)
	}
	if data.BrowserBreakdown["chrome"] != 100 {
		t.Errorf("BrowserBreakdown[chrome] = %v, want 100", data.BrowserBreakdown["chrome"])
	}
}

func TestGetData_NewVersusReturningAreOnTheSameScale(t *testing.T) {
	_, a := testutil.NewAnalytics(t, neverFlush(),
		analytics.WithCacheTTL(0),
		analytics.WithSessionWindow(50*time.Millisecond),
	)

	// Far more page views than the old 50-row ring could represent.
	for v := 0; v < 20; v++ {
		ip := fmt.Sprintf("10.0.1.%d", v)
		for i := 0; i < 30; i++ {
			trackVisit(a, ip, "/")
		}
	}
	time.Sleep(120 * time.Millisecond)
	for v := 0; v < 5; v++ {
		trackVisit(a, fmt.Sprintf("10.0.1.%d", v), "/")
	}

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}

	if data.NewVisitors != 20 {
		t.Errorf("NewVisitors = %d, want 20", data.NewVisitors)
	}
	if data.ReturningVisitors != 5 {
		t.Errorf("ReturningVisitors = %d, want 5", data.ReturningVisitors)
	}
	if data.UniqueVisitors != data.NewVisitors+data.ReturningVisitors {
		t.Errorf("UniqueVisitors = %d, want NewVisitors+ReturningVisitors = %d",
			data.UniqueVisitors, data.NewVisitors+data.ReturningVisitors)
	}
	if data.TotalPageViews != 605 {
		t.Errorf("TotalPageViews = %d, want 605", data.TotalPageViews)
	}
}

func TestGetData_HourlyActivityIsNotCappedByRingSize(t *testing.T) {
	_, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithCacheTTL(0))

	// The old implementation derived this from a 50-row table, so it could
	// never report more than 50 visits in the trailing hour.
	const visits = analytics.SessionRingSize * 20
	for i := 0; i < visits; i++ {
		trackVisit(a, fmt.Sprintf("10.0.2.%d", i%255), "/")
	}

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}

	if data.RecentVisitCount != visits {
		t.Fatalf("RecentVisitCount = %d, want %d", data.RecentVisitCount, visits)
	}
	if data.HourlyActivityPercentage <= 0 || data.HourlyActivityPercentage > 100 {
		t.Fatalf("HourlyActivityPercentage = %v, want (0, 100]", data.HourlyActivityPercentage)
	}
	if len(data.RecentVisits) != analytics.SessionRingSize {
		t.Fatalf("RecentVisits = %d, want %d", len(data.RecentVisits), analytics.SessionRingSize)
	}
}

func TestGetData_ExcludesRowsOutsideLookbackWindow(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithCacheTTL(0))

	seed := func(date string, views int) {
		_, err := app.AuxDB().NewQuery(
			"INSERT INTO " + analytics.TableName + ` (path, date, device_type, browser, views, unique_sessions, returning_sessions)
			 VALUES ({:path}, {:date}, 'desktop', 'chrome', {:views}, 1, 0)`,
		).Bind(dbx.Params{"path": "/old-" + date, "date": date, "views": views}).Execute()
		if err != nil {
			t.Fatalf("seed %s: %v", date, err)
		}
	}

	inside := time.Now().AddDate(0, 0, -(analytics.LookbackDays - 1)).Format("2006-01-02")
	outside := time.Now().AddDate(0, 0, -(analytics.LookbackDays + 10)).Format("2006-01-02")
	seed(inside, 100)
	seed(outside, 900)

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if data.TotalPageViews != 100 {
		t.Fatalf("TotalPageViews = %d, want 100 (rows older than LookbackDays must be excluded)", data.TotalPageViews)
	}
}

func TestGetData_CachesWithinTTL(t *testing.T) {
	_, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithCacheTTL(time.Minute))

	trackVisit(a, "1.2.3.4", "/")
	first, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if first.TotalPageViews != 1 {
		t.Fatalf("first TotalPageViews = %d, want 1", first.TotalPageViews)
	}

	// New traffic stays buffered; the cached aggregate is reused.
	trackVisit(a, "1.2.3.4", "/")
	second, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if second.TotalPageViews != 1 {
		t.Fatalf("second TotalPageViews = %d, want 1 (cache should still be warm)", second.TotalPageViews)
	}

	// Live fields bypass the cache.
	if second.RecentVisitCount != 2 {
		t.Fatalf("RecentVisitCount = %d, want 2 (live fields must not be cached)", second.RecentVisitCount)
	}
}

func TestGetData_CacheInvalidatedByFlush(t *testing.T) {
	_, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithCacheTTL(time.Minute))

	trackVisit(a, "1.2.3.4", "/")
	if _, err := a.GetData(); err != nil {
		t.Fatalf("GetData: %v", err)
	}

	trackVisit(a, "1.2.3.4", "/")
	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	data, err := a.GetData()
	if err != nil {
		t.Fatalf("GetData: %v", err)
	}
	if data.TotalPageViews != 2 {
		t.Fatalf("TotalPageViews = %d, want 2 (flush must invalidate the cache)", data.TotalPageViews)
	}
}

// --- concurrency ---

func TestConcurrentTrackingLosesNoViews(t *testing.T) {
	app, a := testutil.NewAnalytics(t, neverFlush(), analytics.WithMaxDistinctPaths(10000))

	const goroutines, perGoroutine = 32, 500
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				trackVisit(a, fmt.Sprintf("10.1.%d.%d", g, i%255), fmt.Sprintf("/p%d", i%50))
			}
		}(g)
	}
	wg.Wait()

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	_, views, _, _ := testutil.AnalyticsTotals(t, app)
	want := goroutines * perGoroutine
	if views != want {
		t.Fatalf("views = %d, want %d", views, want)
	}
}

func TestConcurrentTrackingWhileReadingDashboard(t *testing.T) {
	_, a := testutil.NewAnalytics(t, analytics.WithFlushInterval(10*time.Millisecond))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				trackVisit(a, fmt.Sprintf("10.2.%d.%d", g, i%255), fmt.Sprintf("/p%d", i%20))
			}
		}(g)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := a.GetData(); err != nil {
				t.Errorf("GetData during traffic: %v", err)
				return
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}
