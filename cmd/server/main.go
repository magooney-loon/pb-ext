package main

import (
	"log"
	"os"

	"magooney-loon/pb-ext/internal/logging"
	"magooney-loon/pb-ext/internal/server"
	"magooney-loon/pb-ext/pkg/api"

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

	// Set server address from environment if specified
	// This is useful for tests to avoid port conflicts
	if addr := os.Getenv("PB_SERVER_ADDR"); addr != "" {
		srv.App().RootCmd.SetArgs([]string{"serve", "--http=" + addr})
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
