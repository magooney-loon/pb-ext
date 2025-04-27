package monitoring

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ServerStats represents server usage statistics
type ServerStats struct {
	StartTime         time.Time
	TotalRequests     atomic.Int64
	ActiveConnections atomic.Int64
	LastRequestTime   atomic.Int64
}

// NewServerStats creates a new stats tracker
func NewServerStats() *ServerStats {
	stats := &ServerStats{
		StartTime: time.Now(),
	}
	return stats
}

// IncrementRequests increases the request counter
func (s *ServerStats) IncrementRequests() {
	s.TotalRequests.Add(1)
}

// TrackConnection updates active connection count
func (s *ServerStats) TrackConnection(isOpen bool) {
	if isOpen {
		s.ActiveConnections.Add(1)
	} else {
		s.ActiveConnections.Add(-1)
	}
}

// UpdateLastRequestTime sets the last request timestamp
func (s *ServerStats) UpdateLastRequestTime() {
	s.LastRequestTime.Store(time.Now().UnixNano())
}

// TestNewServerStats tests the ServerStats creation and initial state
func TestNewServerStats(t *testing.T) {
	// Create a new stats collector
	stats := NewServerStats()

	// Verify initial state
	assert.NotNil(t, stats, "Stats collector should not be nil")
	assert.NotZero(t, stats.StartTime, "Start time should be initialized")
	assert.Equal(t, int64(0), stats.TotalRequests.Load(), "Initial request count should be zero")
	assert.Equal(t, int64(0), stats.ActiveConnections.Load(), "Initial connections should be zero")
}

func TestIncrementRequests(t *testing.T) {
	// Create a new stats collector
	stats := NewServerStats()

	// Initial state
	initialRequests := stats.TotalRequests.Load()
	assert.Equal(t, int64(0), initialRequests, "Initial request count should be zero")

	// Increment requests
	stats.IncrementRequests()

	// Verify count increased
	assert.Equal(t, initialRequests+1, stats.TotalRequests.Load(), "Request count should increment by 1")
}

func TestConnectionTracking(t *testing.T) {
	// Create a new stats collector
	stats := NewServerStats()

	// Initial state
	assert.Equal(t, int64(0), stats.ActiveConnections.Load(), "Initial connections should be zero")

	// Track connection open
	stats.TrackConnection(true)
	assert.Equal(t, int64(1), stats.ActiveConnections.Load(), "Active connections should increment")

	// Track connection close
	stats.TrackConnection(false)
	assert.Equal(t, int64(0), stats.ActiveConnections.Load(), "Active connections should decrement")
}

func TestUpdateLastRequestTime(t *testing.T) {
	// Create a new stats collector
	stats := NewServerStats()

	// Initial state
	initialTime := stats.LastRequestTime.Load()

	// Wait briefly to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Update last request time
	stats.UpdateLastRequestTime()

	// Verify time was updated
	assert.Greater(t, stats.LastRequestTime.Load(), initialTime, "Last request time should be updated")
}

func TestSystemStats(t *testing.T) {
	// Simple test to verify system stats collection works
	stats, err := CollectSystemStatsWithoutContext(time.Now())

	// Assert collection works
	assert.NoError(t, err, "Should collect system stats without error")
	assert.NotNil(t, stats, "Stats should not be nil")

	// Verify basic stats structure
	assert.NotEmpty(t, stats.OS, "OS info should be populated")

	// Test with context
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	statsWithCtx, err := CollectSystemStats(ctx, time.Now())
	assert.NoError(t, err, "Should collect system stats with context without error")
	assert.NotNil(t, statsWithCtx, "Stats with context should not be nil")
}

// TestSystemStatsIntegration tests that system stats integration works properly
func TestSystemStatsIntegration(t *testing.T) {
	// Create a stats object
	startTime := time.Now()
	stats, err := CollectSystemStatsWithoutContext(startTime)
	require.NoError(t, err, "Should collect stats without error")

	// Verify uptime calculation is working properly
	assert.WithinDuration(t, startTime, stats.StartTime, 500*time.Millisecond,
		"Start time should be set correctly within a reasonable margin")

	// Check that CPU info is populated
	assert.NotEmpty(t, stats.CPUInfo, "CPU info should be populated")

	// Verify memory info is collected
	assert.NotZero(t, stats.MemoryInfo.Total, "Memory total should be non-zero")

	// Verify disk info
	assert.True(t, stats.DiskTotal > 0, "Disk total should be positive")
	assert.True(t, stats.DiskUsed <= stats.DiskTotal, "Disk used should be less than or equal to total")
}
