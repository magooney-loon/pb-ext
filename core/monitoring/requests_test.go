package monitoring

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
)

func TestNewCircularBuffer(t *testing.T) {
	size := 10
	buffer := NewCircularBuffer(size)

	if buffer.size != size {
		t.Errorf("Expected buffer size %d, got %d", size, buffer.size)
	}
	if len(buffer.buffer) != size {
		t.Errorf("Expected buffer length %d, got %d", size, len(buffer.buffer))
	}
	if buffer.head != 0 {
		t.Errorf("Expected head to be 0, got %d", buffer.head)
	}
	if buffer.count != 0 {
		t.Errorf("Expected count to be 0, got %d", buffer.count)
	}
}

func TestCircularBuffer_Add(t *testing.T) {
	buffer := NewCircularBuffer(3)

	metrics1 := RequestMetrics{Path: "/test1", Method: "GET", StatusCode: 200}
	metrics2 := RequestMetrics{Path: "/test2", Method: "POST", StatusCode: 201}
	metrics3 := RequestMetrics{Path: "/test3", Method: "PUT", StatusCode: 204}
	metrics4 := RequestMetrics{Path: "/test4", Method: "DELETE", StatusCode: 404}

	// Add first item
	buffer.Add(metrics1)
	if buffer.count != 1 {
		t.Errorf("Expected count 1, got %d", buffer.count)
	}
	if buffer.head != 1 {
		t.Errorf("Expected head 1, got %d", buffer.head)
	}

	// Add second item
	buffer.Add(metrics2)
	if buffer.count != 2 {
		t.Errorf("Expected count 2, got %d", buffer.count)
	}
	if buffer.head != 2 {
		t.Errorf("Expected head 2, got %d", buffer.head)
	}

	// Add third item (buffer full)
	buffer.Add(metrics3)
	if buffer.count != 3 {
		t.Errorf("Expected count 3, got %d", buffer.count)
	}
	if buffer.head != 0 {
		t.Errorf("Expected head to wrap to 0, got %d", buffer.head)
	}

	// Add fourth item (should overwrite first)
	buffer.Add(metrics4)
	if buffer.count != 3 {
		t.Errorf("Expected count to stay at 3, got %d", buffer.count)
	}
	if buffer.head != 1 {
		t.Errorf("Expected head 1, got %d", buffer.head)
	}
}

func TestCircularBuffer_GetAll(t *testing.T) {
	buffer := NewCircularBuffer(3)

	// Test empty buffer
	items := buffer.GetAll()
	if len(items) != 0 {
		t.Errorf("Expected empty slice, got %d items", len(items))
	}

	// Add items
	metrics1 := RequestMetrics{Path: "/test1", Method: "GET"}
	metrics2 := RequestMetrics{Path: "/test2", Method: "POST"}
	metrics3 := RequestMetrics{Path: "/test3", Method: "PUT"}

	buffer.Add(metrics1)
	buffer.Add(metrics2)

	items = buffer.GetAll()
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
	if items[0].Path != "/test1" || items[1].Path != "/test2" {
		t.Error("Items not returned in correct order")
	}

	// Fill buffer
	buffer.Add(metrics3)
	items = buffer.GetAll()
	if len(items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(items))
	}

	// Overflow buffer
	metrics4 := RequestMetrics{Path: "/test4", Method: "DELETE"}
	buffer.Add(metrics4)
	items = buffer.GetAll()
	if len(items) != 3 {
		t.Errorf("Expected 3 items after overflow, got %d", len(items))
	}
	// Should have test2, test3, test4 (test1 was overwritten)
	if items[0].Path != "/test2" || items[1].Path != "/test3" || items[2].Path != "/test4" {
		t.Error("Items not in correct order after overflow")
	}
}

