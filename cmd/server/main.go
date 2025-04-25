package main

import (
	"log"

	"magooney-loon/pb-ext-dash/internal/logging"
	"magooney-loon/pb-ext-dash/internal/server"

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

	// Start the server
	if err := srv.Start(); err != nil {
		srv.App().Logger().Error("Fatal application error",
			"error", err,
			"uptime", srv.Stats().StartTime,
			"total_requests", srv.Stats().TotalRequests.Load(),
		)
		log.Fatal(err)
	}
}
