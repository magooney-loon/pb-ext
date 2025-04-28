# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

![image](https://github.com/user-attachments/assets/4466de28-d885-4112-95a9-84dde7f67dc7)

## Architecture

```
├── cmd/
│   └── server/          # Server initialization
├── core/
│   ├── logging/         # Logging and error handling
│   ├── monitoring/      # System metrics collection 
│   └── server/          # Core server implementation
├── pkg/
│   └── api/             # Custom API endpoints
```

## Core Features

- **System Monitoring**: Real-time metrics for CPU, memory, disk, network, and runtime stats
- **Structured Logging**: Comprehensive logging with error tracking and request tracing
- **API Group Endpoints**: Custom example endpoints:
  - `/api/utils/time`: Server time
- [Core Overview](core/README.md) - Core implementation modules

## Quick Start

```go
package main

import (
	"log"
	"os"

	"github.com/magooney-loon/pb-ext/core/logging"
	"github.com/magooney-loon/pb-ext/core/server"
	"github.com/magooney-loon/pb-ext/pkg/api"

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
	api.RegisterRoutes(srv.App())

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
