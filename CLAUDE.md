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
| `go test ./core/analytics/ -run TestStress -v` | Analytics sustained-load tests (skipped by `-short`) |

The dev server runs at `127.0.0.1:8090` by default. PocketBase admin: `/_/`, pb-ext dashboard: `/_/_`.

## Architecture

```
core/core.go          — Public facade, re-exports from core/server and core/logging
core/server/          — Server struct, health dashboard, errors, embedded templates
core/server/api/      — OpenAPI doc system: registry, versioned routers, Go AST parsing
core/analytics/       — Visitor analytics: collector (request path), aggregator (in-memory counters), visitors (session tracking), storage (dashboard queries), collection, types
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
2. `OnBootstrap`: applies pb-ext migrations → initializes JobLogger → JobManager → registers system jobs → JobHandlers
3. `OnServe`: registers health route, analytics, job routes, static file serving
4. `OnTerminate`: closes the analytics collector, flushing buffered counters
5. User code hooks in via `srv.App().OnServe().BindFunc()`

**Key singletons**: `GetJobManager()` returns a package-level `*JobManager` initialized during bootstrap.

## Schema Migrations

pb-ext's system collections are created by PocketBase migrations, not by imperative setup code.

| Migration | Creates |
|---|---|
| `1780000000_pbext_jobs.go` | `_job_logs` |
| `1780000001_pbext_analytics.go` | `_analytics` |

Each package registers its migration into `core.AppMigrations` from an `init()`, so importing `core/jobs` or `core/analytics` is enough — `apis.Serve` runs `RunAllMigrations()` before building the router, and `tests.NewTestApp` runs them too (which is why `testutil.NewTestApp` needs no extra setup).

**Two non-obvious constraints, both covered by tests — don't break them:**

1. **Migration file names must stay `<timestamp>_pbext_<name>.go`.** PocketBase keys applied migrations on `filepath.Base` alone, with no import path. A plain name like `migrations.go` would silently collide with an app's own file of the same name and be skipped. The timestamp keeps pb-ext ordered after PocketBase's system migrations (newest is `1778828400`) and before anything an app generates today. Names are passed explicitly via `core.Migration.File` rather than derived from `runtime.Caller`.

2. **`Server.Start` applies pb-ext's migrations during `OnBootstrap`** (`core/server/migrations.go`), because PocketBase only runs `core.AppMigrations` at the start of `apis.Serve` — *after* `OnBootstrap`. The job manager is built during bootstrap since user code expects `GetManager()` to work from its own `OnServe` hooks, which are registered before `srv.Start()` and therefore run first. Only pb-ext's own migrations are applied early; the app's run at their normal time. Applying them early records them in `_migrations`, so the later `RunAllMigrations` pass is a no-op.

`Initialize` in both packages verifies its collection exists and returns an error naming the missing migration, rather than failing obscurely later.

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

## Analytics

Request middleware counts page views as daily aggregates. **No personal data is persisted** — no IP, user agent, or visitor ID ever reaches the database.

**The request path does zero database work.** A tracked view folds into an in-memory counter map, a recent-visit ring, and a minute bucket under one short mutex (~500ns, 1 alloc). A background goroutine writes accumulated deltas every `FlushInterval` (default 10s) in a single batched transaction, so throughput does not depend on PocketBase's single-writer connection (`NonconcurrentDB()` is capped at 1 connection). Sustained: ~1.8M views/sec with zero loss.

Never add per-request database work to `Track` — that is the exact bottleneck this design exists to remove.

**Storage**: one `_analytics` row per `(path, date, device_type, browser)` with `views`, `unique_sessions` (sessions started by a new visitor) and `returning_sessions` counters. Schema lives in `collection.go`; the migration in `migrations.go` applies it. Deleted after 90 days by the `__pbExtAnalyticsClean__` system job.

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

**Dashboard template** (`core/server/templates/components/visitor_analytics.tmpl`): the only consumer of `analytics.Data` — there is no JSON endpoint for it, so display limits belong in the template, not in `GetData`. `RecentVisits` carries the full 50-entry ring but the card renders 8; `TopPages` carries 10 and renders 5. Template funcs live in `templateFuncs` (`health.go`); `pathLabel` renders the `/*` overflow bucket as "other pages". `core/server/templates_test.go` parses every template and renders this one against populated and empty data, which is the only place a missing template func gets caught (at runtime it is merely logged).

**Testing**: `testutil.NewTestApp(t)` already has both pb-ext collections (migrations run automatically). `testutil.NewAnalytics(t, opts...)` returns an app plus a running collector (closed on cleanup); `testutil.AnalyticsTotals(t, app)` reads persisted counters. Pass `WithFlushInterval(time.Hour)` to observe buffering and flush by hand. Internals (`aggregator`, `visitorTracker`) are tested white-box in package `analytics`; anything needing `testutil` must live in package `analytics_test` to avoid an import cycle.

## Example App Patterns

`cmd/server/` is the canonical reference for how to integrate pb-ext:
- `routes.go` — how to initialize versioned API routers and register routes
- `handlers.go` — how to use `API_SOURCE`, `API_DESC`, `API_TAGS` directives and define request/response types
- `jobs.go` — how to register cron jobs with `GetJobManager().RegisterJob`
- `collections.go` — how to define PocketBase collections programmatically

## Conventions

- The `core/` package is the library; `cmd/server/` is the example app showing how to use it
- Server options use the functional options pattern (`WithConfig`, `WithPocketbase`, `InDeveloperMode`)
- PocketBase system collections prefixed with `_` (e.g., `_analytics`, `_job_logs`), created via registered migrations
- Schema changes go in a **new** migration file; never mutate an already-released one
- Dashboard templates use Go `text/template` with `embed.FS`
- Module path: `github.com/magooney-loon/pb-ext`
- AST parser files are split by responsibility: `ast.go` (entry points), `ast_func.go` (handler/function analysis), `ast_struct.go` (struct/schema), `ast_metadata.go` (value/type resolution), `ast_file.go` (file utilities)
- Registry is split: `registry.go` (core), `registry_routes.go` (route registration), `registry_spec.go` (OpenAPI spec generation)
