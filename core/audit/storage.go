package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// accessRow mirrors one _admin_access row. The db tags are explicit because not
// every column name matches its Go field name.
type accessRow struct {
	Created    string  `db:"created"`
	LastSeen   string  `db:"last_seen"`
	Kind       string  `db:"kind"`
	Method     string  `db:"method"`
	Path       string  `db:"path"`
	Query      string  `db:"query"`
	Status     int     `db:"status"`
	Outcome    string  `db:"outcome"`
	AuthState  string  `db:"auth_state"`
	Identity   string  `db:"identity"`
	IP         string  `db:"ip"`
	UserAgent  string  `db:"user_agent"`
	Referer    string  `db:"referer"`
	TraceID    string  `db:"trace_id"`
	DurationMs float64 `db:"duration_ms"`
	Error      string  `db:"error"`
	Count      int     `db:"count"`
}

// write inserts a batch inside one auxiliary transaction, so a flush costs a
// single commit however many rows it carries — and takes the auxiliary writer
// lock rather than the application's.
func (a *Auditor) write(batch map[eventKey]*eventAgg) error {
	keys := make([]eventKey, 0, len(batch))
	for key := range batch {
		keys = append(keys, key)
	}

	return a.app.AuxRunInTransaction(func(txApp core.App) error {
		for start := 0; start < len(keys); start += flushChunkRows {
			end := min(start+flushChunkRows, len(keys))

			chunk := keys[start:end]
			sql, params := buildInsert(chunk, batch)

			if _, err := txApp.AuxNonconcurrentDB().NewQuery(sql).Bind(params).Execute(); err != nil {
				return fmt.Errorf("insert %d admin access rows: %w", len(chunk), err)
			}
		}
		return nil
	})
}

// buildInsert renders a multi-row INSERT for the batch.
func buildInsert(keys []eventKey, batch map[eventKey]*eventAgg) (string, dbx.Params) {
	var b strings.Builder
	params := dbx.Params{}

	b.WriteString("INSERT INTO ")
	b.WriteString(TableName)
	b.WriteString(` (created, last_seen, kind, method, path, query, status, outcome,
		auth_state, identity, ip, user_agent, referer, trace_id, duration_ms, error, count) VALUES `)

	for i, key := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		p := fmt.Sprintf("p%d", i)
		fmt.Fprintf(&b,
			"({:%[1]s_created},{:%[1]s_last},{:%[1]s_kind},{:%[1]s_method},{:%[1]s_path},{:%[1]s_query},"+
				"{:%[1]s_status},{:%[1]s_outcome},{:%[1]s_auth},{:%[1]s_ident},{:%[1]s_ip},{:%[1]s_ua},"+
				"{:%[1]s_ref},{:%[1]s_trace},{:%[1]s_dur},{:%[1]s_err},{:%[1]s_count})", p)

		agg := batch[key]
		params[p+"_created"] = stamp(agg.First)
		params[p+"_last"] = stamp(agg.Last)
		params[p+"_kind"] = key.Kind
		params[p+"_method"] = key.Method
		params[p+"_path"] = key.Path
		params[p+"_query"] = key.Query
		params[p+"_status"] = key.Status
		params[p+"_outcome"] = key.Outcome
		params[p+"_auth"] = key.AuthState
		params[p+"_ident"] = key.Identity
		params[p+"_ip"] = key.IP
		params[p+"_ua"] = key.UserAgent
		params[p+"_ref"] = key.Referer
		params[p+"_trace"] = agg.TraceID
		params[p+"_dur"] = agg.DurationMs
		params[p+"_err"] = agg.Error
		params[p+"_count"] = agg.Count
	}

	return b.String(), params
}

// stamp renders a time in PocketBase's storage format, which sorts
// lexicographically — that is what lets the range queries below compare
// strings instead of parsing every row.
func stamp(t time.Time) string {
	dt, err := types.ParseDateTime(t)
	if err != nil {
		return types.NowDateTime().String()
	}
	return dt.String()
}

// since renders the start of the reporting window.
func since(days int) string {
	return stamp(time.Now().AddDate(0, 0, -days))
}

// Recent returns the tail of the access log, newest first.
func (a *Auditor) Recent(limit int) []Record {
	if a == nil || a.app == nil {
		return []Record{}
	}
	if limit <= 0 {
		limit = RecentLimit
	}

	var rows []accessRow
	err := a.app.AuxDB().
		Select("created", "last_seen", "kind", "method", "path", "query", "status", "outcome",
			"auth_state", "identity", "ip", "user_agent", "referer", "trace_id", "duration_ms", "error", "count").
		From(TableName).
		OrderBy("id DESC").
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		a.app.Logger().Error("Failed to read the admin access log", "error", err)
		return []Record{}
	}

	out := make([]Record, 0, len(rows))
	for _, r := range rows {
		created, _ := types.ParseDateTime(r.Created)
		lastSeen, _ := types.ParseDateTime(r.LastSeen)
		out = append(out, Record{
			Created:    created.Time(),
			LastSeen:   lastSeen.Time(),
			Kind:       r.Kind,
			Method:     r.Method,
			Path:       r.Path,
			Query:      r.Query,
			Status:     r.Status,
			Outcome:    r.Outcome,
			AuthState:  r.AuthState,
			Identity:   r.Identity,
			IP:         r.IP,
			UserAgent:  r.UserAgent,
			Referer:    r.Referer,
			TraceID:    r.TraceID,
			DurationMs: r.DurationMs,
			Error:      r.Error,
			Count:      r.Count,
		})
	}
	return out
}

