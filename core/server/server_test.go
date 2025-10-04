package server

import (
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
)

func TestServerStatsStruct(t *testing.T) {
	startTime := time.Now()
	stats := &ServerStats{
		StartTime: startTime,
	}

	// Test initial values
	if stats.StartTime != startTime {
		t.Errorf("Expected StartTime %v, got %v", startTime, stats.StartTime)
	}
	if stats.TotalRequests.Load() != 0 {
		t.Errorf("Expected TotalRequests 0, got %d", stats.TotalRequests.Load())
	}
	if stats.ActiveConnections.Load() != 0 {
		t.Errorf("Expected ActiveConnections 0, got %d", stats.ActiveConnections.Load())
	}
	if stats.LastRequestTime.Load() != 0 {
		t.Errorf("Expected LastRequestTime 0, got %d", stats.LastRequestTime.Load())
	}
	if stats.TotalErrors.Load() != 0 {
		t.Errorf("Expected TotalErrors 0, got %d", stats.TotalErrors.Load())
	}
	if stats.AverageRequestTime.Load() != 0 {
		t.Errorf("Expected AverageRequestTime 0, got %d", stats.AverageRequestTime.Load())
	}
}

func TestServerStatsAtomicOperations(t *testing.T) {
	stats := &ServerStats{
		StartTime: time.Now(),
	}

	// Test atomic operations
	stats.TotalRequests.Add(5)
	if stats.TotalRequests.Load() != 5 {
		t.Errorf("Expected TotalRequests 5, got %d", stats.TotalRequests.Load())
	}

	stats.ActiveConnections.Add(3)
	if stats.ActiveConnections.Load() != 3 {
		t.Errorf("Expected ActiveConnections 3, got %d", stats.ActiveConnections.Load())
	}

	stats.ActiveConnections.Add(-1)
	if stats.ActiveConnections.Load() != 2 {
		t.Errorf("Expected ActiveConnections 2, got %d", stats.ActiveConnections.Load())
	}

	stats.TotalErrors.Add(1)
	if stats.TotalErrors.Load() != 1 {
		t.Errorf("Expected TotalErrors 1, got %d", stats.TotalErrors.Load())
	}

	now := time.Now().Unix()
	stats.LastRequestTime.Store(now)
	if stats.LastRequestTime.Load() != now {
		t.Errorf("Expected LastRequestTime %d, got %d", now, stats.LastRequestTime.Load())
	}

	stats.AverageRequestTime.Store(1000000) // 1ms in nanoseconds
	if stats.AverageRequestTime.Load() != 1000000 {
		t.Errorf("Expected AverageRequestTime 1000000, got %d", stats.AverageRequestTime.Load())
	}
}

func TestNewServerDefault(t *testing.T) {
	server := New()

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.app == nil {
		t.Error("Expected non-nil app")
	}
	if server.stats == nil {
		t.Error("Expected non-nil stats")
	}
	if server.options == nil {
		t.Error("Expected non-nil options")
	}

	// Check that stats are initialized
	if server.stats.StartTime.IsZero() {
		t.Error("Expected non-zero start time")
	}
}

func TestNewServerWithDeveloperMode(t *testing.T) {
	server := New(InDeveloperMode())

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.options == nil {
		t.Fatal("Expected non-nil options")
	}
	if !server.options.developer_mode {
		t.Error("Expected developer mode to be enabled")
	}
}

func TestNewServerWithNormalMode(t *testing.T) {
	server := New(InNormalMode())

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.options == nil {
		t.Fatal("Expected non-nil options")
	}
	if server.options.developer_mode {
		t.Error("Expected developer mode to be disabled")
	}
}

func TestNewServerWithMode(t *testing.T) {
	// Test WithMode(true)
	server1 := New(WithMode(true))
	if server1 == nil {
		t.Fatal("Expected non-nil server")
	}
	if !server1.options.developer_mode {
		t.Error("Expected developer mode to be enabled")
	}

	// Test WithMode(false)
	server2 := New(WithMode(false))
	if server2 == nil {
		t.Fatal("Expected non-nil server")
	}
	if server2.options.developer_mode {
		t.Error("Expected developer mode to be disabled")
	}
}

func TestNewServerWithConfig(t *testing.T) {
	config := &pocketbase.Config{
		DefaultDev: true,
	}
	server := New(WithConfig(config))

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.options == nil {
		t.Fatal("Expected non-nil options")
	}
	if server.options.config != config {
		t.Error("Expected config to be set")
	}
}

func TestNewServerWithMultipleOptions(t *testing.T) {
	config := &pocketbase.Config{DefaultDev: false}
	server := New(
		InDeveloperMode(),
		WithConfig(config),
	)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if !server.options.developer_mode {
		t.Error("Expected developer mode to be enabled")
	}
	if server.options.config != config {
		t.Error("Expected config to be set")
	}
}

