// Package audit_test holds the tests that need a real database.
//
// They cannot live in package audit: they use core/testutil, which imports
// core/jobs, which imports core/audit — an import cycle. Same split as
// core/analytics and core/alerts, for the same reason.
package audit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/magooney-loon/pb-ext/core/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func newAuditor(t testing.TB, opts ...audit.Option) (*tests.TestApp, *audit.Auditor, *testutil.AlertCapture) {
	t.Helper()

	app := testutil.NewTestApp(t)
	capture := testutil.NewAlertCapture()

	notifier := alerts.Initialize(app,
		alerts.WithTransport(capture),
		alerts.WithEnabled(true),
		alerts.WithMinSendInterval(0),
		alerts.WithLifecycleAlerts(false),
		alerts.WithPersistence(false),
		alerts.WithEvaluateInterval(time.Hour),
	)
	t.Cleanup(func() { _ = notifier.Close() })

	defaults := []audit.Option{
		audit.WithNotifier(notifier),
		// Long interval: these tests flush by hand so they observe exactly the
		// batch they created.
		audit.WithFlushInterval(time.Hour),
	}

	a := audit.Initialize(app, append(defaults, opts...)...)
	t.Cleanup(func() { _ = a.Close() })

	return app, a, capture
}

func TestMigration_CreatesTheAuxTable(t *testing.T) {
	app := testutil.NewTestApp(t)

	if !app.AuxHasTable(audit.TableName) {
		t.Fatalf("%s was not created in auxiliary.db", audit.TableName)
	}
	// It must not share the application's writer connection: this table is
	// written precisely when the server is being scanned.
	if app.HasTable(audit.TableName) {
		t.Fatalf("%s was created in data.db; it belongs in auxiliary.db", audit.TableName)
	}

	var found bool
	for _, m := range core.AppMigrations.Items() {
		if m.File == audit.MigrationFile {
			found = true
			if m.ReapplyCondition == nil {
				t.Error("the audit migration has no ReapplyCondition; deleting auxiliary.db would leave history claiming it is applied")
			}
		}
	}
	if !found {
		t.Fatalf("migration %q is not registered", audit.MigrationFile)
	}
}

func TestFlush_WritesAggregatedRows(t *testing.T) {
	_, a, _ := newAuditor(t)

	for range 12 {
		a.Track(audit.Event{
			At: time.Now(), Kind: audit.KindAdminUI, Method: "GET", Path: "/_/",
			Status: 200, Outcome: audit.OutcomeAllowed, AuthState: audit.AuthAnonymous,
			IP: "203.0.113.7", UserAgent: "curl/8.0",
		})
	}

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	records := a.Recent(10)
	if len(records) != 1 {
		t.Fatalf("Recent returned %d rows, want the repeats collapsed into 1", len(records))
	}
	if records[0].Count != 12 {
		t.Fatalf("Count = %d, want 12", records[0].Count)
	}
	if records[0].IP != "203.0.113.7" {
		t.Fatalf("IP = %q", records[0].IP)
	}
	if records[0].Created.IsZero() || records[0].LastSeen.IsZero() {
		t.Error("first/last seen were not persisted")
	}
}

func TestFlush_RaisesAnAlertOnAFailedLogin(t *testing.T) {
	_, a, capture := newAuditor(t)

	a.Track(audit.Event{
		At: time.Now(), Kind: audit.KindAuthFailure, Method: "POST",
		Path:   "/api/collections/_superusers/auth-with-password",
		Status: 400, Outcome: audit.OutcomeDenied, AuthState: audit.AuthAnonymous,
		Identity: "admin@example.com", IP: "203.0.113.7",
	})
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	msg := capture.Wait(t)
	if !strings.Contains(msg.Title, "Failed superuser login") {
		t.Fatalf("title = %q", msg.Title)
	}
	// The targeted account is the field that exists nowhere else.
	if msg.Fields["account"] != "admin@example.com" {
		t.Fatalf("fields = %v, want the targeted account", msg.Fields)
	}
	if msg.Fields["source"] != "203.0.113.7" {
		t.Fatalf("fields = %v, want the source address", msg.Fields)
	}
}

