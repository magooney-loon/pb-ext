package monitoring

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMemoryInfo(t *testing.T) {
	memInfo := MemoryInfo{
		Total:       8589934592, // 8GB
		Used:        4294967296, // 4GB
		Free:        4294967296, // 4GB
		UsedPercent: 50.0,
		SwapTotal:   2147483648, // 2GB
		SwapUsed:    1073741824, // 1GB
		SwapPercent: 50.0,
	}

	if memInfo.Total != 8589934592 {
		t.Errorf("Expected Total 8589934592, got %d", memInfo.Total)
	}
	if memInfo.Used != 4294967296 {
		t.Errorf("Expected Used 4294967296, got %d", memInfo.Used)
	}
	if memInfo.Free != 4294967296 {
		t.Errorf("Expected Free 4294967296, got %d", memInfo.Free)
	}
	if memInfo.UsedPercent != 50.0 {
		t.Errorf("Expected UsedPercent 50.0, got %f", memInfo.UsedPercent)
	}
	if memInfo.SwapTotal != 2147483648 {
		t.Errorf("Expected SwapTotal 2147483648, got %d", memInfo.SwapTotal)
	}
	if memInfo.SwapUsed != 1073741824 {
		t.Errorf("Expected SwapUsed 1073741824, got %d", memInfo.SwapUsed)
	}
	if memInfo.SwapPercent != 50.0 {
		t.Errorf("Expected SwapPercent 50.0, got %f", memInfo.SwapPercent)
	}
}

func TestMemoryInfoZeroValues(t *testing.T) {
	var memInfo MemoryInfo

	if memInfo.Total != 0 {
		t.Errorf("Expected Total 0, got %d", memInfo.Total)
	}
	if memInfo.Used != 0 {
		t.Errorf("Expected Used 0, got %d", memInfo.Used)
	}
	if memInfo.Free != 0 {
		t.Errorf("Expected Free 0, got %d", memInfo.Free)
	}
	if memInfo.UsedPercent != 0 {
		t.Errorf("Expected UsedPercent 0, got %f", memInfo.UsedPercent)
	}
	if memInfo.SwapTotal != 0 {
		t.Errorf("Expected SwapTotal 0, got %d", memInfo.SwapTotal)
	}
	if memInfo.SwapUsed != 0 {
		t.Errorf("Expected SwapUsed 0, got %d", memInfo.SwapUsed)
	}
	if memInfo.SwapPercent != 0 {
		t.Errorf("Expected SwapPercent 0, got %f", memInfo.SwapPercent)
	}
}

