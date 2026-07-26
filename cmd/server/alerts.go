package main

import (
	"fmt"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/pocketbase/pocketbase/core"
)

// This file is the reference for configuring alerting and admin access
// auditing. Options that are commented out are showing their default, so the
// commented lines double as documentation of what you get without them.
//
// The short version: with a bot token and chat id in the environment, crashes,
// failed cron jobs, recovered panics, intrusion attempts and resource
// saturation all report themselves. Nothing below is required.

// alertOptions configures the notification subsystem.
func alertOptions() []alerts.Option {
	return []alerts.Option{
		// --- Transport -------------------------------------------------------
		//
		// Credentials fall back to PBEXT_TELEGRAM_BOT_TOKEN and
		// PBEXT_TELEGRAM_CHAT_ID, so passing them explicitly is only needed when
		// they come from somewhere else (a secret manager, a config file).
		// With neither, alerting stays disabled and every Send is a no-op.
		//
		// alerts.WithTelegram(token, chatID),
		//
		// A thread inside a forum-style group:
		// alerts.WithTelegramTopic(42),
		//
		// A label on every message, for when several servers report into one
		// chat. Defaults to the hostname.
		// alerts.WithInstance("prod-1"),
		//
		// Somewhere other than api.telegram.org — tests point this at an
		// httptest server; a proxy is the other use.
		// alerts.WithAPIBaseURL("https://telegram.internal"),
		//
		// Replace Telegram entirely. Implement alerts.Transport for Discord,
		// Slack or a plain webhook.
		// alerts.WithTransport(myTransport),

		// --- What gets reported ----------------------------------------------
		//
		// On by default and needing no configuration: server started, recovery
		// from an unexpected exit, graceful shutdown, cron job failures,
		// recovered panics, and the resource ceilings below.
		//
		// alerts.WithLifecycleAlerts(false),  // drop the start/stop/crash notices
		// alerts.WithEnabled(false),          // kill the whole subsystem
		//
		// Alerts are off in developer mode, because pb-cli restarts on every
		// file save and each save would otherwise fire two notices.
		// alerts.WithEnabledInDev(true),

		// --- Resource ceilings (on by default) -------------------------------
		//
		// Defaults: CPU 90%, memory 90%, disk 90%, swap 80%, descriptors 80%,
		// each requiring 3 consecutive checks. Swap is skipped on hosts with
		// none configured; descriptors are skipped where RLIMIT_NOFILE is
		// unknown. Pass 0 to any of these to switch that one off.
		//
		// alerts.WithDiskAlert(95),
		// alerts.WithMemoryAlert(85),
		// alerts.WithCPUAlert(0),              // this one off, others untouched
		// alerts.WithSwapAlert(70),
		// alerts.WithFileDescriptorAlert(75),
		// alerts.WithResourceAlerts(90, 90, 90), // CPU, memory, disk together
		// alerts.WithoutResourceAlerts(),      // drop the lot
		// alerts.WithSustainTicks(5),          // how long a breach must hold

		// --- Traffic thresholds (opt-in) -------------------------------------
		//
		// These are off by default because a request rate has no universal
		// danger zone: 50/s is idle for one deployment and an incident for
		// another. Saturation is absolute; traffic is not.

		// 5xx responses above 10% of a window covering at least 20 requests.
		alerts.WithErrorRateAlert(10, 20),

		// Five times the rolling baseline, but only above 50 req/s — the floor
		// is what stops traffic doubling from 1 to 2 req/s waking anyone.
		alerts.WithTrafficSurgeAlert(5, 50),

		// Goroutine count. Opt-in with no default: a healthy busy server
		// legitimately runs thousands, and a leak is unbounded growth rather
		// than any particular number.
		// alerts.WithGoroutineAlert(10000),

		// --- Flood control ---------------------------------------------------
		//
		// Defaults: one alert per Key per 15 minutes, 20 an hour overall, then
		// an hourly digest summarising what was held back.
		//
		// alerts.WithCooldown(30 * time.Minute),
		// alerts.WithMaxAlertsPerHour(10),
		// alerts.WithQueueSize(512),        // in-memory queue; a full one drops and counts

		// --- Timing and delivery ---------------------------------------------
		//
		// Defaults: evaluate every 30s, 10s per send attempt, 3s minimum
		// between sends (Telegram's per-chat limit), 3 retries with a
		// 2s/8s/30s backoff, and 5s to drain the queue at shutdown.
		//
		// alerts.WithEvaluateInterval(15 * time.Second),
		// alerts.WithSendTimeout(20 * time.Second),
		// alerts.WithMinSendInterval(5 * time.Second),
		// alerts.WithMaxRetries(5),
		// alerts.WithDrainTimeout(10 * time.Second),

		// --- Delivery log ----------------------------------------------------
		//
		// Every delivery is recorded in _alerts (auxiliary.db) and kept 30 days.
		//
		// alerts.WithPersistence(false),
		// alerts.WithRetentionDays(7),

		// Not listed: alerts.WithMetrics. The server supplies the metric
		// snapshot the rules read and appends it last, so a value passed here
		// would be overridden.
	}
}