func TestFlush_EscalatesRepeatedFailuresToBruteForce(t *testing.T) {
	_, a, capture := newAuditor(t, audit.WithBruteForceAlert(5, 10*time.Minute))

	// Below the threshold: a warning, not an escalation.
	for i := range 3 {
		a.Track(audit.Event{
			At: time.Now(), Kind: audit.KindAuthFailure, Path: "/api/collections/_superusers/auth-with-password",
			Outcome: audit.OutcomeDenied, AuthState: audit.AuthAnonymous,
			Identity: "admin@example.com", IP: "203.0.113.9", Status: 400 + i%1,
		})
	}
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}
	if msg := capture.Wait(t); strings.Contains(msg.Title, "Repeated") {
		t.Fatalf("escalated at 3 failures, below the threshold of 5: %q", msg.Title)
	}

	// Crossing the threshold, counting what is already on record.
	for range 3 {
		a.Track(audit.Event{
			At: time.Now(), Kind: audit.KindAuthFailure, Path: "/api/collections/_superusers/auth-with-password",
			Outcome: audit.OutcomeDenied, AuthState: audit.AuthAnonymous,
			Identity: "root@example.com", IP: "203.0.113.9", Status: 400,
		})
	}
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	msg := capture.Wait(t)
	if !strings.Contains(msg.Title, "Repeated failed superuser logins") {
		t.Fatalf("title = %q, want a brute-force escalation", msg.Title)
	}
	if msg.Level != alerts.LevelCritical {
		t.Fatalf("level = %v, want critical", msg.Level)
	}
}

// The highest-signal event here: a failed login is usually scanner noise, a
// successful one from an unfamiliar address is the thing you built this for.
func TestFlush_AlertsOnASignInFromANewAddress(t *testing.T) {
	_, a, capture := newAuditor(t)

	success := func(ip string) audit.Event {
		return audit.Event{
			At: time.Now(), Kind: audit.KindAuthSuccess, Method: "POST",
			Path:   "/api/collections/_superusers/auth-with-password",
			Status: 200, Outcome: audit.OutcomeAllowed, AuthState: audit.AuthSuperuser,
			Identity: "admin@example.com", IP: ip,
		}
	}

	a.Track(success("203.0.113.20"))
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	msg := capture.Wait(t)
	if !strings.Contains(msg.Title, "new address") {
		t.Fatalf("title = %q, want a new-address alert", msg.Title)
	}

	// The same address again is now known, and must stay quiet.
	a.Track(success("203.0.113.20"))
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, m := range capture.Messages() {
		if strings.Contains(m.Title, "new address") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("alerted %d times for the same address, want exactly 1", count)
	}
}

func TestStats_SummarisesTheWindow(t *testing.T) {
	_, a, _ := newAuditor(t)

	a.Track(audit.Event{At: time.Now(), Kind: audit.KindAuthFailure, Outcome: audit.OutcomeDenied, IP: "203.0.113.1", Identity: "a@example.com"})
	a.Track(audit.Event{At: time.Now(), Kind: audit.KindAuthSuccess, Outcome: audit.OutcomeAllowed, IP: "203.0.113.2", Identity: "b@example.com"})
	a.Track(audit.Event{At: time.Now(), Kind: audit.KindDashboard, Outcome: audit.OutcomeDenied, IP: "203.0.113.3", Path: "/_/_"})
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	stats := a.Stats()
	if stats.AuthFailures != 1 {
		t.Errorf("AuthFailures = %d, want 1", stats.AuthFailures)
	}
	if stats.AuthSuccesses != 1 {
		t.Errorf("AuthSuccesses = %d, want 1", stats.AuthSuccesses)
	}
	if stats.DeniedAttempts != 2 {
		t.Errorf("DeniedAttempts = %d, want 2 (the failure and the anonymous dashboard hit)", stats.DeniedAttempts)
	}
	if stats.DistinctIPs != 3 {
		t.Errorf("DistinctIPs = %d, want 3", stats.DistinctIPs)
	}
	if stats.LastFailure.IsZero() || stats.LastSuccess.IsZero() {
		t.Error("last failure/success timestamps were not reported")
	}
}

