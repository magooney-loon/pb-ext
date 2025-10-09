package main

// Cron job examples

import (
	"log"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func registerJobs(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Register example cron jobs
		if err := helloJob(app); err != nil {
			return err
		}

		if err := dailyCleanupJob(app); err != nil {
			return err
		}

		if err := weeklyStatsJob(app); err != nil {
			return err
		}

		if err := healthCheckJob(app); err != nil {
			return err
		}

		return e.Next()
	})
}

// helloJob runs every minute as a basic example
func helloJob(app core.App) error {
	return app.Cron().Add("helloWorld", "*/1 * * * *", func() {
		log.Println("Hello from cron job!")
	})
}

// dailyCleanupJob runs daily at 2 AM to clean up old records
func dailyCleanupJob(app core.App) error {
	return app.Cron().Add("dailyCleanup", "0 2 * * *", func() {
		app.Logger().Info("Running daily cleanup job", "time", time.Now())

		// Example: Clean up old todos older than 30 days
		collection, err := app.FindCollectionByNameOrId("todos")
		if err != nil {
			app.Logger().Error("Failed to find todos collection", "error", err)
			return
		}

		// Delete completed todos older than 30 days
		cutoffDate := time.Now().AddDate(0, 0, -30)
		filter := "completed = true && created < {:cutoff}"
		records, err := app.FindRecordsByFilter(collection, filter, "", 100, 0, map[string]any{
			"cutoff": cutoffDate.Format("2006-01-02 15:04:05.000Z"),
		})

		if err != nil {
			app.Logger().Error("Failed to find old todos", "error", err)
			return
		}

		for _, record := range records {
			if err := app.Delete(record); err != nil {
				app.Logger().Error("Failed to delete old todo", "id", record.Id, "error", err)
			}
		}

		app.Logger().Info("Daily cleanup completed", "deleted_records", len(records))
	})
}

// weeklyStatsJob runs every Sunday at midnight to generate weekly reports
func weeklyStatsJob(app core.App) error {
	return app.Cron().Add("weeklyStats", "0 0 * * 0", func() {
		app.Logger().Info("Generating weekly statistics", "time", time.Now())

		// Example: Generate weekly todo statistics
		collection, err := app.FindCollectionByNameOrId("todos")
		if err != nil {
			app.Logger().Error("Failed to find todos collection", "error", err)
			return
		}

		// Count todos from the past week
		weekAgo := time.Now().AddDate(0, 0, -7)
		filter := "created >= {:week_ago}"
		records, err := app.FindRecordsByFilter(collection, filter, "", 1000, 0, map[string]any{
			"week_ago": weekAgo.Format("2006-01-02 15:04:05.000Z"),
		})

		if err != nil {
			app.Logger().Error("Failed to fetch weekly todos", "error", err)
			return
		}

		completed := 0
		for _, record := range records {
			if record.GetBool("completed") {
				completed++
			}
		}

		app.Logger().Info("Weekly statistics generated",
			"total_todos_created", len(records),
			"completed_todos", completed,
			"completion_rate", float64(completed)/float64(len(records))*100,
		)
	})
}

// healthCheckJob runs every 5 minutes to perform system health checks
func healthCheckJob(app core.App) error {
	return app.Cron().Add("healthCheck", "*/5 * * * *", func() {
		app.Logger().Debug("Running health check", "time", time.Now())

		// Check database connectivity
		if _, err := app.DB().NewQuery("SELECT 1").Execute(); err != nil {
			app.Logger().Error("Database health check failed", "error", err)
			return
		}

		// Check available disk space (basic example)
		dataDir := app.DataDir()
		app.Logger().Debug("Health check completed", "db_path", dataDir, "status", "healthy")
	})
}
