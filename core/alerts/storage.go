package alerts

import (
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

// maxStoredTextRunes caps the body kept in the delivery log. A panic alert can
// carry tens of kilobytes of stack trace; the message that was actually sent is
// the record that matters, and the full trace is already in the application log.
const maxStoredTextRunes = 2000

// alertRow mirrors one _alerts row. The db tags are explicit because the column
// names do not all match the Go field names.
type alertRow struct {
	Created   string `db:"created"`
	Level     string `db:"level"`
	AlertKey  string `db:"alert_key"`
	Title     string `db:"title"`
	Text      string `db:"text"`
	Transport string `db:"transport"`
	Status    string `db:"status"`
	Attempts  int    `db:"attempts"`
	Error     string `db:"error"`
}

// persist appends one delivery outcome to the log.
//
// It runs on the worker goroutine, never on a request path, and writes through
// the auxiliary writer connection so it cannot queue behind an application
// write to data.db. Alerts are rate-limited to a couple of dozen an hour, so
// one INSERT per delivery needs no batching.
func (n *Notifier) persist(rec Record) {
	if n.app == nil || !n.cfg.Persist {
		return
	}

	_, err := n.app.AuxNonconcurrentDB().
		NewQuery(`INSERT INTO ` + TableName + `
			(created, level, alert_key, title, text, transport, status, attempts, error)
			VALUES ({:created}, {:level}, {:key}, {:title}, {:text}, {:transport}, {:status}, {:attempts}, {:error})`).
		Bind(dbx.Params{
			"created":   types.NowDateTime().String(),
			"level":     rec.Level,
			"key":       rec.Key,
			"title":     rec.Title,
			"text":      truncateRunes(rec.Text, maxStoredTextRunes),
			"transport": rec.Transport,
			"status":    rec.Status,
			"attempts":  rec.Attempts,
			"error":     truncateRunes(rec.Error, maxStoredTextRunes),
		}).
		Execute()
	if err != nil {
		// A failure to log the alert must not become an alert — that is how you
		// get a loop. Log it and move on.
		n.app.Logger().Error("Failed to record alert delivery", "error", err)
	}
}

// Recent returns the tail of the delivery log, newest first.
//
// Ordering is by id, not created: several alerts can share a millisecond, and
// id is the rowid, so this is both stable and index-free.
func (n *Notifier) Recent(limit int) []Record {
	if n == nil || n.app == nil || !n.cfg.Persist {
		return []Record{}
	}
	if limit <= 0 {
		limit = RecentLimit
	}

	var rows []alertRow
	err := n.app.AuxDB().
		Select("created", "level", "alert_key", "title", "text", "transport", "status", "attempts", "error").
		From(TableName).
		OrderBy("id DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		n.app.Logger().Error("Failed to read alert history", "error", err)
		return []Record{}
	}

	out := make([]Record, 0, len(rows))
	for _, r := range rows {
		created, _ := types.ParseDateTime(r.Created)
		out = append(out, Record{
			Created:   created.Time(),
			Level:     r.Level,
			Key:       r.AlertKey,
			Title:     r.Title,
			Text:      r.Text,
			Transport: r.Transport,
			Status:    r.Status,
			Attempts:  r.Attempts,
			Error:     r.Error,
		})
	}
	return out
}

// Purge deletes delivery records older than the configured retention. It is
// what the __pbExtAlertsClean__ system job calls.
func (n *Notifier) Purge() (int64, error) {
	if n == nil || n.app == nil {
		return 0, nil
	}

	cutoff, err := types.ParseDateTime(time.Now().AddDate(0, 0, -n.cfg.RetentionDays))
	if err != nil {
		return 0, err
	}

	// The auxiliary writer, so a bulk delete never takes the data.db lock.
	res, err := n.app.AuxNonconcurrentDB().
		NewQuery(`DELETE FROM ` + TableName + ` WHERE created < {:cutoff}`).
		Bind(dbx.Params{"cutoff": cutoff.String()}).
		Execute()
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Data assembles the dashboard payload: live counters plus the log tail.
func (n *Notifier) Data() *Data {
	if n == nil {
		return DefaultData()
	}
	return &Data{
		Stats:  n.Stats(),
		Recent: n.Recent(RecentLimit),
	}
}

// truncateRunes shortens s to at most max runes, marking it when it cuts.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