func TestTopIPs_RanksFailuresFirst(t *testing.T) {
	_, a, _ := newAuditor(t)

	// Noisy but harmless.
	for i := range 20 {
		a.Track(audit.Event{At: time.Now(), Kind: audit.KindAdminUI, Path: "/_/", IP: "198.51.100.1", Status: 200 + i%1})
	}
	// Quiet but failing.
	for range 3 {
		a.Track(audit.Event{At: time.Now(), Kind: audit.KindAuthFailure, Outcome: audit.OutcomeDenied, IP: "203.0.113.66", Identity: "admin@example.com"})
	}
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	sources := a.TopIPs(10)
	if len(sources) < 2 {
		t.Fatalf("TopIPs returned %d sources", len(sources))
	}
	if sources[0].IP != "203.0.113.66" {
		t.Fatalf("first source = %q, want the failing one ranked above the merely noisy one", sources[0].IP)
	}
}

func TestPurge_DeletesPastRetention(t *testing.T) {
	app, a, _ := newAuditor(t, audit.WithRetentionDays(30))

	a.Track(audit.Event{At: time.Now(), Kind: audit.KindAdminUI, Path: "/_/", IP: "203.0.113.7"})
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}

	// Backdate a row past the window.
	if _, err := app.AuxNonconcurrentDB().NewQuery(
		"INSERT INTO " + audit.TableName + " (created, last_seen, kind, ip) VALUES ('2020-01-01 00:00:00.000Z', '2020-01-01 00:00:00.000Z', 'admin_ui', '198.51.100.9')",
	).Execute(); err != nil {
		t.Fatal(err)
	}

	deleted, err := a.Purge()
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("Purge deleted %d rows, want 1", deleted)
	}
	if got := len(a.Recent(10)); got != 1 {
		t.Fatalf("%d rows remain, want 1", got)
	}
}

// A security log that has quietly stopped recording is worse than one that was
// never enabled: the empty table reads as "nothing happened".
func TestInitialize_MissingTableIsLoudAndNotSilent(t *testing.T) {
	app := testutil.NewTestApp(t)
	capture := testutil.NewAlertCapture()

	notifier := alerts.Initialize(app,
		alerts.WithTransport(capture),
		alerts.WithEnabled(true),
		alerts.WithMinSendInterval(0),
		alerts.WithLifecycleAlerts(false),
		alerts.WithPersistence(false),
		alerts.WithEvaluateInterval(time.Hour),
	)
	t.Cleanup(func() { _ = notifier.Close() })

	if _, err := app.AuxDB().NewQuery("DROP TABLE " + audit.TableName).Execute(); err != nil {
		t.Fatal(err)
	}

	a := audit.Initialize(app, audit.WithNotifier(notifier))
	t.Cleanup(func() { _ = a.Close() })

	if a.Recording() {
		t.Fatal("the auditor claims to be recording with no table")
	}

	msg := capture.Wait(t)
	if msg.Level != alerts.LevelCritical {
		t.Fatalf("level = %v, want critical", msg.Level)
	}
	if !strings.Contains(msg.Title, "not recording") {
		t.Fatalf("title = %q", msg.Title)
	}
}

// The password submitted to a login attempt must never be read, logged or
// stored. The auth hook has it in hand — RecordAuthWithPasswordRequestEvent
// carries a Password field — so the guarantee is that nothing in the package
// touches it. A source scan proves that in a way a behavioural test cannot.
func TestPackage_NeverReadsTheSubmittedPassword(t *testing.T) {
	dir := packageDir(t)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing the audit package: %v", err)
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == "Password" {
					t.Errorf("%s reads a Password field at %s — the audit log must never see a submitted password",
						filepath.Base(name), fset.Position(sel.Pos()))
				}
				return true
			})
		}
	}
}

func packageDir(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
