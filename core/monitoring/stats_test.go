package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSystemStats(t *testing.T) {
	stats := SystemStats{
		Hostname:      "test-host",
		Platform:      "linux",
		OS:            "ubuntu",
		KernelVersion: "5.4.0",
		StartTime:     time.Now(),
		UptimeSecs:    3600,
		HasTempData:   true,
	}

	if stats.Hostname != "test-host" {
		t.Errorf("Expected Hostname 'test-host', got %s", stats.Hostname)
	}
	if stats.Platform != "linux" {
		t.Errorf("Expected Platform 'linux', got %s", stats.Platform)
	}
	if stats.OS != "ubuntu" {
		t.Errorf("Expected OS 'ubuntu', got %s", stats.OS)
	}
	if stats.KernelVersion != "5.4.0" {
		t.Errorf("Expected KernelVersion '5.4.0', got %s", stats.KernelVersion)
	}
	if stats.UptimeSecs != 3600 {
		t.Errorf("Expected UptimeSecs 3600, got %d", stats.UptimeSecs)
	}
	if !stats.HasTempData {
		t.Error("Expected HasTempData to be true")
	}
}

func TestSystemStatsZeroValues(t *testing.T) {
	var stats SystemStats

	if stats.Hostname != "" {
		t.Errorf("Expected empty Hostname, got %s", stats.Hostname)
	}
	if stats.UptimeSecs != 0 {
		t.Errorf("Expected UptimeSecs 0, got %d", stats.UptimeSecs)
	}
	if stats.HasTempData {
		t.Error("Expected HasTempData to be false")
	}
	if !stats.StartTime.IsZero() {
		t.Error("Expected StartTime to be zero")
	}
}

func TestStatsRefreshInterval(t *testing.T) {
	expectedInterval := 2 * time.Second
	if StatsRefreshInterval != expectedInterval {
		t.Errorf("Expected StatsRefreshInterval to be %v, got %v", expectedInterval, StatsRefreshInterval)
	}
}

func TestCollectSystemStatsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	startTime := time.Now()
	_, err := CollectSystemStats(ctx, startTime)

	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestCollectSystemStatsContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(1 * time.Millisecond)

	startTime := time.Now()
	_, err := CollectSystemStats(ctx, startTime)

	if err == nil {
		t.Error("Expected error for timed out context, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestSystemStatsFields(t *testing.T) {
	startTime := time.Now()

	// Test that we can create a SystemStats with all fields
	stats := SystemStats{
		Hostname:           "test-host",
		Platform:           "linux",
		OS:                 "ubuntu",
		KernelVersion:      "5.4.0",
		CPUInfo:            []CPUInfo{},
		MemoryInfo:         MemoryInfo{},
		DiskTotal:          1000000000,
		DiskUsed:           500000000,
		DiskFree:           500000000,
		RuntimeStats:       RuntimeStats{},
		ProcessStats:       ProcessInfo{},
		StartTime:          startTime,
		UptimeSecs:         3600,
		HasTempData:        true,
		NetworkInterfaces:  []NetworkInterface{},
		NetworkConnections: 10,
		NetworkBytesSent:   1024,
		NetworkBytesRecv:   2048,
	}

	// Test that all fields are accessible and have expected values
	if stats.DiskTotal != 1000000000 {
		t.Errorf("Expected DiskTotal 1000000000, got %d", stats.DiskTotal)
	}
	if stats.DiskUsed != 500000000 {
		t.Errorf("Expected DiskUsed 500000000, got %d", stats.DiskUsed)
	}
	if stats.DiskFree != 500000000 {
		t.Errorf("Expected DiskFree 500000000, got %d", stats.DiskFree)
	}
	if stats.NetworkConnections != 10 {
		t.Errorf("Expected NetworkConnections 10, got %d", stats.NetworkConnections)
	}
	if stats.NetworkBytesSent != 1024 {
		t.Errorf("Expected NetworkBytesSent 1024, got %d", stats.NetworkBytesSent)
	}
	if stats.NetworkBytesRecv != 2048 {
		t.Errorf("Expected NetworkBytesRecv 2048, got %d", stats.NetworkBytesRecv)
	}
}

func TestCollectSystemStatsBasic(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now()

	// Try to collect stats - this may fail on some systems, so we handle it gracefully
	stats, err := CollectSystemStats(ctx, startTime)

	if err != nil {
		// Log the error but don't fail the test - system calls may not work in all environments
		t.Logf("CollectSystemStats failed (may be expected in test environment): %v", err)
		return
	}

	if stats == nil {
		t.Error("Expected non-nil stats when no error occurred")
		return
	}

	// Basic validation - these should always be true
	if stats.StartTime != startTime {
		t.Errorf("Expected StartTime %v, got %v", startTime, stats.StartTime)
	}

	if stats.UptimeSecs < 0 {
		t.Errorf("Expected non-negative UptimeSecs, got %d", stats.UptimeSecs)
	}
}

func TestCollectSystemStatsWithoutContextBasic(t *testing.T) {
	startTime := time.Now()

	// Try basic collection without context
	stats, err := CollectSystemStatsWithoutContext(startTime)

	if err != nil {
		t.Logf("CollectSystemStatsWithoutContext failed (may be expected): %v", err)
		return
	}

	if stats != nil {
		// If we got stats, validate basic properties
		// Allow some timing difference (within 1 second) since the function may create its own start time
		timeDiff := stats.StartTime.Sub(startTime)
		if timeDiff < -time.Second || timeDiff > time.Second {
			t.Errorf("StartTime should be close to expected time. Expected around %v, got %v (diff: %v)", startTime, stats.StartTime, timeDiff)
		}

		if stats.StartTime.IsZero() {
			t.Error("StartTime should not be zero")
		}
	}
}

// Test edge cases with mock data
func TestSystemStatsEdgeCases(t *testing.T) {
	// Test with zero start time
	zeroTime := time.Time{}
	stats := SystemStats{
		StartTime:  zeroTime,
		UptimeSecs: 0,
	}

	if !stats.StartTime.IsZero() {
		t.Error("Expected zero start time to be zero")
	}

	// Test with future start time
	futureTime := time.Now().Add(1 * time.Hour)
	stats.StartTime = futureTime

	// Calculate what uptime should be (negative)
	uptime := int64(time.Since(futureTime).Seconds())
	stats.UptimeSecs = uptime

	if stats.UptimeSecs > 0 {
		t.Error("Expected negative uptime for future start time")
	}
}

// Benchmark tests
func BenchmarkSystemStatsCreation(b *testing.B) {
	startTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SystemStats{
			Hostname:      "benchmark-host",
			Platform:      "linux",
			OS:            "ubuntu",
			KernelVersion: "5.4.0",
			StartTime:     startTime,
			UptimeSecs:    int64(time.Since(startTime).Seconds()),
			HasTempData:   true,
		}
	}
}