// auditOptions configures admin access auditing.
//
// It returns nothing, deliberately: every default is right for most
// deployments — auditing on, everything captured, 90-day retention — and the
// commented switches below are for the deployments where they are not.
func auditOptions() []audit.Option {
	return []audit.Option{
		// --- Privacy ---------------------------------------------------------
		//
		// This is the one place pb-ext stores personal data: the client address,
		// the user agent, and the account name a login attempt supplied. That is
		// the point — "who tried to get into the admin panel" is unanswerable
		// without them — but it is also the setting most likely to need changing
		// for a given jurisdiction or policy.
		//
		// Each field switches off independently. All three off leaves a record
		// of what happened and when, with no record of who: enough for capacity
		// questions, not enough for an intrusion investigation.
		//
		// audit.WithPersonalData(true, true, false), // keep IP + agent, drop the account
		// audit.WithPersonalData(false, false, false),
		//
		// Retention, in days. Everything older is deleted by the
		// __pbExtAuditClean__ system job. Default 90.
		// audit.WithRetentionDays(30),

		// --- What gets recorded ----------------------------------------------
		//
		// Default: the admin UI at /_/ , pb-ext's dashboard at /_/_ ,
		// superuser API calls, and every superuser authentication attempt.
		//
		// audit.WithTracking(true, false, true), // skip the per-call API audit
		// audit.WithEnabled(false),              // stop auditing entirely

		// --- Intrusion detection ---------------------------------------------
		//
		// Defaults: alert on each failed superuser login, escalate to critical
		// at 5 failures from one source inside 10 minutes, and alert when a
		// superuser signs in from an address with no prior successful sign-in.
		//
		// audit.WithBruteForceAlert(3, 5*time.Minute),
		// audit.WithAlerts(false, true),  // no per-failure noise, keep new-source
		// audit.WithAlerts(false, false), // record silently, alert on nothing

		// --- Buffering -------------------------------------------------------
		//
		// Defaults: written every 5s, at most 5000 distinct pending events.
		// At the ceiling only new distinct events are refused, so a flood from
		// one source against one path keeps being counted exactly.
		//
		// audit.WithFlushInterval(10 * time.Second),
		// audit.WithMaxPendingEvents(10000),
	}
}

// registerAlerts shows how application code hooks into the alert system.
//
// Nothing here needs to know whether alerting is configured: alerts.Get() never
// returns nil, and Send on an unconfigured notifier is a no-op that does no I/O,
// so these calls can sit in normal code paths without a guard or a feature flag.
func registerAlerts(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// A custom rule. The evaluator runs Check every 30 seconds and turns the
		// boolean into edges: it fires once when the condition starts holding,
		// stays quiet while it continues to hold, and sends a recovery notice
		// when it clears.
		//
		// Check runs on the evaluator goroutine, so it may query the database —
		// but it delays every other rule, so it should return promptly.
		if err := alerts.Get().AddRule(alerts.Rule{
			Key:      "todo_backlog",
			Level:    alerts.LevelWarn,
			Recovery: true,
			// Require three consecutive breaches, so a momentary spike during a
			// bulk import does not page anyone.
			Sustain: 3,
			Check: func() (bool, alerts.Message) {
				collection, err := app.FindCollectionByNameOrId("todos")
				if err != nil {
					return false, alerts.Message{}
				}

				pending, err := app.CountRecords(collection, nil)
				if err != nil || pending < 500 {
					return false, alerts.Message{}
				}

				return true, alerts.Message{
					Title: fmt.Sprintf("Todo backlog at %d items", pending),
					Fields: map[string]string{
						"threshold": "500",
					},
				}
			},
		}); err != nil {
			app.Logger().Error("Failed to register the backlog alert rule", "error", err)
		}

		// A rule reading the audit trail, showing how the two fit together.
		// audit.Get() is also never nil.
		if err := alerts.Get().AddRule(alerts.Rule{
			Key:      "admin_probing",
			Level:    alerts.LevelWarn,
			Recovery: true,
			Check: func() (bool, alerts.Message) {
				stats := audit.Get().Stats()
				if stats.AuthFailures < 20 {
					return false, alerts.Message{}
				}
				return true, alerts.Message{
					Title: fmt.Sprintf("%d failed admin logins this week", stats.AuthFailures),
					Fields: map[string]string{
						"distinct sources": fmt.Sprintf("%d", stats.DistinctIPs),
					},
				}
			},
		}); err != nil {
			app.Logger().Error("Failed to register the admin probing rule", "error", err)
		}

		// A one-off alert from application code. An empty Key is never
		// suppressed by the cooldown, which is what makes an explicit Send like
		// this one reliable.
		alerts.Get().Send(alerts.Message{
			Level: alerts.LevelInfo,
			Title: "Example app ready",
			Fields: map[string]string{
				"started": time.Now().Format(time.RFC3339),
			},
		})

		return e.Next()
	})
}