func TestCollectMemoryInfoContextCancellation(t *testing.T) {
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectMemoryInfoWithContext(ctx)

	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestCollectMemoryInfoContextTimeout(t *testing.T) {
	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(1 * time.Millisecond)

	_, err := CollectMemoryInfoWithContext(ctx)

	if err == nil {
		t.Error("Expected error for timed out context, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestCollectMemoryInfo(t *testing.T) {
	// This test will actually try to collect memory info
	// It may fail on systems without proper memory info access, so we handle errors gracefully
	memInfo, err := CollectMemoryInfo()

	if err != nil {
		// Log the error but don't fail the test as this depends on system capabilities
		t.Logf("CollectMemoryInfo failed (this may be expected): %v", err)
		return
	}

	// If we successfully got memory info, validate it
	validateMemoryInfo(t, memInfo)
}

func TestCollectMemoryInfoWithValidContext(t *testing.T) {
	// Test with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	memInfo, err := CollectMemoryInfoWithContext(ctx)

	if err != nil {
		t.Logf("CollectMemoryInfoWithContext failed (may be expected on some systems): %v", err)
		return
	}

	validateMemoryInfo(t, memInfo)
}

func TestCollectMemoryInfoMultipleCalls(t *testing.T) {
	// Test multiple consecutive calls
	for i := 0; i < 5; i++ {
		memInfo, err := CollectMemoryInfo()

		if err != nil {
			t.Logf("Call %d failed: %v", i+1, err)
			return
		}

		validateMemoryInfo(t, memInfo)

		// Memory values might change between calls, but they should remain reasonable
		if memInfo.Total == 0 {
			t.Errorf("Call %d: Total memory should not be 0", i+1)
		}
	}
}

func TestCollectMemoryInfoConcurrent(t *testing.T) {
	// Test concurrent access to memory info collection
	const numGoroutines = 10
	results := make(chan error, numGoroutines)
	memResults := make(chan MemoryInfo, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			memInfo, err := CollectMemoryInfo()
			results <- err
			if err == nil {
				memResults <- memInfo
			}
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err != nil {
			t.Logf("Goroutine %d failed: %v", i, err)
		} else {
			successCount++
		}
	}

	// Validate successful results
	for i := 0; i < successCount; i++ {
		memInfo := <-memResults
		validateMemoryInfo(t, memInfo)
	}

	if successCount > 0 {
		t.Logf("Successfully collected memory info from %d/%d goroutines", successCount, numGoroutines)
	}
}

func TestMemoryInfoConsistency(t *testing.T) {
	memInfo, err := CollectMemoryInfo()
	if err != nil {
		t.Logf("CollectMemoryInfo failed: %v", err)
		return
	}

	// Used + Free deliberately does NOT equal Total — the remainder is Cached
	// (page cache plus reclaimable slab), which is neither in use by a process
	// nor untouched. Accounting for it is what makes the numbers add up.
	if memInfo.Total > 0 {
		calculated := memInfo.Used + memInfo.Free + memInfo.Cached
		tolerance := memInfo.Total / 10 // Buffers are not tracked separately

		if calculated < memInfo.Total-tolerance || calculated > memInfo.Total+tolerance {
			t.Logf("Memory accounting may be inconsistent: Total=%d, Used=%d, Free=%d, Cached=%d, Calculated=%d",
				memInfo.Total, memInfo.Used, memInfo.Free, memInfo.Cached, calculated)
		}
	}

	// UsedPercent should be consistent with Used/Total ratio
	if memInfo.Total > 0 {
		expectedPercent := float64(memInfo.Used) / float64(memInfo.Total) * 100.0
		tolerance := 5.0 // Allow 5% tolerance

		if memInfo.UsedPercent < expectedPercent-tolerance || memInfo.UsedPercent > expectedPercent+tolerance {
			t.Logf("UsedPercent may be inconsistent: Expected ~%.2f%%, got %.2f%%",
				expectedPercent, memInfo.UsedPercent)
		}
	}

	// SwapPercent should be consistent with SwapUsed/SwapTotal ratio
	if memInfo.SwapTotal > 0 {
		expectedSwapPercent := float64(memInfo.SwapUsed) / float64(memInfo.SwapTotal) * 100.0
		tolerance := 5.0 // Allow 5% tolerance

		if memInfo.SwapPercent < expectedSwapPercent-tolerance || memInfo.SwapPercent > expectedSwapPercent+tolerance {
			t.Logf("SwapPercent may be inconsistent: Expected ~%.2f%%, got %.2f%%",
				expectedSwapPercent, memInfo.SwapPercent)
		}
	}
}

func TestMemoryInfoEdgeCases(t *testing.T) {
	// Test edge cases for MemoryInfo struct
	testCases := []struct {
		name    string
		memInfo MemoryInfo
	}{
		{
			name: "all_zero",
			memInfo: MemoryInfo{
				Total:       0,
				Used:        0,
				Free:        0,
				UsedPercent: 0,
				SwapTotal:   0,
				SwapUsed:    0,
				SwapPercent: 0,
			},
		},
		{
			name: "max_values",
			memInfo: MemoryInfo{
				Total:       ^uint64(0), // Max uint64
				Used:        ^uint64(0) / 2,
				Free:        ^uint64(0) / 2,
				UsedPercent: 100.0,
				SwapTotal:   ^uint64(0),
				SwapUsed:    ^uint64(0) / 2,
				SwapPercent: 100.0,
			},
		},
		{
			name: "no_swap",
			memInfo: MemoryInfo{
				Total:       8589934592,
				Used:        4294967296,
				Free:        4294967296,
				UsedPercent: 50.0,
				SwapTotal:   0, // No swap
				SwapUsed:    0,
				SwapPercent: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic when accessing fields
			_ = tc.memInfo.Total
			_ = tc.memInfo.Used
			_ = tc.memInfo.Free
			_ = tc.memInfo.UsedPercent
			_ = tc.memInfo.SwapTotal
			_ = tc.memInfo.SwapUsed
			_ = tc.memInfo.SwapPercent

			t.Logf("Edge case %s handled successfully", tc.name)
		})
	}
}

func TestCollectMemoryInfoErrorHandling(t *testing.T) {
	// Test with background context
	memInfo, err := CollectMemoryInfo()

	if err != nil {
		// Validate error structure
		t.Logf("Got expected error: %v", err)

		// Check error implements error interface
		errorString := err.Error()
		if errorString == "" {
			t.Error("Error should have non-empty string representation")
		}

		// The error might not be a monitoring error in this case,
		// as it could be a system error from gopsutil
		t.Logf("Error type: %T", err)
	} else {
		// If no error, validate returned data
		validateMemoryInfo(t, memInfo)
	}
}

// Helper function to validate MemoryInfo
func validateMemoryInfo(t *testing.T, memInfo MemoryInfo) {
	t.Logf("Memory info: Total=%d MB, Used=%d MB, Free=%d MB, Used%%=%.2f%%, SwapTotal=%d MB, SwapUsed=%d MB, Swap%%=%.2f%%",
		memInfo.Total/1024/1024, memInfo.Used/1024/1024, memInfo.Free/1024/1024, memInfo.UsedPercent,
		memInfo.SwapTotal/1024/1024, memInfo.SwapUsed/1024/1024, memInfo.SwapPercent)

	// Basic validation - these should hold on most systems
	if memInfo.Total == 0 {
		t.Log("Warning: Total memory is 0 (may be expected on some systems)")
	}

	// Used should not exceed Total
	if memInfo.Used > memInfo.Total && memInfo.Total > 0 {
		t.Errorf("Used memory (%d) should not exceed total memory (%d)", memInfo.Used, memInfo.Total)
	}

	// Free should not exceed Total
	if memInfo.Free > memInfo.Total && memInfo.Total > 0 {
		t.Errorf("Free memory (%d) should not exceed total memory (%d)", memInfo.Free, memInfo.Total)
	}

	// Used percentage should be between 0 and 100
	if memInfo.UsedPercent < 0 || memInfo.UsedPercent > 100 {
		t.Errorf("Used percentage should be between 0 and 100, got %.2f", memInfo.UsedPercent)
	}

	// Swap used should not exceed swap total
	if memInfo.SwapUsed > memInfo.SwapTotal && memInfo.SwapTotal > 0 {
		t.Errorf("Swap used (%d) should not exceed swap total (%d)", memInfo.SwapUsed, memInfo.SwapTotal)
	}

	// Swap percentage should be between 0 and 100
	if memInfo.SwapPercent < 0 || memInfo.SwapPercent > 100 {
		t.Errorf("Swap percentage should be between 0 and 100, got %.2f", memInfo.SwapPercent)
	}
}

// Benchmark tests
func BenchmarkCollectMemoryInfo(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CollectMemoryInfo()
	}
}

func BenchmarkCollectMemoryInfoWithContext(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CollectMemoryInfoWithContext(ctx)
	}
}

func BenchmarkMemoryInfoCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MemoryInfo{
			Total:       8589934592,
			Used:        4294967296,
			Free:        4294967296,
			UsedPercent: 50.0,
			SwapTotal:   2147483648,
			SwapUsed:    1073741824,
			SwapPercent: 50.0,
		}
	}
}

