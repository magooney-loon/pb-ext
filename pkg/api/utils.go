package api

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// registerUtilRoutes adds utility endpoints that don't require auth
func registerUtilRoutes(e *core.ServeEvent) {
	// Group routes under /api/utils prefix
	utilGroup := e.Router.Group("/api/utils")

	// Add routes to the group
	utilGroup.GET("/time", handleGetTime)
}

// handleGetTime returns server time in various formats
func handleGetTime(c *core.RequestEvent) error {
	now := time.Now()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"timestamp": now.Unix(),
		"iso8601":   now.Format(time.RFC3339),
		"rfc822":    now.Format(time.RFC822),
		"date":      now.Format("2006-01-02"),
		"time":      now.Format("15:04:05"),
	})
}
