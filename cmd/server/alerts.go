package main

import (
	"fmt"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/pocketbase/pocketbase/core"
)

// Example of hooking application code into the alert system.
//
// Credentials are configured in main.go (or via the PBEXT_TELEGRAM_* variables).
// Nothing here needs to know whether they were: alerts.Get() never returns nil,
// and Send on an unconfigured notifier is a no-op that does no I/O — so alerting
// calls can sit in normal code paths without a guard or a feature flag.
func registerAlerts(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// A custom rule. The evaluator runs Check every 30 seconds and turns the
		// boolean into edges: it fires once when the condition starts holding,
		// stays quiet while it continues to hold, and sends a recovery notice
		// when it clears.
		//
		// Check runs on the evaluator goroutine, so it may query the database —
		// but it delays every other rule, so it should return promptly.
		if err := alerts.Get().AddRule(alerts.Rule{
			Key:      "todo_backlog",
			Level:    alerts.LevelWarn,
			Recovery: true,
			// Require three consecutive breaches, so a momentary spike during a
			// bulk import does not page anyone.
			Sustain: 3,
			Check: func() (bool, alerts.Message) {
				collection, err := app.FindCollectionByNameOrId("todos")
				if err != nil {
					return false, alerts.Message{}
				}

				pending, err := app.CountRecords(collection, nil)
				if err != nil || pending < 500 {
					return false, alerts.Message{}
				}

				return true, alerts.Message{
					Title: fmt.Sprintf("Todo backlog at %d items", pending),
					Fields: map[string]string{
						"threshold": "500",
					},
				}
			},
		}); err != nil {
			app.Logger().Error("Failed to register the backlog alert rule", "error", err)
		}

		// A one-off alert from application code. An empty Key is never
		// suppressed by the cooldown, which is what makes an explicit Send like
		// this one reliable.
		alerts.Get().Send(alerts.Message{
			Level: alerts.LevelInfo,
			Title: "Example app ready",
			Fields: map[string]string{
				"started": time.Now().Format(time.RFC3339),
			},
		})

		return e.Next()
	})
}
