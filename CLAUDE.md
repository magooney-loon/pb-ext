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
2. `OnBootstrap`: initializes JobLogger → JobManager → registers system jobs → JobHandlers
3. `OnServe`: registers health route, analytics, job routes, static file serving
4. `OnTerminate`: closes the analytics collector, flushing buffered counters
5. User code hooks in via `srv.App().OnServe().BindFunc()`

**Key singletons**: `GetJobManager()` returns a package-level `*JobManager` initialized during bootstrap.

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

**Storage**: one `_analytics` row per `(path, date, device_type, browser)` with `views`, `unique_sessions` (sessions started by a new visitor) and `returning_sessions` counters. Deleted after 90 days by the `__pbExtAnalyticsClean__` system job.

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

**Testing**: `testutil.NewAnalytics(t, opts...)` returns an app plus a running collector (closed on cleanup); `testutil.AnalyticsTotals(t, app)` reads persisted counters. Pass `WithFlushInterval(time.Hour)` to observe buffering and flush by hand. Internals (`aggregator`, `visitorTracker`) are tested white-box in package `analytics`; anything needing `testutil` must live in package `analytics_test` to avoid an import cycle.

## Example App Patterns

`cmd/server/` is the canonical reference for how to integrate pb-ext:
- `routes.go` — how to initialize versioned API routers and register routes
- `handlers.go` — how to use `API_SOURCE`, `API_DESC`, `API_TAGS` directives and define request/response types
- `jobs.go` — how to register cron jobs with `GetJobManager().RegisterJob`
- `collections.go` — how to define PocketBase collections programmatically

## Conventions

- The `core/` package is the library; `cmd/server/` is the example app showing how to use it
- Server options use the functional options pattern (`WithConfig`, `WithPocketbase`, `InDeveloperMode`)
- PocketBase system collections prefixed with `_` (e.g., `_analytics`, `_job_logs`)
- Dashboard templates use Go `text/template` with `embed.FS`
- Module path: `github.com/magooney-loon/pb-ext`
- AST parser files are split by responsibility: `ast.go` (entry points), `ast_func.go` (handler/function analysis), `ast_struct.go` (struct/schema), `ast_metadata.go` (value/type resolution), `ast_file.go` (file utilities)
- Registry is split: `registry.go` (core), `registry_routes.go` (route registration), `registry_spec.go` (OpenAPI spec generation)
