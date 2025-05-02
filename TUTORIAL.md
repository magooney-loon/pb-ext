# Beginner's Guide to pb-ext

This guide will help you get started with pb-ext even if you're new to Golang.

## What is pb-ext?

pb-ext is a wrapper around PocketBase that adds enhanced monitoring, logging, and analytics features. Since it builds on top of PocketBase, you can continue using all PocketBase features and documentation for extending it as a Go framework.

## Prerequisites

1. Install Golang:
   - Download from [golang.org/dl](https://go.dev/dl/)
   - Follow the installation instructions for your OS
   - Verify installation with `go version`

## Step-by-Step Setup

### 1. Create a project folder

```bash
mkdir my-pb-project
cd my-pb-project
```

### 2. Initialize Go module

```bash
go mod init my-pb-project
```

### 3. Create main.go file

Create a file named `main.go` in your project root and copy the following code:

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

### 4. Download dependencies

```bash
go mod tidy
```

This will download pb-ext and all required dependencies.

### 5. Run your server

```bash
go run . serve
```

Your server should now be running!

## Access your application

- PocketBase Admin panel: `http://127.0.0.1:8090/_`
- pb-ext Dashboard: `http://127.0.0.1:8090/_/_`
- Default example route: `http://127.0.0.1:8090/api/time`

## Next Steps

Now that you have pb-ext running, you can:

1. Use the PocketBase Admin UI to create collections and manage your data
2. Use the pb-ext dashboard to monitor your server health and visitor analytics
3. Extend your application with additional routes and functionality

For more information on using PocketBase as a Go framework, refer to the [PocketBase documentation](https://pocketbase.io/docs/go-overview/). 