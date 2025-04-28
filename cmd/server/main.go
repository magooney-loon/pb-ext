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
	RegisterRoutes(srv.App())

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

// RegisterRoutes sets up all custom API routes
func RegisterRoutes(app core.App) {
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
		// e.Router.GET("/api/your-endpoint", yourHandler)
		// e.Router.POST("/api/another-endpoint", anotherHandler)

		return e.Next()
	})
}
