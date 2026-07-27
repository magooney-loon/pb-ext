package audit

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// maxQueryLimit caps how many rows one request can pull back. The table holds
// client addresses and account names; a generous limit would turn a single
// compromised superuser token into a bulk export.
const maxQueryLimit = 500

// Handlers exposes the access log over HTTP.
type Handlers struct {
	auditor *Auditor
}

// NewHandlers creates the HTTP handlers for an auditor.
func NewHandlers(a *Auditor) *Handlers {
	return &Handlers{auditor: a}
}

// RegisterRoutes registers the audit endpoints, all superuser-only.
//
// These return personal data — who connected from where — so they carry the
// same guard as the job routes, and are the reason maxQueryLimit exists.
func (h *Handlers) RegisterRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/audit/status", h.handleStatus).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/audit/recent", h.handleRecent).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/audit/sources", h.handleSources).Bind(apis.RequireSuperuserAuth())
}

// handleStatus reports the capture state and the trailing-window summary.
func (h *Handlers) handleStatus(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, h.auditor.Stats())
}

// handleRecent returns the tail of the access log.
func (h *Handlers) handleRecent(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"records": h.auditor.Recent(queryLimit(c, RecentLimit)),
	})
}

// handleSources returns the per-address rollup.
func (h *Handlers) handleSources(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"sources": h.auditor.TopIPs(queryLimit(c, TopIPsLimit)),
	})
}

func queryLimit(c *core.RequestEvent, fallback int) int {
	raw := c.Request.URL.Query().Get("limit")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return min(parsed, maxQueryLimit)
}
