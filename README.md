# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

![image](https://github.com/user-attachments/assets/4466de28-d885-4112-95a9-84dde7f67dc7)

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

## Quick Start

- Set up Your .env file, check env.txt

```go
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/magooney-loon/pb-ext/core/logging"
	"github.com/magooney-loon/pb-ext/core/server"

	"github.com/pocketbase/pocketbase/core"
)

func main() {
	initApp()
}

func initApp() {
	// Create new server instance
	srv := server.New()

	// Setup logging and recovery
	logging.SetupLogging(srv)

	// Setup recovery middleware
	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		logging.SetupRecovery(srv.App(), e)
		return e.Next()
	})

	// Register custom API routes
	registerRoutes(srv.App())

	// Set domain name from environment if specified
	args := []string{"serve"}
	if domain := os.Getenv("PB_SERVER_DOMAIN"); domain != "" {
		args = append(args, domain)
	}
	srv.App().RootCmd.SetArgs(args)

	// Start the server
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

// registerRoutes sets up all custom API routes
func registerRoutes(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Server time route
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
go run cmd/server/main.go
```

## Webmaster Panel

- Admin panel `127.0.0.1:8090/_`
- Server panel `127.0.0.1:8090/_/_`
