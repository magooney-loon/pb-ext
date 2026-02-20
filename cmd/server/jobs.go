package main

import (
	"fmt"
	"time"

	"github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/pocketbase/pocketbase/core"
)

func registerJobs(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {

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

		app.Logger().Info("All cron jobs registered successfully")
		return e.Next()
	})
}

func helloJob(app core.App) error {
	jm := jobs.GetManager()
	if jm == nil {
		return fmt.Errorf("job manager not initialized")
	}
	return jm.RegisterJob("helloWorld", "Hello World Job",
		"A simple demonstration job that runs every 5 minutes, outputs timestamped hello messages and simulates basic task processing",
		"*/5 * * * *", func(el *jobs.ExecutionLogger) {
			el.Start("Hello World Job")
			el.Info("Current time: %s", time.Now().Format("2006-01-02 15:04:05"))
			el.Progress("Processing hello world task...")

			// Simulate some work
			time.Sleep(100 * time.Millisecond)

			el.Success("Hello from cron job! Task completed successfully.")
			el.Complete(fmt.Sprintf("Job finished at: %s", time.Now().Format("2006-01-02 15:04:05")))
		})
}

func dailyCleanupJob(app core.App) error {
	jm := jobs.GetManager()
	if jm == nil {
		return fmt.Errorf("job manager not initialized")
	}
	return jm.RegisterJob("dailyCleanup", "Daily Cleanup Job",
		"Automated maintenance job that runs daily at 2 AM to clean up completed todos older than 30 days, helping keep the database optimized",
		"0 2 * * *", func(el *jobs.ExecutionLogger) {
			el.Start("Daily Cleanup Job")
			el.Info("Cleanup job started at: %s", time.Now().Format("2006-01-02 15:04:05"))

			app.Logger().Info("Running daily cleanup job", "time", time.Now())

			collection, err := app.FindCollectionByNameOrId("todos")
			if err != nil {
				el.Error("Failed to find todos collection: %v", err)
				app.Logger().Error("Failed to find todos collection", "error", err)
				el.Fail(err)
				return
			}

			el.Success("Found todos collection, proceeding with cleanup...")

			cutoffDate := time.Now().AddDate(0, 0, -30)
			el.Info("Cleaning up todos older than: %s", cutoffDate.Format("2006-01-02"))

			filter := "completed = true && created < {:cutoff}"
			records, err := app.FindRecordsByFilter(collection, filter, "", 100, 0, map[string]any{
				"cutoff": cutoffDate.Format("2006-01-02 15:04:05.000Z"),
			})
			if err != nil {
				el.Error("Failed to find old todos: %v", err)
				app.Logger().Error("Failed to find old todos", "error", err)
				el.Fail(err)
				return
			}

			el.Info("Found %d old completed todos to clean up", len(records))

			deletedCount := 0
			for _, record := range records {
				if err := app.Delete(record); err != nil {
					el.Error("Failed to delete todo %s: %v", record.Id, err)
					app.Logger().Error("Failed to delete old todo", "id", record.Id, "error", err)
				} else {
					deletedCount++
				}
			}

			el.Statistics(map[string]interface{}{
				"total_found": len(records),
				"deleted":     deletedCount,
				"failed":      len(records) - deletedCount,
			})

			el.Complete(fmt.Sprintf("Deleted %d/%d records", deletedCount, len(records)))
			app.Logger().Info("Daily cleanup completed", "deleted_records", deletedCount)
		})
}

func weeklyStatsJob(app core.App) error {
	jm := jobs.GetManager()
	if jm == nil {
		return fmt.Errorf("job manager not initialized")
	}
	return jm.RegisterJob("weeklyStats", "Weekly Statistics Job",
		"Weekly analytics job that runs every Sunday at midnight to generate comprehensive todo statistics including completion rates and productivity metrics",
		"0 0 * * 0", func(el *jobs.ExecutionLogger) {
			el.Start("Weekly Statistics Job")
			el.Info("Generating weekly report for week ending: %s", time.Now().Format("2006-01-02"))

			app.Logger().Info("Generating weekly statistics", "time", time.Now())

			collection, err := app.FindCollectionByNameOrId("todos")
			if err != nil {
				el.Error("Failed to find todos collection: %v", err)
				app.Logger().Error("Failed to find todos collection", "error", err)
				el.Fail(err)
				return
			}

			el.Success("Found todos collection, analyzing data...")

			weekAgo := time.Now().AddDate(0, 0, -7)
			el.Info("Analyzing todos created since: %s", weekAgo.Format("2006-01-02"))

			filter := "created >= {:week_ago}"
			records, err := app.FindRecordsByFilter(collection, filter, "", 1000, 0, map[string]any{
				"week_ago": weekAgo.Format("2006-01-02 15:04:05.000Z"),
			})
			if err != nil {
				el.Error("Failed to fetch weekly todos: %v", err)
				app.Logger().Error("Failed to fetch weekly todos", "error", err)
				el.Fail(err)
				return
			}

			el.Progress("Processing %d todos from the past week...", len(records))

			completed, pending := 0, 0
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

			el.Info("WEEKLY STATISTICS REPORT")
			el.Statistics(map[string]interface{}{
				"Total todos created": len(records),
				"Completed todos":     completed,
				"Pending todos":       pending,
				"Completion rate":     fmt.Sprintf("%.1f%%", completionRate),
			})
			el.Complete("Weekly statistics report generated successfully")

			app.Logger().Info("Weekly statistics generated",
				"total_todos_created", len(records),
				"completed_todos", completed,
				"pending_todos", pending,
				"completion_rate", completionRate,
			)
		})
}
