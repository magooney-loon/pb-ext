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
- [Core Overview](core/README.md) - Core implementation modules

## Quick Start

- Set up Your .env file, check env.txt

```go
package main

import (
	"log"
	"net/http"
	"os"
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
	if domain := os.Getenv("PB_SERVER_DOMAIN"); domain != "" {
		srv.App().RootCmd.SetArgs([]string{"serve", "--domain", domain})
	} else {
		srv.App().RootCmd.SetArgs([]string{"serve"})
	}

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
		// Example utility route
		e.Router.GET("/api/utils/time", func(c *core.RequestEvent) error {
			now := time.Now()
			return c.JSON(http.StatusOK, map[string]interface{}{
				"timestamp": now.Unix(),
				"iso8601":   now.Format(time.RFC3339),
				"rfc822":    now.Format(time.RFC822),
				"date":      now.Format("2006-01-02"),
				"time":      now.Format("15:04:05"),
			})
		})

		// Add your custom routes here
		// Example:
		// e.Router.POST("/api/another-endpoint", anotherHandler)

		return e.Next()
	})
}
```

```bash
go run cmd/server/main.go
```

```bash
go test ./... -v
```

## Webmaster Panel

- Admin panel `127.0.0.1:8090/_`
- Server panel `127.0.0.1:8090/_/_`
