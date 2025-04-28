package main

import (
	"log"
	"net/http"
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
