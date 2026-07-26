# pb-ext

Enhanced PocketBase server with monitoring, alerting, auditing, logging & API docs.

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/magooney-loon/pb-ext)

## Core Features

- **API Schema**: Auto-generates OpenAPI docs UI for your endpoints
- **Cron Tracking**: Logs and monitors scheduled cron jobs
- **System Monitoring**: Real-time CPU, memory, disk, network, and runtime metrics
- **Operational Alerts**: Telegram notifications for crashes, failed jobs, panics and resource saturation
- **Access Auditing**: Who reached the admin surfaces, with brute-force & new-address detection
- **Structured Logging**: Complete logging with error tracking and request tracing
- **Visitor Analytics**: Track GDPR & PII compliant visitors, page views, device types, and browsers
- **PocketBase Integration**: Uses PocketBase's auth system and styling

## Access

- Admin panel:
```bash
127.0.0.1:8090/_
```
- pb-ext dashboard:
```bash
127.0.0.1:8090/_/_
```

The dashboard has six sections: **Health** (system metrics), **Analytics** (visitors), **Alerts & Access** (notification status and the admin access log), **API** (OpenAPI docs and tester), **Cron** (jobs and their logs) and **Rules** (collection access rules).

## Quick Start

```go
package main

import (
	"flag"
	"log"

	app "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	devMode := flag.Bool("dev", false, "Run in developer mode")
	generateSpecsDir := flag.String("generate-specs-dir", "", "Generate OpenAPI specs into the provided directory and exit")
	generateSpecVersion := flag.String("generate-spec-version", "", "Optional API version to generate (requires --generate-specs-dir)")
	validateSpecsDir := flag.String("validate-specs-dir", "", "Validate OpenAPI specs from the provided directory and exit")
	flag.Parse()

	if *generateSpecsDir != "" {
		gen := app.NewSpecGeneratorWithInitializer(func() (*app.APIVersionManager, error) {
			return initVersionedSystem(), nil
		})
		if err := gen.Generate(*generateSpecsDir, *generateSpecVersion); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *validateSpecsDir != "" {
		gen := app.NewSpecGeneratorWithInitializer(func() (*app.APIVersionManager, error) {
			return initVersionedSystem(), nil
		})
		if err := gen.Validate(*validateSpecsDir); err != nil {
			log.Fatal(err)
		}
		return
	}

	initApp(*devMode)
}

func initApp(devMode bool) {
	var opts []app.Option

	if devMode {
		opts = append(opts, app.InDeveloperMode())
	} else {
		opts = append(opts, app.InNormalMode())
	}

	// Option 1: Use a custom PocketBase config
	// pbConfig := &pocketbase.Config{
	// 	DefaultDev:     true,
	// 	DefaultDataDir: "./custom_pb_data",
	// }
	// opts = append(opts, app.WithConfig(pbConfig))

	// Option 2: Use an existing PocketBase instance
	// pb := pocketbase.New()
	// opts = append(opts, app.WithPocketbase(pb))

	// Set custom port programmatically
	// os.Args = []string{"app", "serve", "--http=127.0.0.1:9090"}

	// Note: WithConfig and WithPocketbase cannot be used together

	// Alerting and admin access auditing. Both work with no configuration at
	// all — alerting needs only PBEXT_TELEGRAM_BOT_TOKEN and
	// PBEXT_TELEGRAM_CHAT_ID in the environment, and auditing is on by default.
	//
	// Every available option, with its default, is documented in
	// cmd/server/alerts.go.
	opts = append(opts,
		app.WithAlerts(alertOptions()...),
		app.WithAudit(auditOptions()...),
	)

	srv := app.New(opts...)

	app.SetupLogging(srv)

	registerCollections(srv.App())
	registerRoutes(srv.App())
	registerJobs(srv.App())
	registerAlerts(srv.App())

	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.SetupRecovery(srv.App(), e)
		return e.Next()
	})

	if err := srv.Start(); err != nil {
		srv.App().Logger().Error("Fatal application error",
			"error", err,
			"uptime", srv.Stats().StartTime,
			"total_requests", srv.Stats().TotalRequests.Load(),
			"active_connections", srv.Stats().ActiveConnections.Load(),
			"last_request_time", srv.Stats().LastRequestTime.Load(),
		)
		log.Fatal(err)
	}
}

// Example models in cmd/server/collections.go
// Example routes in cmd/server/routes.go
// Example handlers in cmd/server/handlers.go
// Example cron jobs in cmd/server/jobs.go
// Example alert rules in cmd/server/alerts.go
//
// You can restructure Your project as You wish,
// just keep this main.go in cmd/server/main.go
//
// Build toolchain (pb-cli):
// go install github.com/magooney-loon/pb-ext/cmd/pb-cli@latest
//
// Need a pre-built Svelte5Kit starter template?
// https://github.com/magooney-loon/svelte-gui
//
// Ready for a production build deployment?
// https://github.com/magooney-loon/pb-deployer
```

