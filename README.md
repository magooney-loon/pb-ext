# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

<img width="3830" height="1237" alt="pb-ext" src="https://github.com/user-attachments/assets/6965fd9c-2983-41db-a4f2-4c24f5739dad" />

## Architecture

```
├── cmd/
│   └── server/          # Server initialization
└── core/
    ├── logging/         # Logging and error handling
    ├── monitoring/      # System metrics collection
    └── server/          # Core server implementation

```

## Core Features

- **API Schema**: Automatic discovery openapi style and docs generation
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
	app "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	devMode := flag.Bool("dev", false, "Run in developer mode")
	flag.Parse()

	// Check environment variable as fallback
	if !*devMode && strings.ToLower(os.Getenv("DEV")) == "true" {
		*devMode = true
	}

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

// Check cmd/server/main.go
	// For the full example
```

```bash
go run cmd/scripts/main.go --run-only
```
