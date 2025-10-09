# pb-ext

Enhanced PocketBase server with monitoring, logging & API docs.

<img width="3840" height="2160" alt="pb-ext" src="https://github.com/user-attachments/assets/c0872112-01a7-48fa-8118-604ba79973ed" />

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/magooney-loon/pb-ext)

## Core Features

- **API Schema**: Auto-generates OpenAPI docs UI for your endpoints
- **System Monitoring**: Real-time CPU, memory, disk, network, and runtime metrics
- **Structured Logging**: Complete logging with error tracking and request tracing
- **Visitor Analytics**: Track visitor stats, page views, device types, and browsers
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
// Consider using the cmd/scripts commands for
// streamlined fullstack dx with +Svelte5kit+
//
// Ready for a production build deployment?
// https://github.com/magooney-loon/pb-deployer
```

```bash
go mod tidy
go run cmd/scripts/main.go --run-only
```

See `**/*/README.md` for detailed docs.

Having issues with Your API Docs?
```bash
127.0.0.1:8090/api/docs/debug/ast
```