func TestCircularBuffer_Concurrent(t *testing.T) {
	buffer := NewCircularBuffer(100)
	const numGoroutines = 10
	const itemsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrently add items
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				metrics := RequestMetrics{
					Path:   fmt.Sprintf("/test%d-%d", routineID, j),
					Method: "GET",
				}
				buffer.Add(metrics)
			}
		}(i)
	}

	wg.Wait()

	// Should have exactly 100 items (buffer size)
	items := buffer.GetAll()
	if len(items) != 100 {
		t.Errorf("Expected 100 items, got %d", len(items))
	}
}

func TestNewRequestStats(t *testing.T) {
	stats := NewRequestStats()

	if stats.pathStats == nil {
		t.Error("Expected pathStats to be initialized")
	}
	if stats.recentRequests == nil {
		t.Error("Expected recentRequests to be initialized")
	}
	if stats.requestRate != 0 {
		t.Error("Expected initial request rate to be 0")
	}
	if stats.requestCount != 0 {
		t.Error("Expected initial request count to be 0")
	}
}

func TestRequestStats_TrackRequest(t *testing.T) {
	stats := NewRequestStats()

	metrics := RequestMetrics{
		Path:       "/api/users",
		Method:     "GET",
		StatusCode: 200,
		Duration:   100 * time.Millisecond,
		Timestamp:  time.Now(),
	}

	stats.TrackRequest(metrics)

	// Check path stats
	pathStat, exists := stats.pathStats["/api/users"]
	if !exists {
		t.Error("Expected path stats to be created")
	}
	if pathStat.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", pathStat.TotalRequests)
	}
	if pathStat.TotalErrors != 0 {
		t.Errorf("Expected 0 errors, got %d", pathStat.TotalErrors)
	}
	if pathStat.StatusCodeCount[200] != 1 {
		t.Errorf("Expected 1 count for status 200, got %d", pathStat.StatusCodeCount[200])
	}
	if pathStat.AverageTime != 100*time.Millisecond {
		t.Errorf("Expected average time 100ms, got %v", pathStat.AverageTime)
	}
}

func TestRequestStats_TrackRequestWithError(t *testing.T) {
	stats := NewRequestStats()

	metrics := RequestMetrics{
		Path:       "/api/users",
		Method:     "GET",
		StatusCode: 500,
		Duration:   200 * time.Millisecond,
		Timestamp:  time.Now(),
	}

	stats.TrackRequest(metrics)

	pathStat := stats.pathStats["/api/users"]
	if pathStat.TotalErrors != 1 {
		t.Errorf("Expected 1 error, got %d", pathStat.TotalErrors)
	}
	if pathStat.StatusCodeCount[500] != 1 {
		t.Errorf("Expected 1 count for status 500, got %d", pathStat.StatusCodeCount[500])
	}
}

func TestRequestStats_AverageTimeCalculation(t *testing.T) {
	stats := NewRequestStats()
	path := "/api/test"

	// First request - should set average to the duration
	metrics1 := RequestMetrics{
		Path:     path,
		Duration: 100 * time.Millisecond,
	}
	stats.TrackRequest(metrics1)

	pathStat := stats.pathStats[path]
	if pathStat.AverageTime != 100*time.Millisecond {
		t.Errorf("Expected average time 100ms after first request, got %v", pathStat.AverageTime)
	}

	// Second request - should use exponential moving average
	metrics2 := RequestMetrics{
		Path:     path,
		Duration: 200 * time.Millisecond,
	}
	stats.TrackRequest(metrics2)

	// With alpha = 0.1, new average should be: 100*(1-0.1) + 200*0.1 = 90 + 20 = 110ms
	expectedAvg := 110 * time.Millisecond
	if math.Abs(float64(pathStat.AverageTime-expectedAvg)) > float64(time.Millisecond) {
		t.Errorf("Expected average time around %v, got %v", expectedAvg, pathStat.AverageTime)
	}
}

func TestRequestStats_GetRecentRequests(t *testing.T) {
	stats := NewRequestStats()

	// Add some requests
	for i := 0; i < 5; i++ {
		metrics := RequestMetrics{
			Path:   fmt.Sprintf("/api/test%d", i),
			Method: "GET",
		}
		stats.TrackRequest(metrics)
	}

	recent := stats.GetRecentRequests()
	if len(recent) != 5 {
		t.Errorf("Expected 5 recent requests, got %d", len(recent))
	}
}