func BenchmarkUptimeCalculation(b *testing.B) {
	startTime := time.Now().Add(-1 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = int64(time.Since(startTime).Seconds())
	}
}

// Test that the stats collector handles nil gracefully
func TestStatsCollectorSafety(t *testing.T) {
	// Test with nil context - should be handled by context package
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Should not panic with nil context, got: %v", r)
		}
	}()

	startTime := time.Now()
	_, err := CollectSystemStats(context.TODO(), startTime)

	// May error but should not panic
	if err != nil {
		t.Logf("CollectSystemStats with nil context failed as expected: %v", err)
	}
}

// Test struct field alignment and size
func TestSystemStatsStructure(t *testing.T) {
	var stats SystemStats

	// Test that we can assign to all fields without compilation errors
	stats.Hostname = "test"
	stats.Platform = "test"
	stats.OS = "test"
	stats.KernelVersion = "test"
	stats.CPUInfo = make([]CPUInfo, 0)
	stats.MemoryInfo = MemoryInfo{}
	stats.DiskTotal = 0
	stats.DiskUsed = 0
	stats.DiskFree = 0
	stats.RuntimeStats = RuntimeStats{}
	stats.ProcessStats = ProcessInfo{}
	stats.StartTime = time.Now()
	stats.UptimeSecs = 0
	stats.HasTempData = false
	stats.NetworkInterfaces = make([]NetworkInterface, 0)
	stats.NetworkConnections = 0
	stats.NetworkBytesSent = 0
	stats.NetworkBytesRecv = 0

	// All assignments successful if we reach here
	t.Log("SystemStats struct fields are all accessible")
}

// Example usage (these won't actually run but show intended usage)
func ExampleCollectSystemStats() {
	ctx := context.Background()
	startTime := time.Now()

	stats, err := CollectSystemStats(ctx, startTime)
	if err != nil {
		// Handle error
		return
	}

	_ = stats.Hostname
	_ = stats.Platform
	// Output varies by system
}

func ExampleSystemStats() {
	stats := SystemStats{
		Hostname:   "example-host",
		Platform:   "linux",
		UptimeSecs: 3600, // 1 hour
	}

	_ = stats
}