// Stats summarises the trailing SummaryWindowDays.
func (a *Auditor) Stats() Stats {
	if a == nil {
		return Stats{}
	}

	a.mu.Lock()
	s := Stats{
		Enabled: a.Recording(),
		Pending: len(a.pending),
		Dropped: a.dropped,
		Written: a.written,
	}
	a.mu.Unlock()

	if a.app == nil || !a.app.AuxHasTable(TableName) {
		return s
	}

	var summary struct {
		Total     int    `db:"total"`
		Failures  int    `db:"failures"`
		Successes int    `db:"successes"`
		Denied    int    `db:"denied"`
		IPs       int    `db:"ips"`
		LastFail  string `db:"last_fail"`
		LastOK    string `db:"last_ok"`
	}

	err := a.app.AuxDB().NewQuery(`
		SELECT
			COALESCE(SUM(count), 0)                                                      AS total,
			COALESCE(SUM(CASE WHEN kind = {:fail} THEN count ELSE 0 END), 0)             AS failures,
			COALESCE(SUM(CASE WHEN kind = {:ok} THEN count ELSE 0 END), 0)               AS successes,
			COALESCE(SUM(CASE WHEN outcome = {:denied} THEN count ELSE 0 END), 0)        AS denied,
			COUNT(DISTINCT CASE WHEN ip != '' THEN ip END)                               AS ips,
			COALESCE(MAX(CASE WHEN kind = {:fail} THEN last_seen END), '')               AS last_fail,
			COALESCE(MAX(CASE WHEN kind = {:ok} THEN last_seen END), '')                 AS last_ok
		FROM ` + TableName + `
		WHERE created >= {:since}`).
		Bind(dbx.Params{
			"fail":   KindAuthFailure,
			"ok":     KindAuthSuccess,
			"denied": OutcomeDenied,
			"since":  since(SummaryWindowDays),
		}).
		One(&summary)
	if err != nil {
		a.app.Logger().Error("Failed to summarise the admin access log", "error", err)
		return s
	}

	s.TotalEvents = summary.Total
	s.AuthFailures = summary.Failures
	s.AuthSuccesses = summary.Successes
	s.DeniedAttempts = summary.Denied
	s.DistinctIPs = summary.IPs
	if t, err := types.ParseDateTime(summary.LastFail); err == nil {
		s.LastFailure = t.Time()
	}
	if t, err := types.ParseDateTime(summary.LastOK); err == nil {
		s.LastSuccess = t.Time()
	}

	return s
}

// TopIPs rolls the window up by source address, busiest first.
func (a *Auditor) TopIPs(limit int) []IPSummary {
	if a == nil || a.app == nil || !a.app.AuxHasTable(TableName) {
		return []IPSummary{}
	}
	if limit <= 0 {
		limit = TopIPsLimit
	}

	var rows []struct {
		IP        string `db:"ip"`
		Events    int    `db:"events"`
		Failures  int    `db:"failures"`
		Successes int    `db:"successes"`
		LastSeen  string `db:"last_seen"`
	}

	err := a.app.AuxDB().NewQuery(`
		SELECT
			ip,
			COALESCE(SUM(count), 0)                                          AS events,
			COALESCE(SUM(CASE WHEN kind = {:fail} THEN count ELSE 0 END), 0) AS failures,
			COALESCE(SUM(CASE WHEN kind = {:ok} THEN count ELSE 0 END), 0)   AS successes,
			MAX(last_seen)                                                   AS last_seen
		FROM ` + TableName + `
		WHERE created >= {:since} AND ip != ''
		GROUP BY ip
		ORDER BY failures DESC, events DESC
		LIMIT {:limit}`).
		Bind(dbx.Params{
			"fail":  KindAuthFailure,
			"ok":    KindAuthSuccess,
			"since": since(SummaryWindowDays),
			"limit": limit,
		}).
		All(&rows)
	if err != nil {
		a.app.Logger().Error("Failed to roll up admin access by source", "error", err)
		return []IPSummary{}
	}

	out := make([]IPSummary, 0, len(rows))
	for _, r := range rows {
		lastSeen, _ := types.ParseDateTime(r.LastSeen)
		out = append(out, IPSummary{
			IP:        r.IP,
			Events:    r.Events,
			Failures:  r.Failures,
			Successes: r.Successes,
			LastSeen:  lastSeen.Time(),
		})
	}
	return out
}

// Purge deletes access records past the retention window. It is what the
// __pbExtAuditClean__ system job calls.
func (a *Auditor) Purge() (int64, error) {
	if a == nil || a.app == nil || !a.app.AuxHasTable(TableName) {
		return 0, nil
	}

	// The auxiliary writer, so a bulk delete never takes the data.db lock.
	res, err := a.app.AuxNonconcurrentDB().
		NewQuery(`DELETE FROM ` + TableName + ` WHERE created < {:cutoff}`).
		Bind(dbx.Params{"cutoff": since(a.cfg.RetentionDays)}).
		Execute()
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Data assembles the dashboard payload.
func (a *Auditor) Data() *Data {
	if a == nil {
		return DefaultData()
	}
	return &Data{
		Stats:  a.Stats(),
		Recent: a.Recent(RecentLimit),
		TopIPs: a.TopIPs(TopIPsLimit),
	}
}
