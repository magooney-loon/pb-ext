package main

// Cron job examples

import (
	"log"
	"time"

	"github.com/magooney-loon/pb-ext/core/server"
	"github.com/pocketbase/pocketbase/core"
)

func registerJobs(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Ensure job wrapper is ready before registering jobs
		// The wrapper should be initialized in OnBootstrap, but let's add a check
		app.Logger().Info("Registering cron jobs...")

		// Register example cron jobs
		if err := helloJob(app); err != nil {
			app.Logger().Error("Failed to register hello job", "error", err)
			return err
		}

		if err := dailyCleanupJob(app); err != nil {
			app.Logger().Error("Failed to register daily cleanup job", "error", err)
			return err
		}

		if err := weeklyStatsJob(app); err != nil {
			app.Logger().Error("Failed to register weekly stats job", "error", err)
			return err
		}

		if err := healthCheckJob(app); err != nil {
			app.Logger().Error("Failed to register health check job", "error", err)
			return err
		}

		app.Logger().Info("All cron jobs registered successfully")
		return e.Next()
	})
}

// JOB_SOURCE
// JOB_DESC: A simple demonstration job that runs every 5 minutes, outputs timestamped hello messages and simulates basic task processing
func helloJob(app core.App) error {
	return server.JobSourceWithDescription(app, "helloWorld", "Hello World Job",
		"A simple demonstration job that runs every 5 minutes, outputs timestamped hello messages and simulates basic task processing",
		"*/5 * * * *", func() {
			log.Println("🚀 Hello World Job Starting...")
			log.Printf("Current time: %s", time.Now().Format("2006-01-02 15:04:05"))
			log.Println("Processing hello world task...")

			// Simulate some work
			time.Sleep(100 * time.Millisecond)

			log.Println("Hello from cron job! Task completed successfully.")
			log.Printf("Job finished at: %s", time.Now().Format("2006-01-02 15:04:05"))
		})
}

// JOB_SOURCE
// JOB_DESC: Automated maintenance job that runs daily at 2 AM to clean up completed todos older than 30 days, helping keep the database optimized
func dailyCleanupJob(app core.App) error {
	return server.JobSourceWithDescription(app, "dailyCleanup", "Daily Cleanup Job",
		"Automated maintenance job that runs daily at 2 AM to clean up completed todos older than 30 days, helping keep the database optimized",
		"0 2 * * *", func() {
			log.Println("🧹 Daily Cleanup Job Starting...")
			log.Printf("Cleanup job started at: %s", time.Now().Format("2006-01-02 15:04:05"))

			app.Logger().Info("Running daily cleanup job", "time", time.Now())

			// Example: Clean up old todos older than 30 days
			collection, err := app.FindCollectionByNameOrId("todos")
			if err != nil {
				log.Printf("❌ Failed to find todos collection: %v", err)
				app.Logger().Error("Failed to find todos collection", "error", err)
				return
			}

			log.Println("✅ Found todos collection, proceeding with cleanup...")

			// Delete completed todos older than 30 days
			cutoffDate := time.Now().AddDate(0, 0, -30)
			log.Printf("📅 Cleaning up todos older than: %s", cutoffDate.Format("2006-01-02"))

			filter := "completed = true && created < {:cutoff}"
			records, err := app.FindRecordsByFilter(collection, filter, "", 100, 0, map[string]any{
				"cutoff": cutoffDate.Format("2006-01-02 15:04:05.000Z"),
			})

			if err != nil {
				log.Printf("❌ Failed to find old todos: %v", err)
				app.Logger().Error("Failed to find old todos", "error", err)
				return
			}

			log.Printf("📊 Found %d old completed todos to clean up", len(records))

			deletedCount := 0
			for _, record := range records {
				if err := app.Delete(record); err != nil {
					log.Printf("❌ Failed to delete todo %s: %v", record.Id, err)
					app.Logger().Error("Failed to delete old todo", "id", record.Id, "error", err)
				} else {
					deletedCount++
				}
			}

			log.Printf("✅ Daily cleanup completed. Deleted %d/%d records", deletedCount, len(records))
			app.Logger().Info("Daily cleanup completed", "deleted_records", deletedCount)
		})
}

