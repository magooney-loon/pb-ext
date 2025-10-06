package main

// Cron job example

import (
	"log"

	"github.com/pocketbase/pocketbase/core"
)

func registerJobs(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := helloJob(app); err != nil {
			return err
		}

		return e.Next()
	})
}

func helloJob(app core.App) error {
	return app.Cron().Add("helloWorld", "*/1 * * * *", func() {
		log.Println("Hello from cron job!")
	})
}
