# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

<img width="3840" height="2160" alt="pb-ext" src="https://github.com/user-attachments/assets/155cacab-9a2a-4ea0-84b5-137209a4512b" />

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/magooney-loon/pb-ext)

## Core Features

- **API Schema**: Automatic endpoint openapi style docs UI generation
- **System Monitoring**: Real-time metrics for CPU, memory, disk, network, and runtime stats
- **Structured Logging**: Comprehensive logging with error tracking and request tracing
- **Visitor Analytics**: Track and analyze visitor statistics, page views, device types, and browsers
- **PocketBase Integration**: Seamlessly piggybacks off PocketBase's superuser authentication and CSS

## Access

- Admin panel: `127.0.0.1:8090/_`
- Server dashboard: `127.0.0.1:8090/_/_`
- API Schema: `127.0.0.1:8090/api/docs/openapi`

## Authentication

The dashboard utilizes PocketBase's superuser authentication system, ensuring that only authorized administrators can access the monitoring and analytics features.


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
	var srv *app.Server
	if devMode {
		srv = app.New(app.InDeveloperMode())
		log.Println("🔧 Developer mode enabled")
	} else {
		srv = app.New(app.InNormalMode())
		log.Println("🚀 Production mode")
	}

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

// Example routes in cmd/server/routes.go
// Example handlers in cmd/server/handlers.go
// Example cron jobs in cmd/server/jobs.go
// Example collections in cmd/server/collections.go
//
// You can restructure Your project as you wish,
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
