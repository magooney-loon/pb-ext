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
		// Index route, served from ./pb_public, visitor tracking
		e.Router.GET("/", func(c *core.RequestEvent) error {
			return c.JSON(http.StatusOK, map[string]any{
				"message": "Welcome to the API",
				"version": "1.0.0",
				"time":    time.Now().Format(time.RFC3339),
			})
		})

		// e.Router.POST("/api/another-endpoint", anotherHandler)

		return e.Next()
	})
}
