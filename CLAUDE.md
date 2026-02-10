# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

pb-ext is a Go library that wraps PocketBase with production-ready features: auto-generated OpenAPI docs, cron job tracking, system monitoring, structured logging, and visitor analytics. It includes a superuser dashboard at `/_/_`.

Users import `github.com/magooney-loon/pb-ext/core` and build their server in `cmd/server/`.

## Build & Run Commands

All operations go through the custom Go build toolchain — there is no Makefile.

| Command | Purpose |
|---|---|
| `go run cmd/scripts/main.go` | Build frontend + start dev server |
| `go run cmd/scripts/main.go --run-only` | Start server only (skip frontend build) |
| `go run cmd/scripts/main.go --build-only` | Build frontend to `pb_public/`, no server |
| `go run cmd/scripts/main.go --install` | Install all deps, then build + run |
| `go run cmd/scripts/main.go --production` | Full production build to `dist/` |
| `go run cmd/scripts/main.go --test-only` | Run tests with coverage reports |
| `go test ./...` | Run all Go tests directly |
| `go test ./core/server/api/...` | Run tests for a specific package |

The dev server runs at `127.0.0.1:8090` by default. PocketBase admin: `/_/`, pb-ext dashboard: `/_/_`.

## Architecture

```
core/core.go          — Public facade, re-exports from core/server and core/logging
core/server/          — Core server: Server struct, analytics, jobs, health dashboard, templates
core/server/api/      — OpenAPI doc system: registry, versioned routers, Go AST parsing
core/logging/         — Structured logging, request middleware, trace IDs
core/monitoring/      — System metrics (CPU, memory, disk, network, runtime)
cmd/server/           — Example application (user's app entry point)
cmd/scripts/          — Build toolchain CLI
core/server/templates/ — Embedded Go templates for the dashboard UI
```

**Server lifecycle** (`core/server/server.go`):
1. `New(opts...)` creates a Server wrapping PocketBase
2. `OnBootstrap`: initializes JobLogger → JobManager → registers system jobs → JobHandlers
3. `OnServe`: registers health route, analytics, job routes, static file serving
4. User code hooks in via `srv.App().OnServe().BindFunc()`

**Key singletons**: `GetJobManager()` returns a package-level `*JobManager` initialized during bootstrap.

## OpenAPI Documentation System

The API doc system uses Go AST parsing at startup to extract endpoint metadata:

- Handler files must have `// API_SOURCE` comment at the top of the file
- Individual handlers use `// API_DESC <description>` and `// API_TAGS <tag>` comments
- Request/response types are reflected from Go structs in the same file
- Routes are registered through `api.VersionedAPIRouter` which wraps PocketBase's router
- Debug endpoint: `/api/docs/debug/ast`

## Cron Jobs

Jobs are registered via `server.GetJobManager().RegisterJob(id, name, desc, cronExpr, func(*JobExecutionLogger))`. The `JobExecutionLogger` provides structured logging methods: `Start`, `Info`, `Progress`, `Success`, `Error`, `Statistics`, `Complete`, `Fail`. Execution logs are stored in the `_job_logs` PocketBase collection and auto-purged after 72 hours.

## Analytics

Request middleware captures visitor data (user agent, device, browser, UTM params). Records are buffered in memory and flushed every 10 minutes (or at 50 items) to the `_analytics` PocketBase collection.

## Conventions

- The `core/` package is the library; `cmd/server/` is the example app showing how to use it
- Server options use the functional options pattern (`WithConfig`, `WithPocketbase`, `InDeveloperMode`)
- PocketBase system collections prefixed with `_` (e.g., `_analytics`, `_job_logs`)
- Dashboard templates use Go `text/template` with `embed.FS`
- Module path: `github.com/magooney-loon/pb-ext`
