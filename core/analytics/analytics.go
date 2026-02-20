package analytics

import (
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Analytics tracks page views using aggregated daily counters and a session ring buffer.
// No personal data (IP, user agent, visitor ID) is persisted.
type Analytics struct {
	app core.App

	// knownVisitors is an ephemeral in-memory session map.
	// Keys are FNV-1a hashes of (ip+ua) — never written to the database.
	// Used only to determine whether a visit is new within the session window.
	knownVisitors map[string]time.Time
	visitorsMu    sync.RWMutex
	sessionWindow time.Duration
}

// New creates an Analytics instance. Use Initialize for normal startup.
func New(app core.App) *Analytics {
	return &Analytics{
		app:           app,
		knownVisitors: make(map[string]time.Time),
		sessionWindow: 30 * time.Minute,
	}
}

// Initialize creates both collections, starts the session cleanup worker, and returns an Analytics.
func Initialize(app core.App) (*Analytics, error) {
	app.Logger().Info("Initializing analytics system")

	if err := SetupCollections(app); err != nil {
		return nil, err
	}

	a := New(app)
	go a.sessionCleanupWorker()

	return a, nil
}

// sessionCleanupWorker periodically removes expired entries from the in-memory session map.
func (a *Analytics) sessionCleanupWorker() {
	ticker := time.NewTicker(a.sessionWindow)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-a.sessionWindow)
		a.visitorsMu.Lock()
		before := len(a.knownVisitors)
		for id, t := range a.knownVisitors {
			if t.Before(cutoff) {
				delete(a.knownVisitors, id)
			}
		}
		after := len(a.knownVisitors)
		a.visitorsMu.Unlock()

		if before != after {
			a.app.Logger().Debug("Cleaned up expired sessions", "removed", before-after, "remaining", after)
		}
	}
}
