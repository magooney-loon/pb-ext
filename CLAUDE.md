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

See `core/monitoring/CLAUDE.md` — gopsutil field gotchas for memory, CPU, disk, network, temperature, requests and process/runtime collectors, each pinned by a regression test.

## OpenAPI Documentation System

The API doc system uses Go AST parsing at startup to extract endpoint metadata. See `core/server/api/AGENTS.md` for full internals: source file directives (`API_SOURCE`/`API_DESC`/`API_TAGS`), what's auto-detected from source, indirect parameter extraction, versioned routing, spec generation, and the debug endpoint.

## Cron Jobs

Jobs are registered via `server.GetJobManager().RegisterJob(id, name, desc, cronExpr, func(*JobExecutionLogger))`. The `JobExecutionLogger` provides structured logging methods: `Start`, `Info`, `Progress`, `Success`, `Error`, `Statistics`, `Complete`, `Fail`. Execution logs are stored in the `_job_logs` PocketBase collection and auto-purged after 72 hours.

## Alerts

See `core/alerts/CLAUDE.md` — operational notifications (crashes, failed cron jobs, panics, traffic/error spikes) delivered to a chat transport. Covers the queue/worker design, crash detection, resource-saturation defaults, edge-triggered rules, Telegram specifics, bounds, privacy and configuration.

## Admin Access Auditing

See `core/audit/CLAUDE.md` — records access to administrative surfaces and raises alerts on intrusion patterns. **On by default.** Covers what personal data is captured and why, the gaps this fills versus PocketBase's own logging, detection triggers, and endpoints.

## Analytics

See `core/analytics/CLAUDE.md` — request middleware counts page views as daily aggregates. **No personal data is persisted.** Covers the zero-DB-work request path, the `auxiliary.db` split, bounded limits against junk-URL floods, visitor tracking, dashboard queries, and template rendering traps.

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
- AST parser files are split by responsibility: `ast.go` (entry points), `ast_func.go` (handler/function analysis), `ast_struct.go` (struct/schema), `ast_metadata.go` (value/type resolution), `ast_file.go` (file utilities)
- Registry is split: `registry.go` (core), `registry_routes.go` (route registration), `registry_spec.go` (OpenAPI spec generation)
