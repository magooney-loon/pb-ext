package main

// Cron job definitions and registration for the pb-ext server

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
)

// registerJobs sets up all cron jobs for the application
func registerJobs(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := helloJob(app); err != nil {
			return err
		}

		return e.Next()
	})
}

// helloJob registers a simple hello world cron job that runs every minute
func helloJob(app core.App) error {
	return app.Cron().Add("helloWorld", "*/1 * * * *", func() {
		log.Println("Hello from cron job!")
	})
}
