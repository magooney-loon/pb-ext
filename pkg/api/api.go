package api

import (
	"github.com/pocketbase/pocketbase/core"
)

// RegisterRoutes registers all custom API routes
func RegisterRoutes(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Register custom API routes
		registerUtilRoutes(e)

		// Add more route registrations here as needed

		return e.Next()
	})
}