func TestServerStruct(t *testing.T) {
	server := &Server{
		app:     nil, // Will be set by New()
		stats:   &ServerStats{StartTime: time.Now()},
		options: &options{developer_mode: true},
	}

	if server.stats == nil {
		t.Error("Expected non-nil stats")
	}
	if server.options == nil {
		t.Error("Expected non-nil options")
	}
	if !server.options.developer_mode {
		t.Error("Expected developer mode to be true")
	}
}

func TestServerConcurrentStatsUpdates(t *testing.T) {
	server := New()
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	// Test concurrent access to stats
	var wg sync.WaitGroup
	numGoroutines := 10
	numIncrementsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIncrementsPerGoroutine; j++ {
				server.stats.TotalRequests.Add(1)
				server.stats.ActiveConnections.Add(1)
				server.stats.TotalErrors.Add(1)
				server.stats.ActiveConnections.Add(-1)
			}
		}()
	}

	wg.Wait()

	expectedRequests := uint64(numGoroutines * numIncrementsPerGoroutine)
	expectedErrors := uint64(numGoroutines * numIncrementsPerGoroutine)

	if server.stats.TotalRequests.Load() != expectedRequests {
		t.Errorf("Expected TotalRequests %d, got %d", expectedRequests, server.stats.TotalRequests.Load())
	}
	if server.stats.TotalErrors.Load() != expectedErrors {
		t.Errorf("Expected TotalErrors %d, got %d", expectedErrors, server.stats.TotalErrors.Load())
	}
	if server.stats.ActiveConnections.Load() != 0 {
		t.Errorf("Expected ActiveConnections 0, got %d", server.stats.ActiveConnections.Load())
	}
}

func TestServerRequestMiddlewareSimulation(t *testing.T) {
	server := New()

	// Simulate request processing
	initialRequests := server.stats.TotalRequests.Load()
	initialConnections := server.stats.ActiveConnections.Load()

	// Simulate the middleware logic
	server.stats.ActiveConnections.Add(1)
	server.stats.TotalRequests.Add(1)

	// Simulate request completion
	server.stats.ActiveConnections.Add(-1)
	server.stats.LastRequestTime.Store(time.Now().Unix())

	// Check stats were updated
	if server.stats.TotalRequests.Load() != initialRequests+1 {
		t.Errorf("Expected TotalRequests to increase by 1")
	}
	if server.stats.ActiveConnections.Load() != initialConnections {
		t.Errorf("Expected ActiveConnections to return to initial value")
	}
	if server.stats.LastRequestTime.Load() == 0 {
		t.Error("Expected LastRequestTime to be set")
	}
}

func TestServerAverageRequestTimeCalculation(t *testing.T) {
	server := New()

	// Test first request
	duration1 := int64(1000000) // 1ms
	server.stats.TotalRequests.Add(1)
	server.stats.AverageRequestTime.Store(duration1)

	if server.stats.AverageRequestTime.Load() != duration1 {
		t.Errorf("Expected AverageRequestTime %d, got %d", duration1, server.stats.AverageRequestTime.Load())
	}

	// Test second request (calculate average)
	duration2 := int64(2000000) // 2ms
	server.stats.TotalRequests.Add(1)
	totalReqs := server.stats.TotalRequests.Load()
	oldAvg := server.stats.AverageRequestTime.Load()

	newAvg := (oldAvg*(int64(totalReqs)-1) + duration2) / int64(totalReqs)
	server.stats.AverageRequestTime.Store(newAvg)

	expectedAvg := (duration1 + duration2) / 2
	if server.stats.AverageRequestTime.Load() != expectedAvg {
		t.Errorf("Expected AverageRequestTime %d, got %d", expectedAvg, server.stats.AverageRequestTime.Load())
	}
}

func TestServerErrorTracking(t *testing.T) {
	server := New()

	initialErrors := server.stats.TotalErrors.Load()

	// Simulate error occurrence
	server.stats.TotalErrors.Add(1)

	if server.stats.TotalErrors.Load() != initialErrors+1 {
		t.Errorf("Expected TotalErrors to increase by 1")
	}

	// Simulate multiple errors
	server.stats.TotalErrors.Add(5)

	if server.stats.TotalErrors.Load() != initialErrors+6 {
		t.Errorf("Expected TotalErrors to be %d, got %d", initialErrors+6, server.stats.TotalErrors.Load())
	}
}

func TestServerZeroValues(t *testing.T) {
	var server Server

	if server.app != nil {
		t.Error("Expected nil app in zero Server")
	}
	if server.stats != nil {
		t.Error("Expected nil stats in zero Server")
	}
	if server.options != nil {
		t.Error("Expected nil options in zero Server")
	}
}

