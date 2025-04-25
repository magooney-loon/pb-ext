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

// NewCircularBuffer creates a new circular buffer with given size
func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		buffer: make([]RequestMetrics, size),
		size:   size,
	}
}

// Add adds an item to the circular buffer
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

// RequestMetrics holds detailed request tracking information
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

// RequestStats holds aggregated request statistics
type RequestStats struct {
	mu             sync.RWMutex
	pathStats      map[string]*PathStats
	recentRequests *CircularBuffer
	requestRate    float64
	lastRateCalc   time.Time
	requestCount   int64
}

// PathStats holds statistics for a specific path
type PathStats struct {
	TotalRequests   int64
	TotalErrors     int64
	AverageTime     time.Duration
	LastAccessTime  time.Time
	StatusCodeCount map[int]int64
}

// NewRequestStats creates a new RequestStats instance
func NewRequestStats() *RequestStats {
	return &RequestStats{
		pathStats:      make(map[string]*PathStats),
		recentRequests: NewCircularBuffer(100), // Keep last 100 requests
		lastRateCalc:   time.Now(),
	}
}

// TrackRequest records a new request
func (rs *RequestStats) TrackRequest(metrics RequestMetrics) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Update path stats
	pathStat, exists := rs.pathStats[metrics.Path]
	if !exists {
		pathStat = &PathStats{
			StatusCodeCount: make(map[int]int64),
		}
		rs.pathStats[metrics.Path] = pathStat
	}

	pathStat.TotalRequests++
	if metrics.StatusCode >= 400 {
		pathStat.TotalErrors++
	}
	pathStat.StatusCodeCount[metrics.StatusCode]++
	pathStat.LastAccessTime = metrics.Timestamp

	// Update average time using exponential moving average
	if pathStat.TotalRequests == 1 {
		pathStat.AverageTime = metrics.Duration
	} else {
		alpha := 0.1 // Smoothing factor
		pathStat.AverageTime = time.Duration(float64(pathStat.AverageTime)*(1-alpha) + float64(metrics.Duration)*alpha)
	}

	// Add to circular buffer
	rs.recentRequests.Add(metrics)

	// Update request rate
	rs.requestCount++
	elapsed := time.Since(rs.lastRateCalc).Seconds()
	if elapsed >= 5 { // Update rate every 5 seconds
		rs.requestRate = float64(rs.requestCount) / elapsed
		rs.requestCount = 0
		rs.lastRateCalc = time.Now()
	}
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
	if d > 1*time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
}