func TestRequestStats_GetRequestRate(t *testing.T) {
	stats := NewRequestStats()

	// Initially should be 0
	if rate := stats.GetRequestRate(); rate != 0 {
		t.Errorf("Expected initial rate 0, got %f", rate)
	}

	// Add some requests quickly
	for i := 0; i < 10; i++ {
		stats.TrackRequest(RequestMetrics{Path: "/test"})
	}

	// Rate calculation happens internally, but we can check it's accessible
	rate := stats.GetRequestRate()
	if rate < 0 {
		t.Errorf("Expected non-negative rate, got %f", rate)
	}
}

func TestRequestStats_ConcurrentAccess(t *testing.T) {
	stats := NewRequestStats()
	const numGoroutines = 10
	const requestsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Readers and writers

	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				metrics := RequestMetrics{
					Path:       fmt.Sprintf("/api/test%d", routineID),
					Method:     "GET",
					StatusCode: 200,
					Duration:   time.Duration(j) * time.Millisecond,
				}
				stats.TrackRequest(metrics)
			}
		}(i)
	}

	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				_ = stats.GetRequestRate()
				_ = stats.GetRecentRequests()
				time.Sleep(time.Microsecond) // Small delay
			}
		}()
	}

	wg.Wait()

	// Verify final state
	if len(stats.pathStats) != numGoroutines {
		t.Errorf("Expected %d paths, got %d", numGoroutines, len(stats.pathStats))
	}
}

