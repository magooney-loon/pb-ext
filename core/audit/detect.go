package audit

import (
	"fmt"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/pocketbase/dbx"
)

// Detection runs on the flush worker, never on a request.
//
// Both questions it asks — "how many times has this address failed recently"
// and "has this address ever signed in successfully before" — are database
// reads. Asking them from the auth handler would put a query on the login path,
// which is precisely the path an attacker is hammering: the check meant to
// detect the flood would amplify it. Answering them a few seconds later, off
// the request, costs nothing and loses nothing.

// detect inspects a drained batch and raises whatever it warrants.
//
// It runs *before* the batch is written, so "has this address succeeded before"
// is not answered by the row that is about to be inserted.
func (a *Auditor) detect(batch map[eventKey]*eventAgg) {
	for key, agg := range batch {
		switch key.Kind {
		case KindAuthFailure:
			a.onAuthFailure(key, agg)
		case KindAuthSuccess:
			a.onAuthSuccess(key, agg)
		}

		if key.AuthState == AuthUser {
			a.onPrivilegeProbe(key, agg)
		}
	}

	a.reportDropped()
}

// onAuthFailure alerts on rejected superuser logins, escalating to critical
// once one address crosses the threshold inside the window.
func (a *Auditor) onAuthFailure(key eventKey, agg *eventAgg) {
	if !a.cfg.AlertOnFailure {
		return
	}

	// Failures already recorded for this address, plus the ones in hand.
	recent := a.recentFailures(key.IP) + agg.Count

	fields := map[string]string{
		"attempts in window": fmt.Sprintf("%d", recent),
		"window":             a.cfg.BruteForceWindow.String(),
	}
	if key.IP != "" {
		fields["source"] = key.IP
	}
	if key.Identity != "" {
		fields["account"] = key.Identity
	}
	if key.UserAgent != "" {
		fields["user agent"] = key.UserAgent
	}

	if recent >= a.cfg.BruteForceThreshold {
		a.alert(alerts.Message{
			Level:  alerts.LevelCritical,
			Key:    "admin_bruteforce:" + key.IP,
			Title:  fmt.Sprintf("Repeated failed superuser logins (%d)", recent),
			Text:   "One source has crossed the failed-login threshold for the admin panel.",
			Fields: fields,
		})
		return
	}

	a.alert(alerts.Message{
		Level:  alerts.LevelWarn,
		Key:    "admin_auth_failure:" + key.IP,
		Title:  "Failed superuser login",
		Fields: fields,
	})
}

// onAuthSuccess alerts when a superuser signs in from an address that has never
// signed in successfully before.
//
// This is the highest-signal event here. A failed login is usually background
// noise from a scanner; a *successful* one from an unfamiliar address is either
// the administrator on a new machine or the thing you built this to catch.
func (a *Auditor) onAuthSuccess(key eventKey, agg *eventAgg) {
	if !a.cfg.AlertOnNewIP || key.IP == "" {
		return
	}
	if a.hasPriorSuccess(key.IP) {
		return
	}

	fields := map[string]string{"source": key.IP}
	if key.Identity != "" {
		fields["account"] = key.Identity
	}
	if key.UserAgent != "" {
		fields["user agent"] = key.UserAgent
	}
	fields["at"] = agg.First.UTC().Format("2006-01-02 15:04:05 MST")

	a.alert(alerts.Message{
		Level:  alerts.LevelWarn,
		Key:    "admin_new_source:" + key.IP,
		Title:  "Superuser signed in from a new address",
		Text:   "This source has no previous successful superuser sign-in on record.",
		Fields: fields,
	})
}

// onPrivilegeProbe alerts when an authenticated non-superuser touches an
// administrative surface — somebody finding out what their token reaches.
func (a *Auditor) onPrivilegeProbe(key eventKey, agg *eventAgg) {
	switch key.Kind {
	case KindAdminUI, KindDashboard, KindAdminAPI:
	default:
		return
	}

	fields := map[string]string{
		"path":     key.Path,
		"method":   key.Method,
		"status":   fmt.Sprintf("%d", key.Status),
		"attempts": fmt.Sprintf("%d", agg.Count),
	}
	if key.IP != "" {
		fields["source"] = key.IP
	}
	if key.Identity != "" {
		fields["account"] = key.Identity
	}

	a.alert(alerts.Message{
		Level:  alerts.LevelWarn,
		Key:    "admin_privilege_probe:" + key.IP,
		Title:  "Non-superuser account reached an admin surface",
		Fields: fields,
	})
}

// reportDropped raises one alert when the buffer has started shedding events,
// because an audit log with a hole in it must say so.
func (a *Auditor) reportDropped() {
	a.mu.Lock()
	dropped := a.dropped
	reported := a.droppedReported
	a.droppedReported = dropped
	a.mu.Unlock()

	if dropped <= reported {
		return
	}

	a.alert(alerts.Message{
		Level: alerts.LevelError,
		Key:   "audit_dropped",
		Title: "Admin access events were dropped",
		Text:  "The audit buffer filled, so some administrative access went unrecorded. This normally means a flood of distinct requests.",
		Fields: map[string]string{
			"dropped": fmt.Sprintf("%d", dropped),
			"ceiling": fmt.Sprintf("%d", a.cfg.MaxPendingEvents),
		},
	})
}

// recentFailures counts already-recorded failures from one address inside the
// brute-force window.
func (a *Auditor) recentFailures(ip string) int {
	if ip == "" || a.app == nil {
		return 0
	}

	var count int
	err := a.app.AuxDB().NewQuery(`
		SELECT COALESCE(SUM(count), 0)
		FROM ` + TableName + `
		WHERE kind = {:kind} AND ip = {:ip} AND created >= {:since}`).
		Bind(dbx.Params{
			"kind":  KindAuthFailure,
			"ip":    ip,
			"since": stamp(time.Now().Add(-a.cfg.BruteForceWindow)),
		}).
		Row(&count)
	if err != nil {
		a.app.Logger().Error("Failed to count recent admin login failures", "error", err)
		return 0
	}
	return count
}

// hasPriorSuccess reports whether an address has any successful superuser
// sign-in on record. It reads the whole retained history, not the summary
// window: an address last used three months ago is still a known one.
func (a *Auditor) hasPriorSuccess(ip string) bool {
	if a.app == nil {
		return false
	}

	var count int
	err := a.app.AuxDB().NewQuery(`
		SELECT COUNT(*) FROM ` + TableName + `
		WHERE kind = {:kind} AND ip = {:ip} LIMIT 1`).
		Bind(dbx.Params{"kind": KindAuthSuccess, "ip": ip}).
		Row(&count)
	if err != nil {
		// Fail quiet rather than loud: a read error must not manufacture a
		// "new address" alert for somewhere the administrator signs in daily.
		a.app.Logger().Error("Failed to check for prior admin sign-ins", "error", err)
		return true
	}
	return count > 0
}
