package api

import (
	"crypto/rand"
	"fmt"
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
	utilGroup.GET("/uuid", handleGenerateUUID)
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

// handleGenerateUUID generates a random UUID v4
func handleGenerateUUID(c *core.RequestEvent) error {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return c.InternalServerError("Failed to generate UUID", err)
	}

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return c.JSON(http.StatusOK, map[string]string{
		"uuid": fmt.Sprintf("%x-%x-%x-%x-%x",
			uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]),
	})
}