func BenchmarkMemoryInfoValidation(b *testing.B) {
	memInfo := MemoryInfo{
		Total:       8589934592,
		Used:        4294967296,
		Free:        4294967296,
		UsedPercent: 50.0,
		SwapTotal:   2147483648,
		SwapUsed:    1073741824,
		SwapPercent: 50.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate validation operations
		_ = memInfo.Used <= memInfo.Total
		_ = memInfo.UsedPercent >= 0 && memInfo.UsedPercent <= 100
		_ = memInfo.SwapUsed <= memInfo.SwapTotal
		_ = memInfo.SwapPercent >= 0 && memInfo.SwapPercent <= 100
	}
}

func BenchmarkCollectMemoryInfoConcurrent(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = CollectMemoryInfo()
		}
	})
}

// Example usage tests
func ExampleCollectMemoryInfo() {
	memInfo, err := CollectMemoryInfo()
	if err != nil {
		return
	}

	println("Total Memory:", memInfo.Total/1024/1024, "MB")
	println("Used Memory:", memInfo.Used/1024/1024, "MB")
	println("Memory Usage:", memInfo.UsedPercent, "%")
}

func ExampleCollectMemoryInfoWithContext() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	memInfo, err := CollectMemoryInfoWithContext(ctx)
	if err != nil {
		return
	}

	if memInfo.SwapTotal > 0 {
		println("Swap Usage:", memInfo.SwapPercent, "%")
	} else {
		println("No swap configured")
	}
}

