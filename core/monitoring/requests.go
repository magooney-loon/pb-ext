package monitoring

import (
	"fmt"
	"sync"
	"time"
)

// CircularBuffer implements a thread-safe circular buffer
type CircularBuffer struct {
	buffer []RequestMetrics
	size   int
	mu     sync.RWMutex
	head   int
	count  int
}

// NewCircularBuffer creates a buffer with given size
func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		buffer: make([]RequestMetrics, size),
		size:   size,
	}
}

// Add adds an item to the buffer
func (c *CircularBuffer) Add(item RequestMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffer[c.head] = item
	c.head = (c.head + 1) % c.size
	if c.count < c.size {
		c.count++
	}
}

// GetAll returns all items in the buffer
func (c *CircularBuffer) GetAll() []RequestMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]RequestMetrics, c.count)
	for i := 0; i < c.count; i++ {
		pos := (c.size + c.head - c.count + i) % c.size
		result[i] = c.buffer[pos]
	}
	return result
}

// RequestMetrics holds request tracking information
type RequestMetrics struct {
	Path          string
	Method        string
	StatusCode    int
	Duration      time.Duration
	Timestamp     time.Time
	UserAgent     string
	ContentLength int64
	RemoteAddr    string
}

// RequestStats holds aggregated request statistics.
//
// Everything here is bounded: monotonic counters, a scalar rate, and a fixed
// ring of recent requests. There is deliberately no per-path breakdown — see
// the note on TrackRequest.
type RequestStats struct {
	mu             sync.RWMutex
	recentRequests *CircularBuffer
	requestRate    float64
	lastRateCalc   time.Time
	requestCount   int64
	totals         Totals
}

// Totals are monotonic counters since process start.
//
// They exist for consumers that need to derive their own rate over their own
// window — alerting, in particular. requestRate cannot serve that purpose: it
// is only recalculated when a request arrives, so after traffic stops it keeps
// reporting the last busy figure indefinitely, which is exactly the moment a
// rate is worth reading.
type Totals struct {
	Requests     uint64 `json:"requests"`
	ClientErrors uint64 `json:"client_errors"`
	ServerErrors uint64 `json:"server_errors"`
}

// NewRequestStats creates a new RequestStats instance
func NewRequestStats() *RequestStats {
	return &RequestStats{
		recentRequests: NewCircularBuffer(100), // Keep last 100 requests
		lastRateCalc:   time.Now(),
	}
}

// TrackRequest records a new request.
//
// It keeps no per-path breakdown, and must not grow one. The obvious version —
// a map[path]*stats — is keyed by attacker-chosen input: every 404 from the
// static handler is a distinct key, paths are only bounded by the ~8KB header
// limit, and nothing ever evicts. A scanner walking long junk URLs would spend
// server memory that is never returned. This is the same high-cardinality
// hazard core/analytics defends against with MaxDistinctPaths and its "/*"
// overflow bucket, and a per-path breakdown here would need the same treatment
// plus an accessor to justify existing at all.
func (rs *RequestStats) TrackRequest(metrics RequestMetrics) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Client and server errors are counted apart because they mean different
	// things: 4xx is mostly other people's bugs and scanner traffic, 5xx is
	// yours. Alerting on the sum would fire on a bot sweeping for /wp-admin.
	rs.totals.Requests++
	switch {
	case metrics.StatusCode >= 500:
		rs.totals.ServerErrors++
	case metrics.StatusCode >= 400:
		rs.totals.ClientErrors++
	}

	rs.recentRequests.Add(metrics)

	rs.requestCount++
	elapsed := time.Since(rs.lastRateCalc).Seconds()
	if elapsed >= 5 { // Update rate every 5 seconds
		rs.requestRate = float64(rs.requestCount) / elapsed
		rs.requestCount = 0
		rs.lastRateCalc = time.Now()
	}
}

// Totals returns the monotonic request counters since process start.
func (rs *RequestStats) Totals() Totals {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.totals
}

// GetRequestRate returns the current request rate per second
func (rs *RequestStats) GetRequestRate() float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.requestRate
}

// GetRecentRequests returns the most recent requests
func (rs *RequestStats) GetRecentRequests() []RequestMetrics {
	return rs.recentRequests.GetAll()
}

// GetStatusString returns a string representation of the status code
func GetStatusString(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "ERROR"
	case statusCode >= 400:
		return "WARN"
	case statusCode >= 300:
		return "REDIRECT"
	case statusCode >= 200:
		return "SUCCESS"
	default:
		return "UNKNOWN"
	}
}

// FormatDuration returns a formatted duration string
func FormatDuration(d time.Duration) string {
	if d >= 1*time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
}
