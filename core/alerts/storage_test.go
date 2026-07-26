// Package alerts_test holds the tests that need a real database.
//
// They cannot live in package alerts: they use core/testutil, which imports
// core/jobs, which imports core/alerts — an import cycle. This is the same
// split core/analytics uses for the same reason.
package alerts_test

import (
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestMigration_CreatesTheAuxTable(t *testing.T) {
	app := testutil.NewTestApp(t)

	if !app.AuxHasTable(alerts.TableName) {
		t.Fatalf("%s was not created in auxiliary.db", alerts.TableName)
	}

	// It must be in auxiliary.db, not data.db. Sharing the application's single
	// writer connection is the thing this table exists to avoid: an alert about
	// a slow database must not queue behind the writes it is reporting on.
	if app.HasTable(alerts.TableName) {
		t.Fatalf("%s was created in data.db; it belongs in auxiliary.db", alerts.TableName)
	}
}

func TestMigration_IsRegisteredWithTheExpectedFileName(t *testing.T) {
	// PocketBase keys applied migrations on the base file name alone, so a
	// generic name would silently collide with an app's own migration.
	var found bool
	for _, m := range core.AppMigrations.Items() {
		if m.File == alerts.MigrationFile {
			found = true
			if m.ReapplyCondition == nil {
				t.Error("the alerts migration has no ReapplyCondition; deleting auxiliary.db would leave history claiming it is applied with no table to show for it")
			}
		}
	}
	if !found {
		t.Fatalf("migration %q is not registered in core.AppMigrations", alerts.MigrationFile)
	}
}

func TestPersist_WritesDeliveriesToTheLog(t *testing.T) {
	app, notifier, capture := testutil.NewAlerts(t)

	notifier.Send(alerts.Message{
		Level: alerts.LevelError,
		Key:   "job_failed:cleanup",
		Title: "Cron job failed",
		Text:  "collection not found",
	})
	capture.Wait(t)

	// The worker persists after handing the message to the transport.
	waitForRows(t, app, 1)

	records := notifier.Recent(10)
	if len(records) != 1 {
		t.Fatalf("Recent returned %d records, want 1", len(records))
	}

	got := records[0]
	if got.Title != "Cron job failed" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Level != "error" {
		t.Errorf("Level = %q, want error", got.Level)
	}
	if got.Status != alerts.StatusSent {
		t.Errorf("Status = %q, want %q", got.Status, alerts.StatusSent)
	}
	if got.Key != "job_failed:cleanup" {
		t.Errorf("Key = %q", got.Key)
	}
	if got.Created.IsZero() {
		t.Error("Created was not parsed back out of the log")
	}
}

func TestRecent_ReturnsNewestFirst(t *testing.T) {
	app, notifier, capture := testutil.NewAlerts(t)

	for _, title := range []string{"first", "second", "third"} {
		notifier.Send(alerts.Message{Title: title})
		capture.Wait(t)
	}
	waitForRows(t, app, 3)

	records := notifier.Recent(10)
	if len(records) != 3 {
		t.Fatalf("Recent returned %d records, want 3", len(records))
	}
	if records[0].Title != "third" {
		t.Fatalf("newest record = %q, want %q", records[0].Title, "third")
	}
}

func TestRecent_RespectsTheLimit(t *testing.T) {
	app, notifier, capture := testutil.NewAlerts(t)

	for i := range 5 {
		notifier.Send(alerts.Message{Title: string(rune('a' + i))})
		capture.Wait(t)
	}
	waitForRows(t, app, 5)

	if got := len(notifier.Recent(2)); got != 2 {
		t.Fatalf("Recent(2) returned %d records", got)
	}
}

// A stack trace can run to tens of kilobytes. The message that was sent is what
// the log is for; the full trace is already in the application log.
func TestPersist_TruncatesEnormousBodies(t *testing.T) {
	app, notifier, capture := testutil.NewAlerts(t)

	huge := ""
	for range 5000 {
		huge += "goroutine stack frame\n"
	}

	notifier.Send(alerts.Message{Title: "panic", Text: huge})
	capture.Wait(t)
	waitForRows(t, app, 1)

	stored := notifier.Recent(1)[0].Text
	if len([]rune(stored)) > 2100 {
		t.Fatalf("stored body is %d runes; it should have been truncated", len([]rune(stored)))
	}
}

func TestPurge_DeletesRecordsPastRetention(t *testing.T) {
	app, notifier, capture := testutil.NewAlerts(t, alerts.WithRetentionDays(30))

	notifier.Send(alerts.Message{Title: "recent"})
	capture.Wait(t)
	waitForRows(t, app, 1)

	// Backdate a second record past the retention window.
	old, err := types.ParseDateTime(time.Now().AddDate(0, 0, -45))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.AuxNonconcurrentDB().
		NewQuery("INSERT INTO " + alerts.TableName + " (created, level, title, status) VALUES ({:c}, 'info', 'ancient', 'sent')").
		Bind(dbx.Params{"c": old.String()}).
		Execute(); err != nil {
		t.Fatal(err)
	}

	deleted, err := notifier.Purge()
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("Purge deleted %d rows, want 1", deleted)
	}

	remaining := notifier.Recent(10)
	if len(remaining) != 1 || remaining[0].Title != "recent" {
		t.Fatalf("remaining records = %+v, want only the recent one", remaining)
	}
}

// Alerting is diagnostic machinery. A missing table must degrade to "no
// history", never to a failed startup.
func TestInitialize_SurvivesAMissingTable(t *testing.T) {
	app := testutil.NewTestApp(t)

	if _, err := app.AuxDB().NewQuery("DROP TABLE " + alerts.TableName).Execute(); err != nil {
		t.Fatal(err)
	}

	capture := testutil.NewAlertCapture()
	n := alerts.Initialize(app,
		alerts.WithTransport(capture),
		alerts.WithEnabled(true),
		alerts.WithMinSendInterval(0),
		alerts.WithLifecycleAlerts(false),
		alerts.WithEvaluateInterval(time.Hour),
	)
	t.Cleanup(func() { _ = n.Close() })

	if !n.Enabled() {
		t.Fatal("a missing history table disabled alerting entirely")
	}

	n.Send(alerts.Message{Title: "still delivered"})
	if got := capture.Wait(t); got.Title != "still delivered" {
		t.Fatalf("delivered %q", got.Title)
	}
	if got := n.Recent(10); len(got) != 0 {
		t.Fatalf("Recent returned %d records with no table", len(got))
	}
}

func waitForRows(t testing.TB, app core.App, want int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := app.AuxDB().Select("COUNT(*)").From(alerts.TableName).Row(&count); err == nil && count >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d rows in %s", want, alerts.TableName)
}