func TestServerStatsFields(t *testing.T) {
	stats := &ServerStats{
		StartTime: time.Now(),
	}

	// Test that we can access all atomic fields without panicking
	_ = stats.TotalRequests.Load()
	_ = stats.ActiveConnections.Load()
	_ = stats.LastRequestTime.Load()
	_ = stats.TotalErrors.Load()
	_ = stats.AverageRequestTime.Load()

	// Test that we can modify all atomic fields
	stats.TotalRequests.Store(10)
	stats.ActiveConnections.Store(5)
	stats.LastRequestTime.Store(time.Now().Unix())
	stats.TotalErrors.Store(2)
	stats.AverageRequestTime.Store(1500000)

	// Verify values
	if stats.TotalRequests.Load() != 10 {
		t.Error("TotalRequests not set correctly")
	}
	if stats.ActiveConnections.Load() != 5 {
		t.Error("ActiveConnections not set correctly")
	}
	if stats.LastRequestTime.Load() == 0 {
		t.Error("LastRequestTime not set correctly")
	}
	if stats.TotalErrors.Load() != 2 {
		t.Error("TotalErrors not set correctly")
	}
	if stats.AverageRequestTime.Load() != 1500000 {
		t.Error("AverageRequestTime not set correctly")
	}
}

func TestServerOptionsConfiguration(t *testing.T) {
	// Test that options are properly configured
	opts := &options{
		config:         nil,
		pocketbase:     nil,
		developer_mode: true,
	}

	if !opts.developer_mode {
		t.Error("Expected developer_mode to be true")
	}

	// Test options with config
	config := &pocketbase.Config{DefaultDev: true}
	opts.config = config

	if opts.config != config {
		t.Error("Expected config to be set")
	}
}

func TestServerMemoryUsage(t *testing.T) {
	// Test that creating multiple servers doesn't cause obvious memory issues
	servers := make([]*Server, 10)
	for i := 0; i < 10; i++ {
		servers[i] = New()
		if servers[i] == nil {
			t.Fatalf("Server %d is nil", i)
		}
	}

	// All servers should be independent
	servers[0].stats.TotalRequests.Add(5)
	if servers[1].stats.TotalRequests.Load() != 0 {
		t.Error("Servers are not independent")
	}
}

// Benchmark tests
func BenchmarkServerStatsIncrement(b *testing.B) {
	server := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.stats.TotalRequests.Add(1)
	}
}

func BenchmarkServerStatsMultipleFields(b *testing.B) {
	server := New()
	now := time.Now().Unix()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.stats.ActiveConnections.Add(1)
		server.stats.TotalRequests.Add(1)
		server.stats.LastRequestTime.Store(now)
		server.stats.ActiveConnections.Add(-1)
	}
}

func BenchmarkNewServer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		server := New()
		_ = server
	}
}

func BenchmarkAverageRequestTimeCalculation(b *testing.B) {
	server := New()
	server.stats.TotalRequests.Store(100)
	server.stats.AverageRequestTime.Store(1500000) // 1.5ms

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		oldAvg := server.stats.AverageRequestTime.Load()
		totalReqs := server.stats.TotalRequests.Load()
		newDuration := int64(2000000) // 2ms
		newAvg := (oldAvg*(int64(totalReqs)-1) + newDuration) / int64(totalReqs)
		server.stats.AverageRequestTime.Store(newAvg)
	}
}

// Example tests
func ExampleNew() {
	server := New()
	_ = server.stats.TotalRequests.Load()
	// Server is ready to use
}

func ExampleServerStats() {
	server := New()

	// Simulate request
	server.stats.ActiveConnections.Add(1)
	server.stats.TotalRequests.Add(1)

	// Process request...

	// Complete request
	server.stats.ActiveConnections.Add(-1)
	server.stats.LastRequestTime.Store(time.Now().Unix())
}

func TestServerOptionsValidation(t *testing.T) {
	// Test that options function correctly
	server1 := New(InDeveloperMode(), InNormalMode())

	// Last option should win
	if server1.options.developer_mode {
		t.Error("Expected InNormalMode to override InDeveloperMode")
	}

	server2 := New(InNormalMode(), InDeveloperMode())

	// Last option should win
	if !server2.options.developer_mode {
		t.Error("Expected InDeveloperMode to override InNormalMode")
	}
}

func TestServerStatsStartTime(t *testing.T) {
	before := time.Now()
	server := New()
	after := time.Now()

	// Start time should be between before and after
	if server.stats.StartTime.Before(before) || server.stats.StartTime.After(after) {
		t.Errorf("Start time %v should be between %v and %v",
			server.stats.StartTime, before, after)
	}
}

func TestServerRequestTimeMetrics(t *testing.T) {
	server := New()

	// Simulate multiple requests with different durations
	durations := []int64{1000000, 2000000, 3000000, 4000000, 5000000} // 1ms to 5ms

	for i, duration := range durations {
		server.stats.TotalRequests.Add(1)

		if i == 0 {
			// First request
			server.stats.AverageRequestTime.Store(duration)
		} else {
			// Subsequent requests - calculate running average
			oldAvg := server.stats.AverageRequestTime.Load()
			totalReqs := server.stats.TotalRequests.Load()
			newAvg := (oldAvg*(int64(totalReqs)-1) + duration) / int64(totalReqs)
			server.stats.AverageRequestTime.Store(newAvg)
		}
	}

	// Average should be 3ms (3000000ns)
	expectedAvg := int64(3000000)
	actualAvg := server.stats.AverageRequestTime.Load()

	if actualAvg != expectedAvg {
		t.Errorf("Expected average request time %d, got %d", expectedAvg, actualAvg)
	}
}
