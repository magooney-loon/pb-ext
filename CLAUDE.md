# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

pb-ext is a Go library that wraps PocketBase with production-ready features: auto-generated OpenAPI docs, cron job tracking, system monitoring, structured logging, and visitor analytics. It includes a superuser dashboard at `/_/_`.

Users import `github.com/magooney-loon/pb-ext/core` and build their server in `cmd/server/`.

## Build & Run Commands

All operations go through `pb-cli`.

| Command | Purpose |
|---|---|
| `pb-cli` | Build frontend + start dev server |
| `pb-cli --run-only` | Start server only (skip frontend build) |
| `pb-cli --build-only` | Build frontend to `pb_public/`, no server |
| `pb-cli --install` | Install all deps, then build + run |
| `pb-cli --production` | Full production build to `dist/` |
| `pb-cli --test-only` | Run tests with coverage reports |
| `go test ./...` | Run all Go tests directly |
| `go test ./core/server/api/...` | Run tests for a specific package |
| `go test ./core/server/api/... -run TestHandlerScenario` | Run a single named test |
| `go test ./core/analytics/ -bench . -cpu 1,8,20` | Analytics throughput benchmarks across core counts |
| `go test -race ./core/alerts/` | Alerts tests; the package is concurrent, so run it under `-race` |
| `go test -race ./core/audit/` | Admin access auditing tests; also concurrent |
| `go test ./core/analytics/ -run TestStress -v` | Analytics sustained-load tests (skipped by `-short`) |

The dev server runs at `127.0.0.1:8090` by default. PocketBase admin: `/_/`, pb-ext dashboard: `/_/_`.

## Architecture

```
core/core.go          — Public facade, re-exports from core/server, core/logging and core/alerts
core/server/          — Server struct, health dashboard, errors, embedded templates
core/server/api/      — OpenAPI doc system: registry, versioned routers, Go AST parsing
core/alerts/          — Operational notifications: notifier (queue + worker), telegram (transport), rules (evaluator), crash (run marker), storage (auxiliary.db delivery log), format, handlers
core/audit/           — Admin access auditing: collector (route classification + auth hooks), detect (intrusion alerts), storage (auxiliary.db access log), schema, handlers
core/analytics/       — Visitor analytics: collector (request path), aggregator (in-memory counters), visitors (session tracking), storage (dashboard queries), schema (auxiliary.db DDL), types
core/jobs/            — Cron job manager, structured logger, API handlers, types
core/logging/         — Structured logging, request middleware, trace IDs
core/monitoring/      — System metrics (CPU, memory, disk, network, runtime)
core/testutil/        — Shared test helpers and fixture specs
cmd/server/           — Example application (user's app entry point)
pkg/scripts/          — Build toolchain CLI source (compiled to pb-cli)
core/server/templates/ — Embedded Go templates for the dashboard UI
```

**Server lifecycle** (`core/server/server.go`):
1. `New(opts...)` creates a Server wrapping PocketBase
2. `OnBootstrap`: applies pb-ext migrations → initializes alerts → initializes the auditor and binds its auth hooks → JobLogger → JobManager → registers system jobs → JobHandlers
3. `OnServe`: registers health route, analytics, job routes, alert routes, audit middleware and routes, claims the run marker, static file serving
4. `OnTerminate`: closes the analytics collector and the auditor (flushing both), marks the run clean and drains the alert queue
5. User code hooks in via `srv.App().OnServe().BindFunc()`

Alerts come up **before** the job manager so a job that fails during registration or on its first run can already be reported.

**Key singletons**: `GetJobManager()` returns a package-level `*JobManager` initialized during bootstrap. `alerts.Get()` returns the package-level `*Notifier` — never nil, so callers need no guard.

## Schema Migrations

pb-ext's schema is created by PocketBase migrations, not by imperative setup code.

| Migration | Creates | Database |
|---|---|---|
| `1780000000_pbext_jobs.go` | `_job_logs` collection | `data.db` |
| `1780000002_pbext_analytics.go` | `_analytics` table | `auxiliary.db` |
| `1780000003_pbext_alerts.go` | `_alerts` table | `auxiliary.db` |
| `1780000004_pbext_audit.go` | `_admin_access` table | `auxiliary.db` |

Each package registers its migration into `core.AppMigrations` from an `init()`, so importing `core/jobs` or `core/analytics` is enough — `apis.Serve` runs `RunAllMigrations()` before building the router, and `tests.NewTestApp` runs them too (which is why `testutil.NewTestApp` needs no extra setup).