func TestGetStatusString(t *testing.T) {
	testCases := []struct {
		statusCode int
		expected   string
	}{
		{200, "SUCCESS"},
		{201, "SUCCESS"},
		{299, "SUCCESS"},
		{300, "REDIRECT"},
		{301, "REDIRECT"},
		{399, "REDIRECT"},
		{400, "WARN"},
		{404, "WARN"},
		{499, "WARN"},
		{500, "ERROR"},
		{502, "ERROR"},
		{599, "ERROR"},
		{100, "UNKNOWN"},
		{199, "UNKNOWN"},
		{0, "UNKNOWN"},
		{-1, "UNKNOWN"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Status_%d", tc.statusCode), func(t *testing.T) {
			result := GetStatusString(tc.statusCode)
			if result != tc.expected {
				t.Errorf("Expected status string %s for code %d, got %s", tc.expected, tc.statusCode, result)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500.00ms"},
		{1 * time.Second, "1.00s"},
		{1500 * time.Millisecond, "1.50s"},
		{50 * time.Millisecond, "50.00ms"},
		{2 * time.Second, "2.00s"},
		{0, "0.00ms"},
		{time.Microsecond, "0.00ms"},
		{999 * time.Millisecond, "999.00ms"},
		{1001 * time.Millisecond, "1.00s"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Duration_%v", tc.duration), func(t *testing.T) {
			result := FormatDuration(tc.duration)
			if result != tc.expected {
				t.Errorf("Expected formatted duration %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestPathStats(t *testing.T) {
	pathStat := &PathStats{
		StatusCodeCount: make(map[int]int64),
	}

	// Test initial state
	if pathStat.TotalRequests != 0 {
		t.Error("Expected initial TotalRequests to be 0")
	}
	if pathStat.TotalErrors != 0 {
		t.Error("Expected initial TotalErrors to be 0")
	}
	if pathStat.AverageTime != 0 {
		t.Error("Expected initial AverageTime to be 0")
	}
	if pathStat.LastAccessTime.IsZero() == false {
		t.Error("Expected initial LastAccessTime to be zero")
	}
}

func TestRequestMetrics(t *testing.T) {
	timestamp := time.Now()
	duration := 150 * time.Millisecond

	metrics := RequestMetrics{
		Path:          "/api/users/123",
		Method:        "GET",
		StatusCode:    200,
		Duration:      duration,
		Timestamp:     timestamp,
		UserAgent:     "TestAgent/1.0",
		ContentLength: 1024,
		RemoteAddr:    "127.0.0.1:12345",
	}

	// Test all fields are set correctly
	if metrics.Path != "/api/users/123" {
		t.Errorf("Expected path '/api/users/123', got %s", metrics.Path)
	}
	if metrics.Method != "GET" {
		t.Errorf("Expected method 'GET', got %s", metrics.Method)
	}
	if metrics.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", metrics.StatusCode)
	}
	if metrics.Duration != duration {
		t.Errorf("Expected duration %v, got %v", duration, metrics.Duration)
	}
	if !metrics.Timestamp.Equal(timestamp) {
		t.Errorf("Expected timestamp %v, got %v", timestamp, metrics.Timestamp)
	}
	if metrics.UserAgent != "TestAgent/1.0" {
		t.Errorf("Expected user agent 'TestAgent/1.0', got %s", metrics.UserAgent)
	}
	if metrics.ContentLength != 1024 {
		t.Errorf("Expected content length 1024, got %d", metrics.ContentLength)
	}
	if metrics.RemoteAddr != "127.0.0.1:12345" {
		t.Errorf("Expected remote addr '127.0.0.1:12345', got %s", metrics.RemoteAddr)
	}
}

// Benchmark tests
func BenchmarkCircularBuffer_Add(b *testing.B) {
	buffer := NewCircularBuffer(1000)
	metrics := RequestMetrics{Path: "/test", Method: "GET"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Add(metrics)
	}
}

func BenchmarkCircularBuffer_GetAll(b *testing.B) {
	buffer := NewCircularBuffer(1000)

	// Fill buffer
	metrics := RequestMetrics{Path: "/test", Method: "GET"}
	for i := 0; i < 1000; i++ {
		buffer.Add(metrics)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buffer.GetAll()
	}
}

func BenchmarkRequestStats_TrackRequest(b *testing.B) {
	stats := NewRequestStats()
	metrics := RequestMetrics{
		Path:       "/api/benchmark",
		Method:     "GET",
		StatusCode: 200,
		Duration:   100 * time.Millisecond,
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats.TrackRequest(metrics)
	}
}

func BenchmarkGetStatusString(b *testing.B) {
	statusCodes := []int{200, 404, 500, 301}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetStatusString(statusCodes[i%len(statusCodes)])
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	durations := []time.Duration{
		100 * time.Millisecond,
		1 * time.Second,
		500 * time.Millisecond,
		2 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatDuration(durations[i%len(durations)])
	}
}

func BenchmarkRequestStats_ConcurrentTrackRequest(b *testing.B) {
	stats := NewRequestStats()
	metrics := RequestMetrics{
		Path:       "/api/concurrent",
		Method:     "POST",
		StatusCode: 201,
		Duration:   50 * time.Millisecond,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.TrackRequest(metrics)
		}
	})
}

// Example usage tests
func ExampleNewCircularBuffer() {
	buffer := NewCircularBuffer(5)

	for i := 0; i < 7; i++ {
		metrics := RequestMetrics{
			Path:   fmt.Sprintf("/api/test%d", i),
			Method: "GET",
		}
		buffer.Add(metrics)
	}

	items := buffer.GetAll()
	fmt.Printf("Buffer contains %d items\n", len(items))
	// Output: Buffer contains 5 items
}

func ExampleGetStatusString() {
	fmt.Println(GetStatusString(200))
	fmt.Println(GetStatusString(404))
	fmt.Println(GetStatusString(500))
	// Output:
	// SUCCESS
	// WARN
	// ERROR
}

func ExampleFormatDuration() {
	fmt.Println(FormatDuration(500 * time.Millisecond))
	fmt.Println(FormatDuration(1500 * time.Millisecond))
	// Output:
	// 500.00ms
	// 1.50s
}