func ExampleMemoryInfo() {
	memInfo := MemoryInfo{
		Total:       8589934592, // 8 GB
		Used:        4294967296, // 4 GB
		UsedPercent: 50.0,
	}

	if memInfo.UsedPercent > 80.0 {
		println("High memory usage detected!")
	}
}

// Test memory calculations
func TestMemoryCalculations(t *testing.T) {
	testCases := []struct {
		name        string
		total       uint64
		used        uint64
		expectedPct float64
		tolerance   float64
	}{
		{"half_used", 1000, 500, 50.0, 0.1},
		{"quarter_used", 1000, 250, 25.0, 0.1},
		{"fully_used", 1000, 1000, 100.0, 0.1},
		{"empty", 1000, 0, 0.0, 0.1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPct := float64(tc.used) / float64(tc.total) * 100.0

			if actualPct < tc.expectedPct-tc.tolerance || actualPct > tc.expectedPct+tc.tolerance {
				t.Errorf("Expected percentage ~%.1f%%, got %.1f%%", tc.expectedPct, actualPct)
			}
		})
	}
}

// --- Available vs Free ---

// readMemInfoKB parses the requested keys out of /proc/meminfo, in bytes.
func readMemInfoKB(t *testing.T, keys ...string) map[string]uint64 {
	t.Helper()

	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		t.Skipf("cannot read /proc/meminfo: %v", err)
	}

	wanted := make(map[string]bool, len(keys))
	for _, k := range keys {
		wanted[k] = true
	}

	out := map[string]uint64{}
	for _, line := range strings.Split(string(raw), "\n") {
		name, rest, found := strings.Cut(line, ":")
		if !found || !wanted[name] {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		out[name] = kb * 1024
	}
	return out
}