`MigrationsRunner.Up`/`Down` wrap an `AuxRunInTransaction` around a `RunInTransaction`, so a single migration can touch either database — that is how the analytics migration creates an aux table while being registered in the ordinary list. Tests driving a migration by hand must nest the same way (`runMigration` in `analytics_test.go`, `dropOwnSchema` in `server/migrations_test.go`).

**Three non-obvious constraints, all covered by tests — don't break them:**

1. **Migration file names must stay `<timestamp>_pbext_<name>.go`.** PocketBase keys applied migrations on `filepath.Base` alone, with no import path. A plain name like `migrations.go` would silently collide with an app's own file of the same name and be skipped. The timestamp keeps pb-ext ordered after PocketBase's system migrations (newest is `1778828400`) and before anything an app generates today. Names are passed explicitly via `core.Migration.File` rather than derived from `runtime.Caller`.

2. **`Server.Start` applies pb-ext's migrations during `OnBootstrap`** (`core/server/migrations.go`), because PocketBase only runs `core.AppMigrations` at the start of `apis.Serve` — *after* `OnBootstrap`. The job manager is built during bootstrap since user code expects `GetManager()` to work from its own `OnServe` hooks, which are registered before `srv.Start()` and therefore run first. Only pb-ext's own migrations are applied early; the app's run at their normal time. Applying them early records them in `_migrations`, so the later `RunAllMigrations` pass is a no-op.

