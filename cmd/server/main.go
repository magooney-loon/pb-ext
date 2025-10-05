package main

import (
	"flag"
	"log"

	app "github.com/magooney-loon/pb-ext/core"

	"github.com/pocketbase/pocketbase/core"
)

func main() {
	devMode := flag.Bool("dev", false, "Run in developer mode")
	flag.Parse()

	initApp(*devMode)
}

func initApp(devMode bool) {
	var srv *app.Server
	if devMode {
		srv = app.New(app.InDeveloperMode())
		log.Println("🔧 Developer mode enabled")
	} else {
		srv = app.New(app.InNormalMode())
		log.Println("🚀 Production mode")
	}

	app.SetupLogging(srv)

	registerCollections(srv.App())
	registerRoutes(srv.App())
	registerJobs(srv.App())

	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.SetupRecovery(srv.App(), e)
		return e.Next()
	})

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

// Example handlers in cmd/server/handlers.go
// Example cron jobs in cmd/server/jobs.go
// Example collections in cmd/server/collections.go
//
// You can restructure Your project as you wish,
// just keep this main.go in cmd/server/main.go
//
// Highly advised to copy the cmd/scripts folder
//
// Ready for a production build on a VPS?
// pb-deployer > https://github.com/magooney-loon/pb-deployer