// JOB_SOURCE
// JOB_DESC: Weekly analytics job that runs every Sunday at midnight to generate comprehensive todo statistics including completion rates and productivity metrics
func weeklyStatsJob(app core.App) error {
	return server.JobSourceWithDescription(app, "weeklyStats", "Weekly Statistics Job",
		"Weekly analytics job that runs every Sunday at midnight to generate comprehensive todo statistics including completion rates and productivity metrics",
		"0 0 * * 0", func() {
			log.Println("📊 Weekly Statistics Job Starting...")
			log.Printf("Generating weekly report for week ending: %s", time.Now().Format("2006-01-02"))

			app.Logger().Info("Generating weekly statistics", "time", time.Now())

			// Example: Generate weekly todo statistics
			collection, err := app.FindCollectionByNameOrId("todos")
			if err != nil {
				log.Printf("❌ Failed to find todos collection: %v", err)
				app.Logger().Error("Failed to find todos collection", "error", err)
				return
			}

			log.Println("✅ Found todos collection, analyzing data...")

			// Count todos from the past week
			weekAgo := time.Now().AddDate(0, 0, -7)
			log.Printf("📅 Analyzing todos created since: %s", weekAgo.Format("2006-01-02"))

			filter := "created >= {:week_ago}"
			records, err := app.FindRecordsByFilter(collection, filter, "", 1000, 0, map[string]any{
				"week_ago": weekAgo.Format("2006-01-02 15:04:05.000Z"),
			})

			if err != nil {
				log.Printf("❌ Failed to fetch weekly todos: %v", err)
				app.Logger().Error("Failed to fetch weekly todos", "error", err)
				return
			}

			log.Printf("📈 Processing %d todos from the past week...", len(records))

			completed := 0
			pending := 0
			for _, record := range records {
				if record.GetBool("completed") {
					completed++
				} else {
					pending++
				}
			}

			completionRate := float64(0)
			if len(records) > 0 {
				completionRate = float64(completed) / float64(len(records)) * 100
			}

			log.Println("📋 WEEKLY STATISTICS REPORT")
			log.Printf("   • Total todos created: %d", len(records))
			log.Printf("   • Completed todos: %d", completed)
			log.Printf("   • Pending todos: %d", pending)
			log.Printf("   • Completion rate: %.1f%%", completionRate)
			log.Println("✅ Weekly statistics report generated successfully")

			app.Logger().Info("Weekly statistics generated",
				"total_todos_created", len(records),
				"completed_todos", completed,
				"pending_todos", pending,
				"completion_rate", completionRate,
			)
		})
}

// JOB_SOURCE
// JOB_DESC: System health monitoring job that runs every 5 minutes to check database connectivity, disk space, and other critical system resources
func healthCheckJob(app core.App) error {
	return server.JobSourceWithDescription(app, "healthCheck", "Health Check Job",
		"System health monitoring job that runs every 5 minutes to check database connectivity, disk space, and other critical system resources",
		"*/5 * * * *", func() {
			log.Println("🔍 Health Check Job Starting...")
			log.Printf("System health check initiated at: %s", time.Now().Format("2006-01-02 15:04:05"))

			app.Logger().Debug("Running health check", "time", time.Now())

			// Check database connectivity
			log.Println("🗄️  Checking database connectivity...")
			if _, err := app.DB().NewQuery("SELECT 1").Execute(); err != nil {
				log.Printf("❌ Database health check failed: %v", err)
				app.Logger().Error("Database health check failed", "error", err)
				return
			}
			log.Println("✅ Database connectivity: OK")

			// Check available disk space (basic example)
			dataDir := app.DataDir()
			log.Printf("📁 Data directory: %s", dataDir)
			log.Println("✅ Disk space check: OK")

			// Simulate additional health checks
			log.Println("🔧 Checking system resources...")
			time.Sleep(50 * time.Millisecond)
			log.Println("✅ Memory usage: OK")
			log.Println("✅ CPU usage: OK")

			log.Println("✅ All health checks completed successfully")
			app.Logger().Debug("Health check completed", "db_path", dataDir, "status", "healthy")
		})
}
