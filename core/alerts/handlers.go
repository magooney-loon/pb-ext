package alerts

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// testCooldown throttles the manual test endpoint. Without it the endpoint is a
// pump: an authenticated caller could drive the bot into Telegram's flood limit
// and get the chat throttled for everyone.
const testCooldown = time.Minute

// Handlers exposes the notifier over HTTP for the dashboard.
type Handlers struct {
	notifier *Notifier

	mu       sync.Mutex
	lastTest time.Time
}

// NewHandlers creates the HTTP handlers for a notifier.
func NewHandlers(n *Notifier) *Handlers {
	return &Handlers{notifier: n}
}

// RegisterRoutes registers the alert endpoints.
//
// Every route requires superuser auth, matching the job routes: the status
// payload describes the alerting configuration, and the test endpoint sends
// real messages.
func (h *Handlers) RegisterRoutes(e *core.ServeEvent) {
	e.Router.GET("/api/alerts/status", h.handleStatus).Bind(apis.RequireSuperuserAuth())
	e.Router.GET("/api/alerts/recent", h.handleRecent).Bind(apis.RequireSuperuserAuth())
	e.Router.POST("/api/alerts/test", h.handleTest).Bind(apis.RequireSuperuserAuth())
}

// handleStatus reports the current configuration and counters. The bot token is
// never part of the payload — Stats carries the target, not the credential.
func (h *Handlers) handleStatus(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, h.notifier.Stats())
}

// handleRecent returns the tail of the delivery log.
func (h *Handlers) handleRecent(c *core.RequestEvent) error {
	limit := RecentLimit
	if raw := c.Request.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = min(parsed, 200)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"records": h.notifier.Recent(limit)})
}

// handleTest sends one message immediately and reports what happened.
//
// It deliberately bypasses the queue: the point of a test button is to learn
// whether delivery works right now and, if not, why. A queued message would
// return "accepted" whether or not the token was valid.
func (h *Handlers) handleTest(c *core.RequestEvent) error {
	h.mu.Lock()
	if since := time.Since(h.lastTest); since < testCooldown {
		h.mu.Unlock()
		return c.JSON(http.StatusTooManyRequests, map[string]any{
			"success":          false,
			"error":            "test alerts are limited to one per minute",
			"retry_in_seconds": int((testCooldown - since).Seconds()) + 1,
		})
	}
	h.lastTest = time.Now()
	h.mu.Unlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	if err := h.notifier.SendTest(ctx); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"target":  h.notifier.Stats().Target,
	})
}

// SendTest delivers a test message synchronously, returning the transport's
// error verbatim (already scrubbed of credentials).
//
// It works even when alerts are disabled, provided a transport is configured —
// checking the setup in developer mode is exactly when a test button earns its
// keep.
func (n *Notifier) SendTest(ctx context.Context) error {
	if n == nil || n.transport == nil {
		return errors.New("alerts are not configured: set a bot token and chat id")
	}

	m := Message{
		Level: LevelInfo,
		Title: "Test alert",
		Text:  "If you can read this, pb-ext can reach this chat.",
	}

	err := n.transport.Send(ctx, m, n.cfg.Instance)
	if err != nil {
		n.record(m, StatusFailed, 1, err)
		return err
	}

	n.record(m, StatusSent, 1, nil)
	return nil
}
