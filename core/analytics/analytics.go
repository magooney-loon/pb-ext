package analytics

import (
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Analytics tracks page views with in-memory buffering and periodic PocketBase persistence.
type Analytics struct {
	app           core.App
	buffer        []PageView
	bufferMu      sync.Mutex
	flushInterval time.Duration
	batchSize     int
	lastFlush     time.Time
	flushChan     chan struct{}
	flushTicker   *time.Ticker
	flushActive   bool
	flushMu       sync.Mutex

	knownVisitors map[string]time.Time
	visitorsMu    sync.RWMutex
	sessionWindow time.Duration
}

// New creates an Analytics instance. Use Initialize for normal startup.
func New(app core.App) *Analytics {
	return &Analytics{
		app:           app,
		buffer:        make([]PageView, 0, 100),
		flushInterval: 10 * time.Minute,
		batchSize:     50,
		lastFlush:     time.Now(),
		flushChan:     make(chan struct{}, 1),
		knownVisitors: make(map[string]time.Time),
		sessionWindow: 30 * time.Minute,
	}
}

// Initialize creates the collection, starts background workers, and returns an Analytics.
func Initialize(app core.App) (*Analytics, error) {
	app.Logger().Info("Initializing analytics system")

	if err := SetupCollection(app); err != nil {
		return nil, err
	}

	a := New(app)
	go a.backgroundFlushWorker()
	go a.sessionCleanupWorker()

	return a, nil
}
