# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

![image](https://github.com/user-attachments/assets/e76f4a7d-f309-4ce4-b01a-f1e972c8289c)

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

- **System Monitoring**: Real-time metrics for CPU, memory, disk, network, and runtime stats
- **Structured Logging**: Comprehensive logging with error tracking and request tracing
- **Visitor Analytics**: Track and analyze visitor statistics, page views, device types, and browsers
- **PocketBase Integration**: Seamlessly piggybacks off PocketBase's superuser authentication and CSS

## Quick Start

> 🆕 New to Golang and/or PocketBase? [Read this beginner tutorial](TUTORIAL.md).

```go
package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	app "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	initApp()
}

func initApp() {
	srv := app.New()

	app.SetupLogging(srv)

	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.SetupRecovery(srv.App(), e)
		return e.Next()
	})

	registerRoutes(srv.App())

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

func registerRoutes(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.GET("/api/time", func(c *core.RequestEvent) error {
			now := time.Now()
			return c.JSON(http.StatusOK, map[string]any{
				"time": map[string]string{
					"iso":       now.Format(time.RFC3339),
					"unix":      strconv.FormatInt(now.Unix(), 10),
					"unix_nano": strconv.FormatInt(now.UnixNano(), 10),
					"utc":       now.UTC().Format(time.RFC3339),
				},
			})
		})

		return e.Next()
	})
}
```

```bash
go run cmd/server/main.go serve
```

## Dashboard

The enhanced dashboard provides a comprehensive view of your server's health and visitor analytics:

- **Health Tab**: Monitor system metrics including CPU, memory, and network usage
- **Analytics Tab**: Track visitor statistics, page views, and user behavior

## Access

- Admin panel: `127.0.0.1:8090/_`
- Server dashboard: `127.0.0.1:8090/_/_`

## Authentication

The dashboard utilizes PocketBase's superuser authentication system, ensuring that only authorized administrators can access the monitoring and analytics features.