3. **The auxiliary.db migrations need a `ReapplyCondition`** — this applies to `_analytics`, `_alerts` and `_admin_access`. `_migrations` lives in `data.db` (PocketBase's runner creates it with `app.DB()`) but those tables live in `auxiliary.db`. Deleting `auxiliary.db` — a reasonable thing to do, it holds only logs and counters — would otherwise leave history claiming the migration is applied with no table to show for it. The condition re-runs `Up` whenever `AuxHasTable` is false, exactly as PocketBase's `_logs` migration does. `TestApplyOwnMigrations_RecreatesDeletedAuxTable` covers it.

`Initialize` in both packages verifies its schema exists (`FindCollectionByNameOrId` for jobs, `AuxHasTable` for analytics) and returns an error naming the missing migration, rather than failing obscurely later.

**No clash with app migrations**: pb-ext shares `core.AppMigrations` with the app (the same extension point PocketBase's `jsvm` plugin uses). Ordering is by file name, `migrate history-sync` sees pb-ext's entries so it won't prune them, and automigrate only fires on Admin UI/API collection requests — never on pb-ext's programmatic `SaveNoValidate`. The one hazard is `migrate down N` reverting far enough back to hit pb-ext's migrations; the old timestamps make that unlikely in practice.

## System Metrics

Every collector in `core/monitoring/` wraps gopsutil, whose field names rarely mean what they look like. The rules below are each pinned by a regression test — read them before touching a collector or the metric templates.

**Memory** (`memory.go`): `Free` and `Available` are not the same thing and the dashboard must show `Available`.

On Linux `Free` is `MemFree` — completely untouched pages — which sits near zero on any warm machine because the kernel fills RAM with page cache. `Available` is `MemAvailable`: what a new allocation can actually obtain, cache included. Showing `Free` understates usable memory by an order of magnitude (0.7 GB vs 20.7 GB on a 31 GB box).

`Used + Free != Total`. gopsutil computes `Used = Total - Free - Buffers - Cached` (with `Cached` including `SReclaimable`), so the missing remainder is `Cached` — which is why `MemoryInfo` tracks it. `UsedPercent` is `Used/Total` and is correct as-is.

`core/monitoring/memory_test.go` verifies `Total` and `Available` against `/proc/meminfo` directly (Linux-gated) and pins the `Used + Free + Cached ≈ Total` identity.

**CPU** (`cpu.go`): call `cpu.Percent` with `percpu=true`. With `percpu=false` gopsutil returns a *single* aggregate value; assigning it positionally leaves every entry after the first at zero, and since the dashboard averages across entries the meter then reads low by a factor of the CPU count. `assignUsage` also falls back to the mean when `cpu.Info` and `cpu.Percent` disagree on entry count, so the average stays correct either way. Temperature is optional — a host without sensors is not a collection failure.

**Disk** (`disk.go`): `Used + Free != Total`. `Free` excludes reserved blocks (5% on ext4 by default), so usage is `Used/(Used+Free)` — what `df` prints — not `Used/Total`. gopsutil computes this correctly as `UsedPercent`; `SystemStats.DiskUsagePercent` carries it through so templates never recompute it.

`CollectDiskInfo` takes the path to measure, and the dashboard passes `app.DataDir()` — the filesystem holding `pb_data` is the one that decides when the database runs out of room. `/` is often a small read-only image layer in a container, or an ostree overlay on an atomic host, and reports something unrelated (100% of 0.0 GB on one such machine, versus 11.3% of 1.9 TB for the data directory). An empty or unqueryable path falls back to `DefaultDiskPath`, and `DiskInfo.Path` / `SystemStats.DiskPath` report whichever was actually measured — the Disk card shows it so the figure is never ambiguous. `CollectSystemStats` takes the path too, and its refresh cache is keyed on it so a changed path never returns another filesystem's figures.

**Network** (`network.go`): identify loopback by the interface *flag*, never by looking for `"lo"` in the name — that substring also matches `wlo1` and `eno1` (systemd predictable names for onboard wireless/ethernet), which silently drops a machine's primary interface and its byte counters. Byte totals are cumulative since boot, not throughput.

**Temperature** (`temperature.go`): classify ambient *before* system, because `IsSystemTemp` also accepts `"ambient"` and would otherwise make `AmbientTemp` permanently zero. Each category keeps the highest reading rather than whichever sensor came last — sensor order is not guaranteed and groups like coretemp report a package sensor plus one per core. `SystemStats.Temperatures` holds the classified readings so template helpers never re-read sensors mid-render.

**Requests** (`requests.go`): `RequestStats` keeps monotonic counters, a scalar rate and a fixed 100-entry ring — and **no per-path breakdown**. It used to hold a `map[path]*PathStats`: keyed by attacker-chosen input, one entry per 404 from the static file handler, paths bounded only by the ~8KB header limit, never evicted, and — decisively — never read by anything. A scanner walking long junk URLs bought permanent server memory for free. If per-path stats are ever wanted, they need the treatment `core/analytics` gives the same hazard (`MaxDistinctPaths` plus the `/*` overflow bucket, a path-length cap) *and* a consumer to justify them. `TestRequestStats_TrackRequestIsBounded` pins this.

`Totals()` counts 4xx and 5xx separately, because alerting on the sum fires on any bot sweeping for `/wp-admin`. Prefer it over `GetRequestRate()` for anything that computes a rate — see the note in the Alerts section.

**Process** (`process.go`): `OpenFiles` alone is not an alertable figure. The soft `RLIMIT_NOFILE` is 1024 on some hosts and 1048576 on others, so the same count means "about to fail" or "idle" depending on the machine — `OpenFilesPercent` is the ratio against `OpenFilesLimit`, and that is what the saturation rule watches. `OpenFilesLimit` is 0 where the ceiling is unknown (Windows, or a failed `Getrlimit`), which every consumer must treat as "skip the check", never as "0%". The lookup is build-tagged: `rlimit_unix.go` / `rlimit_other.go`.

**Runtime** (`runtime.go`): `HeapObjects` is a *count* of live objects, not a size. Dividing it by 1048576 and labelling it MB renders a meaningless near-zero figure; use `AllocatedBytes` for heap size and the `formatCount` template func for counts. `LastGCDuration` uses the documented `PauseNs[(NumGC+255)%256]` ring-buffer index.

## OpenAPI Documentation System

The API doc system uses Go AST parsing at startup to extract endpoint metadata. See `core/server/api/AGENTS.md` for full internals.

**Source file directives:**
- `// API_SOURCE` at the top of a `.go` file — marks it for AST parsing
- `// API_DESC <text>` before a handler — sets its OpenAPI description
- `// API_TAGS <csv>` before a handler — sets its OpenAPI tags

**What is auto-detected from source (no annotations needed):**
- Request body type (from `c.BindBody(&req)` or `json.Decode`)
- Response schema (from `c.JSON(status, expr)` — struct, map literal, or helper call)
- Query, header, and path parameters — direct access (`q.Get("x")`, `PathValue("id")`, `Header.Get("x")`) AND indirect via helper functions that wrap param access
- Auth requirements (from PocketBase auth pattern detection)

**Indirect parameter extraction**: if a handler calls a helper like `parseTimeParams(e)` that internally reads query params, those params are automatically detected. Generic helpers (`parseIntParam(e, "page", 0)`) resolve the param name from the call site's second string-literal argument.

**Routes** are registered through `api.VersionedAPIRouter` which wraps PocketBase's router. Each API version has its own isolated parser, schema generator, and registry.

**Spec generation**: In dev mode the spec is generated at runtime from AST. In production, pre-built specs are loaded from `core/server/api/specs/`. The `--gen-spec` flag in `cmd/server/main.go` triggers build-time spec generation (used by `pb-cli --production`).

**Debug endpoint:** `GET /api/docs/debug/ast` — full pipeline introspection (structs, handlers, schemas, OpenAPI output). Requires auth.

**Swagger UI** is served with dark mode CSS (SwaggerDark by Amoenus, MIT).

## Cron Jobs

Jobs are registered via `server.GetJobManager().RegisterJob(id, name, desc, cronExpr, func(*JobExecutionLogger))`. The `JobExecutionLogger` provides structured logging methods: `Start`, `Info`, `Progress`, `Success`, `Error`, `Statistics`, `Complete`, `Fail`. Execution logs are stored in the `_job_logs` PocketBase collection and auto-purged after 72 hours.

## Alerts

Operational notifications — crashes, failed cron jobs, recovered panics, traffic and error spikes, plus anything the app wants to report — delivered to a chat transport. Telegram is the only one built in; `Transport` exists so another is a file rather than a redesign. Design rationale and the full failure-mode table live in `TELEGRAM.md`.

**`core/alerts` imports nothing from pb-ext** — only the stdlib and PocketBase. That is forced: `core/server`, `core/jobs` and `core/logging` all emit alerts, and `logging → server → analytics/jobs` already exists, so any dependency the other way is a cycle. System metrics reach the rules through a `MetricsFunc` the server supplies (`Server.metricsSnapshot`), not through an import of `core/monitoring`.

**`Send` does no I/O and never blocks.** A cooldown check (one map lookup under a short mutex) then a non-blocking channel send; a single background worker owns every HTTP call and every database write. Alerts are emitted from request handlers and from panic recovery, so an unreachable Telegram must cost a request nothing. A full queue **drops and counts** rather than blocking — messages are already deduplicated, so 256 queued copies of "CPU high" carry no more information than one.

**Nothing here can break the server.** `Initialize` returns no error: a missing token, a revoked bot, a network partition, a missing `_alerts` table and a full queue are all ordinary states with defined behaviour. `alerts.Get()` never returns nil, so application code calls `alerts.Get().Send(...)` unguarded.

**Crash detection happens on the way back up, not on the way down** (`crash.go`). An OOM kill, a `log.Fatal` or a panic on an unrecovered goroutine ends the process in microseconds; a Telegram delivery needs hundreds of milliseconds, so sending from a dying process buys a hung exit and no message. Instead each run writes `pb_data/.pbext_lastrun.json` with `state: running`, refreshed on the evaluator's heartbeat and set to `stopped` by `OnTerminate` (including on a restart — a dev-mode reload is not an incident). A marker still reading `running` at boot means the previous process never reached its shutdown hook, and the heartbeat timestamp says roughly when it stopped. **A missing, unreadable or corrupt marker produces no alert** — a read-only data directory would otherwise claim a crash at every boot, and an integration that cries wolf gets muted within a day. Fail closed on false alarms.

**Resource saturation is watched out of the box; traffic thresholds are not.** The split is deliberate and the reasoning is not interchangeable. A request rate has no universal danger zone — 50/s is idle for one deployment and an incident for another — so a shipped default would be either silent or deafening. Saturation is not like that: a disk at 95% is about to stop SQLite writing on every machine there has ever been, and memory, swap and descriptor exhaustion are similarly absolute. Shipping those off by default means the common case is a server that looks monitored and says nothing while it fills up.

| Rule | Default | Notes |
|---|---|---|
| `disk_high` | **90%** | Earliest warning of the lot — a full disk stops writes and does not recover on its own. Uses `DiskUsagePercent` (`Used/(Used+Free)`), never `Used/Total` |
| `memory_high` | **90%** | `MemoryInfo.UsedPercent` excludes page cache, so 90% here is real pressure, not a warm cache |
| `swap_high` | **80%** | Skipped entirely when `SwapTotal == 0` — a host with no swap reports 0%, which is not a measurement |
| `cpu_high` | **90%** | Needs `SustainTicks` behind it; a batch job pinning the cores for one tick is working as intended |
| `open_files_high` | **80%** | Ratio against the soft `RLIMIT_NOFILE`. Skipped when the limit is 0 (unknown) — the raw count is meaningless, since the ceiling is 1024 on some hosts and 1048576 on others |
| `error_rate` | off | Opt-in via `WithErrorRateAlert` |
| `traffic_surge` | off | Opt-in via `WithTrafficSurgeAlert` |
| `goroutines_high` | off | Opt-in: a healthy busy server legitimately runs thousands, and a leak is unbounded growth rather than any particular number |

Tune one dimension with `WithDiskAlert(95)` etc., or drop the lot with `WithoutResourceAlerts()`. Passing 0 to any of them disables that rule.

**Rules are edge-triggered, not level-triggered** (`rules.go`). A rule that becomes true fires once and stays quiet until it becomes false, which can send a recovery notice. "CPU above 90%" evaluated every 30s without a state machine is 120 identical messages an hour. `Sustain` requires N consecutive breaches; a panicking rule is disabled after 3 panics rather than taking out the evaluator.

**Rates are differentiated from monotonic counters** inside the evaluator, never read from a pre-computed field. `monitoring.RequestStats.requestRate` is only recalculated when a request arrives, so after traffic stops it reports the last busy figure forever — precisely when a rate matters. `RequestStats.Totals()` supplies the monotonic counters; 4xx and 5xx are tracked separately because alerting on their sum fires on any bot sweeping for `/wp-admin`.

**Telegram specifics** (`telegram.go`, `format.go`):

- **HTML parse mode, never MarkdownV2.** MarkdownV2 requires escaping every one of ``_*[]()~`>#+-=|{}.!`` — which is exactly what alert bodies are made of (paths, stack traces, Go error strings, "1.2s"). One missed escape is a 400, so the message that silently fails is the interesting one. HTML needs three characters escaped.
- **The token is a URL path segment**, so every `net/http` error quotes it. Every error leaving the transport goes through `scrub()`; `TestTelegram_NeverLeaksTheTokenInErrors` and `TestNotifier_NeverLeaksTheTokenIntoStats` are what keep it out of logs, `Stats` and the delivery log.
- **The 4096 limit is in UTF-16 code units.** Counting runes or bytes misjudges it in opposite directions, and every message starts with an emoji (2 units). The header is rendered first and the body gets the remaining budget, so a 40KB stack trace can never push the title out.
- **429 is the one 4xx that means "later, not never"** — its `retry_after` is honoured. 401/403/400/404 are permanent: they are logged once, flag `Misconfigured`, and stop the retry loop rather than hot-looping against someone else's API.

**Bounds** — same discipline as analytics:

| Bound | Default | Effect when exceeded |
|---|---|---|
| `QueueSize` | 256 | Message dropped and counted; reported in the next digest |
| `Cooldown` (per `Key`) | 15m | Suppressed and counted; an empty `Key` is never suppressed |
| `MaxAlertsPerHour` | 20 | Overflow held back, summarised in an hourly digest |
| `maxCooldownKeys` | 1000 | Expired entries pruned, then the map is cleared |
| `MaxRetries` | 3 | Backoff 2s/8s/30s, then recorded as failed |
| `DrainTimeout` | 5s | Shutdown stops waiting; a wedged transport never holds the process open |
| stored body | 2000 runes | Truncated in the delivery log; the full trace is in the app log |

**Privacy**: alert bodies carry path, method, status, job id, error text and host. They do **not** carry client IP, user agent, request body or record contents — the same promise `core/analytics` makes. Don't widen the panic alert in `logging/error_handler.go`.

**Configuration**: `server.WithAlerts(alerts.With*...)`, falling back to `PBEXT_TELEGRAM_BOT_TOKEN` / `PBEXT_TELEGRAM_CHAT_ID`; `PBEXT_ALERTS_ENABLED=false` is a kill switch that outranks everything. **Disabled in developer mode by default** — pb-cli restarts on every file save, and each save would otherwise fire a shutdown and a startup notice.

**Storage**: `_alerts` in `auxiliary.db`, plain SQL, same reasoning as `_analytics` — an alert about a slow database must not queue behind the writes it is reporting on. Retention 30 days via `__pbExtAlertsClean__`. Delivery is low-volume, so one INSERT per delivery needs no batching.

**Dashboard**: a card in the Alerts & Access section (`components/alerts_status.tmpl`) plus `GET /api/alerts/status`, `GET /api/alerts/recent` and `POST /api/alerts/test`, all superuser-only. The test endpoint bypasses the queue deliberately — the point of a test button is to learn whether delivery works *right now* — and is rate-limited to one per minute so it cannot pump the bot into a flood limit. Note the dashboard renders with `text/template`, which escapes nothing: anything originating outside this repository must go through the `escapeHTML` template func.

**Testing**: `testutil.NewAlerts(t, opts...)` returns an app, a running notifier and an `AlertCapture` transport. `alerts.WithAPIBaseURL` points the real Telegram transport at an `httptest` server, so no test touches the network. Delivery is asynchronous — assert with `capture.Wait(t)` or poll `Stats()`, never immediately after `Send`. Tests needing `testutil` must live in package `alerts_test` (testutil → jobs → alerts would otherwise cycle).

## Admin Access Auditing

`core/audit` records access to the administrative surfaces and raises alerts on what looks like an intrusion. **On by default.**

**This is the one place pb-ext stores personal data, and that is deliberate.** Analytics answers "how is the site used" and so keeps no identities at all; this answers "who tried to get into the admin panel", which is unanswerable without the client address, the user agent and the account an attempt targeted. The exception is scoped: `audit.WithPersonalData(ip, userAgent, identity bool)` switches each field off individually, `audit.WithEnabled(false)` turns the whole thing off, and everything is deleted after `RetentionDays` (90 by default) by `__pbExtAuditClean__`. Do not widen what is captured without a reason as specific as these.

**What PocketBase already does, so we don't.** PB's `activityLogger` writes every request into `_logs` (aux db): url, method, status, execTime, referer, userAgent, auth collection, and `userIP`/`remoteIP` when `Settings.Logs.LogIP` is on — it defaults to **true**. Duplicating that would be waste. Four gaps justify this package:

1. **A failed login never records which account was targeted.** The attempted identity is in the request body; PB logs a 400 and nothing more. This is the highest-value field in an intrusion investigation and it exists nowhere else. `OnRecordAuthWithPasswordRequest` is the only place it is observable.
2. `Settings.Logs.LogAuthId` defaults to **false**, so the built-in log says "a superuser" and not which one.
3. `Settings.Logs.MaxDays` defaults to **5**. Intrusions are usually noticed later than that.
4. Admin access in `_logs` is a JSON blob among every other request — no rollup, nothing to alert on.

**The submitted password is never read.** `RecordAuthWithPasswordRequestEvent` carries a `Password` field and nothing in the package touches it. `TestPackage_NeverReadsTheSubmittedPassword` parses the package's AST and fails on any selector named `Password` — a behavioural test cannot prove an absence, a source scan can. Don't defeat it.

**What is captured** (`collector.go`): `/_/_` (pb-ext dashboard), `/_/` (PB admin UI, minus its static assets — the SPA pulls dozens per page view), any `/api/` call carrying superuser auth, anything targeting `_superusers`, plus every superuser auth success and failure. Query strings are stored with `token`/`password`/`secret`/`access_token`/`api_key` values replaced — an audit log is a place people paste into tickets.

**Anonymous dashboard hits are `denied`, not `allowed`.** An unauthenticated GET of `/_/_` renders the login screen and returns 200 (`health.go`), so by status alone every probe would read as success. `outcomeOf` special-cases it; `TestOutcomeOf_AnonymousDashboardHitsAreDenials` pins it.

**Zero database work on the request path**, and it matters more here than elsewhere: this code runs while the server is being scanned and must not become the thing that falls over. Events fold into a bounded map under one short mutex; a worker writes batches every `FlushInterval` (5s). **All detection runs on that worker** — both questions it asks ("how many failures from this address recently", "has this address ever signed in before") are database reads, and asking them from the auth handler would put a query on the login path an attacker is hammering, so the check meant to detect a flood would amplify it.

**Repeats collapse.** Identical events inside one flush window become one row with `count` and `last_seen`. Ten thousand hits on `/_/` from one address in ten seconds is one row that says so, not ten thousand rows burying everything else.

**At the ceiling, only *new* distinct keys are refused.** A flood from one source against one path is a single key, so it keeps being counted exactly — that flood is the reason the ceiling was reached, so losing its count would lose the thing worth knowing. Dropped events are counted and raise their own alert: a security log with a hole in it must say so.

| Alert | Trigger | Level |
|---|---|---|
| Failed superuser login | Any rejected attempt | Warn, keyed per source |
| Repeated failed logins | `BruteForceThreshold` (5) within `BruteForceWindow` (10m), counting rows already on record | Critical |
| Sign-in from a new address | A successful auth from an IP with no prior success in the retained history | Warn |
| Non-superuser on an admin surface | An authenticated non-superuser reaching `/_/`, `/_/_` or a superuser API route | Warn |
| Auditing not recording | The `_admin_access` table is missing | Critical |

A missing table is loud rather than silent: **an empty security log reads as "nothing happened"**, so `Initialize` logs an error, sets `Recording() == false`, alerts, and the dashboard card says "Not recording" instead of showing an empty table.

`hasPriorSuccess` returns **true** on a query error — failing quiet, because a read failure must not manufacture a "new address" alert for somewhere the administrator signs in daily.

**Endpoints**: `GET /api/audit/status`, `/api/audit/recent`, `/api/audit/sources`, all superuser-only and capped at 500 rows, so a stolen superuser token cannot bulk-export the log in one call. Dashboard card lives in the Alerts & Access section (`#alerts`).

## Analytics

Request middleware counts page views as daily aggregates. **No personal data is persisted** — no IP, user agent, or visitor ID ever reaches the database.

**The request path does zero database work.** A tracked view folds into an in-memory counter map, a recent-visit ring, and a minute bucket under one short mutex (~500ns, 1 alloc). A background goroutine writes accumulated deltas every `FlushInterval` (default 10s) in a single batched transaction. Sustained: ~1.8M views/sec with zero loss.

Never add per-request database work to `Track` — that is the exact bottleneck this design exists to remove.

**Counters live in `auxiliary.db`, not `data.db`** — the same split PocketBase uses for `_logs`, and for the same reason. Each database has its own writer connection (`NonconcurrentDB()` and `AuxNonconcurrentDB()` are both capped at 1) and its own WAL, so a flush can neither block nor be blocked by an application write, however many rows it carries. Use `AuxRunInTransaction` + `AuxNonconcurrentDB()` to write and `AuxDB()` to read; `TestFlush_DoesNotWaitOnTheDataDBWriter` holds the data.db writer open and fails if a flush waits on it. The daily retention DELETE (`__pbExtAnalyticsClean__` in `core/jobs/manager.go`) runs on the aux writer for the same reason. Backups still cover it — PocketBase archives the whole data dir and checkpoints the aux WAL first.

**`_analytics` is a plain SQLite table, not a PocketBase collection.** That follows from living in `auxiliary.db`, and costs nothing: every read and write in the package is raw SQL, and nothing needs the records API, realtime or collection rules. The DDL is in `schema.go`; `migrations.go` applies it. One row per `(path, date, device_type, browser)` with `views`, `unique_sessions` (sessions started by a new visitor) and `returning_sessions` counters, deleted after 90 days.

The text columns are `NOT NULL DEFAULT ''`. SQLite treats NULLs as distinct inside a unique index, so one NULL `device_type` would let the same key insert twice and silently split its counters instead of upserting — `idx_analytics_upsert` is what the `ON CONFLICT` target in `buildUpsert` resolves against, so it is required for correctness, not just speed.

**What is filtered out** (`collector.go`): non-GET methods, non-2xx/3xx responses, bot user agents, static assets, and the `/api/`, `/_/`, `/_app/immutable/`, `/.well-known/` prefixes.

**Everything is bounded** — these limits are the defense against junk-URL floods and forged client identities:

| Bound | Default | Effect when exceeded |
|---|---|---|
| `MaxDistinctPaths` | 5000/day | Extra paths collapse into the `/*` bucket; views are still counted |
| `MaxPathLength` | 255 | Longer paths go straight to `/*` |
| `VisitorGenerations` × `MaxVisitorsPerGeneration` | 4 × 25000 | Oldest generation is dropped wholesale |
| `MaxPendingCounters` | 10000 | Triggers an early flush (never drops a view) |

Configure with the `With*` options passed to `analytics.Initialize`.

**Visitor tracking** (`visitors.go`) is memory-only, keyed by a non-reversible FNV-1a hash of `(RealIP, user agent)`. Entries live in rotating generations, giving a hard memory ceiling and O(1) amortized cost — no unbounded map, no full scan under lock. A visitor is remembered for up to `VisitorGenerations × SessionWindow` (default 2h); returning after that reads as new. `RealIP()` honours PocketBase's admin-configured `TrustedProxy` settings, so `X-Forwarded-For` is only trusted when a proxy is configured.

**Dashboard** (`storage.go`): four `GROUP BY` aggregates bounded to `LookbackDays`, each served by a covering index (`idx_analytics_totals`, `_pages`, `_devices`, `_browsers`) so SQLite never does per-row table lookups. Results are memoized for `CacheTTL` (5s); recent visits and hourly activity come live from memory. ~29ms at 50k rows, ~300ns cached. If you change a dashboard query, keep it covered — `TestGetData_QueriesUseCoveringIndexes` asserts this.

**Dashboard templates** (`core/server/templates/`) render into a buffer before anything is written, and the `/_/_` route is wrapped in `recoverDashboardPanic`, so a bad metric degrades to a 500 for that one request instead of unwinding the handler — this does not depend on the embedding app calling `logging.SetupRecovery`. Three traps, all covered by `core/server/dashboard_render_test.go`, which renders the whole page against healthy, sensorless, nil-slice and all-zero data and asserts no panic and no `NaN`/`+Inf`:

- **`isset` on a struct.** It handles slices, maps and (now) structs; the original had no struct case and silently returned `false`, which is why the CPU Details card read `N/C` while System Metrics showed a real temperature from the same data. A guard that always fails closed is worse than no guard — prefer testing the value (`gt $x.Temperature 0.0`) over probing for field existence.
- **`{{with}}` rebinds the dot.** `{{with $t := getSystemTemp .SystemStats}}` makes `.` a `float64` inside the block, so `.SystemStats` there is a template execution error — and `with` skips the block entirely when the value is zero. Use a plain assignment plus `{{if}}`. This one only fired on hosts that actually have a board sensor.
- **Zero denominators.** `divide`/`divideFloat64`/`percentOf`/`errorRate`/`avgCPUUsage`/`requestRate` all return 0 rather than `NaN`/`+Inf`; keep it that way, and note `divide` only accepts `float64`/`uint64` — passing an `int` silently yields 0.

**Analytics template** (`core/server/templates/components/visitor_analytics.tmpl`): the only consumer of `analytics.Data` — there is no JSON endpoint for it, so display limits belong in the template, not in `GetData`. `RecentVisits` carries the full 50-entry ring but the card renders 8; `TopPages` carries 10 and renders 5. Template funcs live in `templateFuncs` (`health.go`); `pathLabel` renders the `/*` overflow bucket as "other pages". `core/server/templates_test.go` parses every template and renders this one against populated and empty data, which is the only place a missing template func gets caught (at runtime it is merely logged).

**Testing**: `testutil.NewTestApp(t)` already has both pb-ext schemas (migrations run automatically). `testutil.NewAnalytics(t, opts...)` returns an app plus a running collector (closed on cleanup); `testutil.AnalyticsTotals(t, app)` reads persisted counters — from `AuxDB()`, like everything else that touches the table. Pass `WithFlushInterval(time.Hour)` to observe buffering and flush by hand. Internals (`aggregator`, `visitorTracker`) are tested white-box in package `analytics`; anything needing `testutil` must live in package `analytics_test` to avoid an import cycle.

## Example App Patterns

`cmd/server/` is the canonical reference for how to integrate pb-ext:
- `routes.go` — how to initialize versioned API routers and register routes
- `handlers.go` — how to use `API_SOURCE`, `API_DESC`, `API_TAGS` directives and define request/response types
- `jobs.go` — how to register cron jobs with `GetJobManager().RegisterJob`
- `alerts.go` — the full alerting and auditing option reference (every option, with its default, commented where inactive), plus custom rules and ad-hoc sends
- `collections.go` — how to define PocketBase collections programmatically

## Conventions

- The `core/` package is the library; `cmd/server/` is the example app showing how to use it
- Server options use the functional options pattern (`WithConfig`, `WithPocketbase`, `InDeveloperMode`)
- pb-ext schema objects are prefixed with `_` (`_job_logs` collection in `data.db`, `_analytics` table in `auxiliary.db`), created via registered migrations
- Schema changes go in a **new** migration file; never mutate an already-released one
- Dashboard templates use Go `text/template` with `embed.FS`
- **Dashboard tabs are driven by one list**, `DASHBOARD_TABS` in `templates/scripts/main.tmpl`. Adding a tab means one entry there plus a matching sidebar `<a class="…-tab" href="#name">` and a `<div id="name-section">` in `index.tmpl`; the wiring (guard, switch, initial hash, click handlers, hashchange) derives from the list. `TestDashboard_EveryRegisteredTabHasMarkup` fails if the three drift apart — which matters because `setupTabNavigation` bails out entirely when any section is missing, taking the whole sidebar down rather than just the new tab
- Module path: `github.com/magooney-loon/pb-ext`
- AST parser files are split by responsibility: `ast.go` (entry points), `ast_func.go` (handler/function analysis), `ast_struct.go` (struct/schema), `ast_metadata.go` (value/type resolution), `ast_file.go` (file utilities)
- Registry is split: `registry.go` (core), `registry_routes.go` (route registration), `registry_spec.go` (OpenAPI spec generation)
