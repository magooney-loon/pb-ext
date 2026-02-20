# Core Library Refactoring Plan

## Overview

This document covers the refactoring plan for `core/` (excluding `core/server/api/`, see `API.md`).

**Goal**: Break up the `core/server/` god package into focused sub-packages.

**Status**: 🔄 In Progress

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Extract Jobs Package (`core/jobs/`) | ✅ Done |
| Phase 2 | Extract Analytics Package (`core/analytics/`) | ✅ Done |
| Phase 3 | Simplify Server Package | ⏳ Pending |

---

## Conventions

- **All pb-ext PocketBase collections are system collections**: use `NewBaseCollection` + `col.System = true`. This prevents rename, deletion, and rules changes from the admin UI.
- **No deprecated shims**: when a subsystem moves to a new package, the old call sites update to the new import. No re-exports, no compatibility wrappers.
- **No backward compatibility as a goal**: `cmd/server/` is the example app and gets updated alongside the library. It is not a frozen public API.
- **Tests are written after refactoring is complete**, not during. Don't port old tests to new packages mid-refactor.

---

## Architecture (Current State)

```
core/
├── core.go                 # Re-export facade
├── logging/                # ✅ Unchanged
├── monitoring/             # ✅ Unchanged
├── jobs/                   # ✅ Phase 1 complete
│   ├── types.go            # All shared types (JobMetadata, LogsData, etc.)
│   ├── collection.go       # _job_logs system collection setup
│   ├── logger.go           # Logger, ExecutionLogger — buffered PB persistence
│   ├── manager.go          # Manager, Initialize(), GetManager()
│   └── handlers.go         # HTTP handlers for job API routes
├── analytics/              # ✅ Phase 2 complete
│   ├── types.go            # PageView, Data, PageStat, RecentVisit, constants
│   ├── collection.go       # _analytics system collection setup
│   ├── collector.go        # Middleware, bot detection, UA parsing, UTM extraction
│   ├── storage.go          # Buffer flush, background workers, aggregate query
│   └── analytics.go        # Analytics struct, New(), Initialize()
└── server/                 # ⚠️ Phase 3 pending
    ├── server.go           # Server struct, lifecycle — uses core/jobs + core/analytics
    ├── server_options.go   # Functional options
    ├── health.go           # Dashboard handler — kept here
    ├── errors.go           # Error types — kept here
    └── templates.go        # embed.FS
```

---

## Phase 1: Jobs — ✅ Complete

**What moved**: `job_manager.go`, `job_logger.go`, `job_handlers.go`, `job_logs.go` → `core/jobs/`

**Key decisions made**:
- `ExecutionLogger` (was `JobExecutionLogger`) is the type passed to job functions
- `Manager` (was `JobManager`) holds the registry and wraps PocketBase cron
- `Logger` (was `JobLogger`) handles buffered persistence to `_job_logs`
- `Initialize(app)` sets up the collection, starts background workers, sets the global, returns `*Manager`
- `GetManager()` returns the global — called from `cmd/server/jobs.go`
- All routes (both `/api/cron/*` and legacy `/api/joblogs/*`) registered in `handlers.go`
- `_job_logs` is a system collection (`col.System = true`)

**Call sites updated**: `cmd/server/jobs.go` imports `core/jobs` directly.

---

## Phase 2: Extract Analytics — ✅ Complete

**What moved**: `core/server/analytics.go` → `core/analytics/`

**Key decisions made**:
- `Analytics` struct holds buffer, flush channel, visitor session map
- `Initialize(app core.App)` — sets up collection, starts background workers, returns `*Analytics`
- `Data` (was `AnalyticsData`), `DefaultData()` (was `defaultAnalyticsData()`)
- `GetData()` (was `GetAnalyticsData()`) — flushes pending then aggregates from DB
- `RegisterRoutes(e)` — attaches tracking middleware (bot filter, static exclusion, UTM extraction)
- `_analytics` is a system collection (`col.System = true`)
- `server.go` calls `analytics.Initialize(app)` in `OnServe`; `health.go` uses `*analytics.Data`

---

## Phase 3: Simplify Server Package — ⏳ Pending

After Phase 2, `core/server/server.go` should be reduced to:
- Wrap `*pocketbase.PocketBase`
- Track `ServerStats` (requests, connections, uptime)
- Orchestrate `OnBootstrap` and `OnServe` hooks
- Register health route and static file serving

Files remaining in `core/server/` after Phase 3:
```
server.go           # ~150 lines
server_options.go   # Unchanged
health.go           # Health dashboard
errors.go           # ServerError types
templates.go        # embed.FS
```

---

## What We're NOT Doing

- **Not** creating `core/templates/` — template embed is 3 lines
- **Not** creating `core/errors/` — low ROI at this scale
- **Not** creating `core/health/` — health is tightly coupled to server stats, small file
- **Not** moving `core/server/api/` — separate plan in `API.md`

---

## Success Criteria

- [ ] `go build ./...` clean at every phase boundary
- [ ] `core/server/server.go` under 200 lines after Phase 3
- [ ] Each extracted package independently importable
- [ ] No circular dependencies
- [ ] All pb-ext collections are system collections

---

**Last Updated**: 2026-02-20
