# pb-ext

Enhanced PocketBase server with monitoring, logging & API docs.

<img width="3840" height="2160" alt="pb-ext" src="https://github.com/user-attachments/assets/af360704-c3d6-4d1f-9b49-80229d6570d2" />
<img width="1920" height="2153" alt="Screenshot_2026-02-10_14-42-37" src="https://github.com/user-attachments/assets/d74cf16e-7b5a-4bd0-9f73-1ea81b8c175c" />
<img width="1656" height="1645" alt="Screenshot_2026-02-20_18-19-39" src="https://github.com/user-attachments/assets/ee4062e3-f9f1-4868-8f17-002c4f1c9f11" />



[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/magooney-loon/pb-ext)

## Core Features

- **API Schema**: Auto-generates OpenAPI docs UI for your endpoints
- **Cron Tracking**: Logs and monitors scheduled cron jobs
- **System Monitoring**: Real-time CPU, memory, disk, network, and runtime metrics
- **Structured Logging**: Complete logging with error tracking and request tracing
- **Visitor Analytics**: Track GDPR compliant visitors, page views, device types, and browsers
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
## Quick Start

> 🆕 New to Golang and/or PocketBase? [Read this beginner tutorial](TUTORIAL.md).

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
	flag.Parse()

	initApp(*devMode)
}

func initApp(devMode bool) {
	var opts []app.Option

	if devMode {
		opts = append(opts, app.InDeveloperMode())
	} else {
		opts = append(opts, app.InNormalMode())
	}

	srv := app.New(opts...)

	app.SetupLogging(srv)

	registerCollections(srv.App())
	registerRoutes(srv.App())
	registerJobs(srv.App())

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
//
// You can restructure Your project as You wish,
// just keep this main.go in cmd/server/main.go
//
// Need a pre-built Svelte5Kit starter template?
// https://github.com/magooney-loon/svelte-gui
//
// Ready for a production build deployment?
// https://github.com/magooney-loon/pb-deployer
```

```bash
go mod tidy
go run cmd/scripts/main.go --run-only
```

See `**/*/README.md` for detailed docs.

## OpenAPI Spec Generation & Embedding

pb-ext now supports build-time OpenAPI spec generation with embedded runtime loading.

### Generate specs manually

```bash
go run ./cmd/server --generate-specs-dir ./core/server/api/specs
```

Optional: generate only one version:

```bash
go run ./cmd/server --generate-specs-dir ./core/server/api/specs --generate-spec-version v1
```

### Validate generated specs

```bash
go run ./cmd/server --validate-specs-dir ./core/server/api/specs
```

### Runtime behavior

- At runtime, docs loading is **embedded-first** by version (`v1`, `v2`, etc.).
- If an embedded spec isn't available or doesn't match runtime config checks, pb-ext falls back to runtime generation.
- For local/debug overrides, set:

```bash
PB_EXT_OPENAPI_SPECS_DIR=/absolute/path/to/specs
```

When set, specs from that directory are preferred for lookup.

### Build pipeline integration

The script pipeline runs OpenAPI generation + validation automatically before server compilation in relevant modes:

```bash
go run cmd/scripts/main.go
go run cmd/scripts/main.go --build-only
go run cmd/scripts/main.go --production
```

Having issues with Your API Docs?
```bash
127.0.0.1:8090/api/docs/debug/ast
```

## Reserved Collections

pb-ext creates the following PocketBase system collections automatically on startup. **Do not create collections with these names in your own code.**

| Collection | Purpose |
|---|---|
| `_analytics` | Daily aggregated page view counters (one row per path/date/device/browser). Retention: 90 days. |
| `_analytics_sessions` | Ring buffer of the 50 most recent visits for the Recent Activity display. No PII stored. |
| `_job_logs` | Cron job execution logs (start time, end time, duration, status, output). Retention: 72 hours. |

**Schema notes:**
- All three collections are system collections (hidden from the PocketBase Collections UI).
- `_analytics` and `_analytics_sessions` store no personal data — no IP, no user agent, no visitor ID. GDPR-compliant by design.
- On upgrade from an old pb-ext version, incompatible schemas are automatically migrated at startup with no manual steps required.

## Reserved Routes

pb-ext registers the following routes. **Do not register your own routes at these paths.**

### Dashboard
| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/_/_` | Superuser | pb-ext health, analytics & jobs dashboard |

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

### API Docs
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/docs/versions` | List registered API versions |
| `GET` | `/api/docs/debug/ast` | AST parsing debug info |
| `GET` | `/api/docs/v{n}` | Version metadata |
| `GET` | `/api/docs/v{n}/openapi.json` | OpenAPI 3.0 spec |
| `GET` | `/api/docs/v{n}/swagger` | Swagger UI |

### Internal System Jobs

pb-ext registers these cron jobs automatically. They appear in the dashboard with the "System" badge.

| Job ID | Schedule | Description |
|---|---|---|
| `__pbExtLogClean__` | `0 0 * * *` (daily midnight) | Purge `_job_logs` records older than 72 hours |
| `__pbExtAnalyticsClean__` | `0 3 * * *` (daily 3 AM) | Purge `_analytics` rows older than 90 days |