```bash
go mod tidy
go install github.com/magooney-loon/pb-ext/cmd/pb-cli@latest
pb-cli --run-only
```

## Alerts

Operational notifications over Telegram. Set two environment variables and restart — no code changes needed:

```bash
export PBEXT_TELEGRAM_BOT_TOKEN="123456789:AA..."   # from @BotFather
export PBEXT_TELEGRAM_CHAT_ID="-1001234567890"      # negative for groups
```

| Variable | Purpose |
|---|---|
| `PBEXT_TELEGRAM_BOT_TOKEN` | Bot token from [@BotFather](https://t.me/BotFather) |
| `PBEXT_TELEGRAM_CHAT_ID` | Target chat; negative for groups |
| `PBEXT_TELEGRAM_TOPIC_ID` | Optional thread id in a forum-style group |
| `PBEXT_ALERTS_ENABLED` | `false` disables alerting regardless of the rest |

With neither credential set, alerting stays disabled and every call is a no-op — nothing to guard in your own code. Alerts are also off in developer mode.

**What reports itself with no configuration:**

| Alert | Trigger |
|---|---|
| Server started / shut down | Lifecycle |
| Recovered from an unexpected exit | A previous run that never reached its shutdown hook — crash, OOM kill, or a host that went away |
| Cron job failed | Any job returning an error or panicking |
| Panic recovered | A panic in a request handler |
| Failed superuser login | A rejected admin sign-in, naming the account targeted |
| Repeated failed logins | 5 failures from one source inside 10 minutes |
| Sign-in from a new address | A successful admin sign-in from an address with no prior success on record |
| Disk / memory / swap / CPU / descriptors | Sustained saturation — 90% disk, 90% memory, 80% swap, 90% CPU, 80% of `RLIMIT_NOFILE` |

**Opt-in**:

```go
app.WithAlerts(
    app.WithErrorRateAlert(10, 20), // 5xx above 10% of a 20+ request window
    app.WithTrafficSurge(5, 50),    // 5× the rolling baseline, floored at 50 req/s
)
```

Tune or drop the defaults with `app.WithDiskAlert(95)`, `app.WithSwapAlert(0)`, or `app.WithoutResourceAlerts()`. Ad-hoc alerts from your own code:

```go
app.GetNotifier().Send(app.AlertMessage{
    Level: app.AlertWarn,
    Title: "Payment webhook rejected",
    Fields: map[string]string{"provider": "stripe", "code": "402"},
})
```

`GetNotifier()` never returns nil and `Send` does no I/O, so it is safe on any code path, including request handlers. Flood control is built in: one alert per key per 15 minutes, 20 an hour, then an hourly digest of what was held back. Custom periodic rules and every option with its default are in `cmd/server/alerts.go`.

### What pb-ext cannot tell you

A process that has been killed cannot report its own death. An OOM kill, a `log.Fatal`, or a panic on an unrecovered goroutine ends the process in microseconds, while a Telegram delivery needs hundreds of milliseconds. pb-ext detects those on the **next** boot instead, from a heartbeat marker in the data directory, and reports roughly when the previous run stopped.

Nothing in-process can tell you about a host that never comes back. For that you need an external dead-man's switch — a cron job pinging healthchecks.io or Uptime Kuma, alerting when the *ping* stops.

## Admin Access Auditing

Records access to the administrative surfaces: PocketBase's admin UI, pb-ext's dashboard, superuser API calls, and every superuser authentication attempt. **On by default.**

It exists because PocketBase's own request log cannot answer four questions: which account a failed login targeted (the attempted identity arrives in the request body and is never logged), which superuser performed an action (`Logs.LogAuthId` defaults to off), what happened more than 5 days ago (`Logs.MaxDays`), and how any of it rolls up per source.

> ⚠️ **This is the one place pb-ext stores personal data.** It keeps the client address, the user agent, and the account name an authentication attempt supplied. That is deliberate — "who tried to get into the admin panel" is unanswerable without them — and bounded: everything is deleted after 90 days.

Narrow or disable it:

```go
app.WithAudit(
    app.WithAuditPersonalData(true, true, false), // keep IP + agent, drop the account name
    app.WithAuditRetentionDays(30),
    app.WithBruteForceAlert(3, 5*time.Minute),
)

app.WithAudit(app.WithAuditEnabled(false)) // off entirely
```

Passwords are never read, logged or stored.

## OpenAPI Spec Generation

### Dev vs Production

- **Development**: Specs are generated at runtime via AST parsing - no disk files needed
- **Production**: Specs are generated at build time and read from disk (`dist/specs/`)

### Build pipeline

The pb-cli toolchain runs OpenAPI generation automatically for production builds:

```bash
pb-cli              # Development mode (no spec generation)
pb-cli --build-only # Build frontend + generate specs
pb-cli --production # Production build with specs
```

For programmatic usage, see `pkg/scripts/README.md`.


Having issues with Your API Docs?
```bash
127.0.0.1:8090/api/docs/debug/ast
```

## Reserved Schema

pb-ext creates the following automatically on startup. **Do not create collections or tables with these names in your own code.**

| Name | Database | Kind | Purpose |
|---|---|---|---|
| `_job_logs` | `pb_data/data.db` | System collection | Cron job execution logs (start time, end time, duration, status, output). Retention: 72 hours. |
| `_analytics` | `pb_data/auxiliary.db` | Plain table | Daily aggregated page view counters (one row per path/date/device/browser). Retention: 90 days. |
| `_alerts` | `pb_data/auxiliary.db` | Plain table | Alert delivery log (level, title, transport, outcome, error). Retention: 30 days. |
| `_admin_access` | `pb_data/auxiliary.db` | Plain table | Admin access log (path, outcome, client address, user agent, account). Retention: 90 days. |

**Schema notes:**
- The three auxiliary tables live in the same database PocketBase uses for `_logs`, so writing to them never contends with your application's writes. They are plain SQLite tables, not collections, and are therefore not exposed through the records API or the Collections UI.
- `_job_logs` is a system collection, hidden from the PocketBase Collections UI.
- **`_job_logs`, `_analytics` and `_alerts` store no personal data** — no IP, no user agent, no visitor ID. Analytics is GDPR-compliant by design: it counts page views as daily aggregates and never records who viewed them.
- **`_admin_access` does store personal data**, deliberately and as the only exception — see [Admin Access Auditing](#admin-access-auditing). Each field can be switched off individually, and the whole table is bounded by its retention window.
- All are included in PocketBase's backups, which archive the whole data directory.
- Schema changes ship as new migrations and are applied at startup with no manual steps required.

## Reserved Routes

pb-ext registers the following routes. **Do not register your own routes at these paths.**

### Dashboard
| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/_/_` | Superuser | pb-ext health, analytics, alerts, access & jobs dashboard |

### Cron Job API
All routes require superuser authentication.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/cron/jobs` | List registered cron jobs |
| `POST` | `/api/cron/jobs/{id}/run` | Trigger a job manually |
| `DELETE` | `/api/cron/jobs/{id}` | Remove a job from the scheduler |
| `GET` | `/api/cron/status` | Cron scheduler status |
| `POST` | `/api/cron/config/timezone` | Update scheduler timezone |
| `GET` | `/api/cron/logs` | Paginated job execution logs |
| `GET` | `/api/cron/logs/{job_id}` | Logs for a specific job |
| `GET` | `/api/cron/logs/analytics` | Aggregated job log statistics |

### Alerts API
All routes require superuser authentication.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/alerts/status` | Delivery state, counters and configured target (never the bot token) |
| `GET` | `/api/alerts/recent` | Recent alert deliveries |
| `POST` | `/api/alerts/test` | Send a test message immediately; limited to one per minute |

### Admin Access API
All routes require superuser authentication and return at most 500 rows per request.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/audit/status` | Capture state and a 7-day summary |
| `GET` | `/api/audit/recent` | Recent admin access records |
| `GET` | `/api/audit/sources` | Per-address rollup, most failures first |

### API Docs
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/docs/versions` | List registered API versions |
| `GET` | `/api/docs/debug/ast` | AST parsing debug info |
| `GET` | `/api/docs/{version}` | Version metadata |
| `GET` | `/api/docs/{version}/spec` | OpenAPI 3.0 spec |
| `GET` | `/api/docs/{version}/swagger` | Swagger UI |

### Internal System Jobs

pb-ext registers these cron jobs automatically. They appear in the dashboard with the "System" badge.

| Job ID | Schedule | Description |
|---|---|---|
| `__pbExtLogClean__` | `0 0 * * *` (daily midnight) | Purge `_job_logs` records older than 72 hours |
| `__pbExtAnalyticsClean__` | `0 3 * * *` (daily 3 AM) | Purge `_analytics` rows older than 90 days |
| `__pbExtAlertsClean__` | `0 4 * * *` (daily 4 AM) | Purge `_alerts` rows past their retention (30 days) |
| `__pbExtAuditClean__` | `0 5 * * *` (daily 5 AM) | Purge `_admin_access` rows past their retention (90 days) |

### Reserved Files

| Path | Purpose |
|---|---|
| `pb_data/.pbext_lastrun.json` | Heartbeat marker used to detect an unclean shutdown on the next boot. Safe to delete; the only effect is that one crash goes unreported. |
