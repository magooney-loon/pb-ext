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