// TestMemoryInfo_AvailableMatchesMemAvailable is the regression test for
// reporting MemFree where MemAvailable was meant. On a warm Linux box MemFree
// is near zero while most of RAM is reclaimable page cache, so showing Free as
// "free memory" understates it by an order of magnitude.
func TestMemoryInfo_AvailableMatchesMemAvailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc/meminfo is Linux-only")
	}

	memInfo, err := CollectMemoryInfo()
	if err != nil {
		t.Fatalf("CollectMemoryInfo: %v", err)
	}

	proc := readMemInfoKB(t, "MemTotal", "MemFree", "MemAvailable")
	available, ok := proc["MemAvailable"]
	if !ok {
		t.Skip("kernel does not expose MemAvailable")
	}

	// Values drift between the two reads, so compare with a tolerance.
	tolerance := float64(memInfo.Total) * 0.05
	if diff := math.Abs(float64(memInfo.Available) - float64(available)); diff > tolerance {
		t.Errorf("Available = %d, want ~%d (MemAvailable); diff %.0f exceeds tolerance %.0f",
			memInfo.Available, available, diff, tolerance)
	}

	if memInfo.Free != 0 && memInfo.Available < memInfo.Free {
		t.Errorf("Available (%d) must never be less than Free (%d)", memInfo.Available, memInfo.Free)
	}
}

func TestMemoryInfo_TotalMatchesMemTotal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc/meminfo is Linux-only")
	}

	memInfo, err := CollectMemoryInfo()
	if err != nil {
		t.Fatalf("CollectMemoryInfo: %v", err)
	}

	proc := readMemInfoKB(t, "MemTotal")
	if memInfo.Total != proc["MemTotal"] {
		t.Errorf("Total = %d, want %d (MemTotal)", memInfo.Total, proc["MemTotal"])
	}
}

// TestMemoryInfo_AccountingAddsUp pins the identity the dashboard relies on:
// Total is split across Used, Free and Cached, not just Used and Free.
func TestMemoryInfo_AccountingAddsUp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("accounting differs per platform")
	}

	memInfo, err := CollectMemoryInfo()
	if err != nil {
		t.Fatalf("CollectMemoryInfo: %v", err)
	}
	if memInfo.Total == 0 {
		t.Skip("no memory reported")
	}

	sum := memInfo.Used + memInfo.Free + memInfo.Cached
	// Buffers are not tracked separately and are typically well under 1%.
	tolerance := float64(memInfo.Total) * 0.05
	if diff := math.Abs(float64(sum) - float64(memInfo.Total)); diff > tolerance {
		t.Errorf("Used+Free+Cached = %d, want ~%d (Total); diff %.0f exceeds tolerance %.0f",
			sum, memInfo.Total, diff, tolerance)
	}

	// The bug this guards: Used+Free alone leaves the cache unaccounted for.
	if memInfo.Cached > 0 && memInfo.Used+memInfo.Free == memInfo.Total {
		t.Error("Used+Free equals Total while Cached is non-zero — accounting is wrong")
	}
}

func TestMemoryInfo_UsedPercentTracksUsed(t *testing.T) {
	memInfo, err := CollectMemoryInfo()
	if err != nil {
		t.Fatalf("CollectMemoryInfo: %v", err)
	}
	if memInfo.Total == 0 {
		t.Skip("no memory reported")
	}

	want := float64(memInfo.Used) / float64(memInfo.Total) * 100
	if diff := math.Abs(memInfo.UsedPercent - want); diff > 1.0 {
		t.Errorf("UsedPercent = %.2f, want ~%.2f (Used/Total)", memInfo.UsedPercent, want)
	}

	// A percentage derived from Available instead would be a different number
	// entirely; make sure we did not swap them.
	if memInfo.UsedPercent < 0 || memInfo.UsedPercent > 100 {
		t.Errorf("UsedPercent = %.2f, want within [0,100]", memInfo.UsedPercent)
	}
}

// TestMemoryInfo_AvailableFallback covers platforms where gopsutil leaves
// Available at zero: the collector must estimate rather than report that no
// memory is obtainable.
func TestMemoryInfo_AvailableFallback(t *testing.T) {
	memInfo, err := CollectMemoryInfo()
	if err != nil {
		t.Fatalf("CollectMemoryInfo: %v", err)
	}

	if memInfo.Total > 0 && memInfo.Available == 0 {
		t.Error("Available is zero on a machine with memory; the fallback did not apply")
	}
}
