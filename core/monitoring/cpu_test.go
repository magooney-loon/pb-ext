package monitoring

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCPUInfo(t *testing.T) {
	cpuInfo := CPUInfo{
		ModelName:   "Intel Core i7-9700K",
		Cores:       8,
		Frequency:   3600.0,
		Usage:       45.5,
		Temperature: 65.2,
	}

	if cpuInfo.ModelName != "Intel Core i7-9700K" {
		t.Errorf("Expected ModelName 'Intel Core i7-9700K', got %s", cpuInfo.ModelName)
	}
	if cpuInfo.Cores != 8 {
		t.Errorf("Expected Cores 8, got %d", cpuInfo.Cores)
	}
	if cpuInfo.Frequency != 3600.0 {
		t.Errorf("Expected Frequency 3600.0, got %f", cpuInfo.Frequency)
	}
	if cpuInfo.Usage != 45.5 {
		t.Errorf("Expected Usage 45.5, got %f", cpuInfo.Usage)
	}
	if cpuInfo.Temperature != 65.2 {
		t.Errorf("Expected Temperature 65.2, got %f", cpuInfo.Temperature)
	}
}

func TestCPUInfoZeroValues(t *testing.T) {
	var cpuInfo CPUInfo

	if cpuInfo.ModelName != "" {
		t.Errorf("Expected empty ModelName, got %s", cpuInfo.ModelName)
	}
	if cpuInfo.Cores != 0 {
		t.Errorf("Expected Cores 0, got %d", cpuInfo.Cores)
	}
	if cpuInfo.Frequency != 0 {
		t.Errorf("Expected Frequency 0, got %f", cpuInfo.Frequency)
	}
	if cpuInfo.Usage != 0 {
		t.Errorf("Expected Usage 0, got %f", cpuInfo.Usage)
	}
	if cpuInfo.Temperature != 0 {
		t.Errorf("Expected Temperature 0, got %f", cpuInfo.Temperature)
	}
}

func TestCollectCPUInfoContextCancellation(t *testing.T) {
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectCPUInfoWithContext(ctx)

	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	// Check if it's a timeout error
	if !IsTimeout(err) {
		t.Errorf("Expected timeout error, got %T: %v", err, err)
	}
}

func TestCollectCPUInfoContextTimeout(t *testing.T) {
	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(1 * time.Millisecond)

	_, err := CollectCPUInfoWithContext(ctx)

	if err == nil {
		t.Error("Expected error for timed out context, got nil")
	}

	// Check if it's a timeout error
	if !IsTimeout(err) {
		t.Errorf("Expected timeout error, got %T: %v", err, err)
	}
}

func TestCollectCPUInfo(t *testing.T) {
	// This test will actually try to collect CPU info
	// It may fail on systems without proper CPU info access, so we handle errors gracefully
	cpuInfo, err := CollectCPUInfo()

	if err != nil {
		// Log the error but don't fail the test as this depends on system capabilities
		t.Logf("CollectCPUInfo failed (this may be expected): %v", err)

		// Check that we get appropriate error types
		if !IsSystemError(err) && !IsSensorError(err) {
			t.Errorf("Expected system or sensor error, got %T: %v", err, err)
		}
		return
	}

	// If we successfully got CPU info, validate it
	if len(cpuInfo) == 0 {
		t.Error("Expected at least one CPU info entry")
		return
	}

	for i, info := range cpuInfo {
		t.Logf("CPU %d: %+v", i, info)

		// Basic validation - model name should not be empty on most systems
		if info.ModelName == "" {
			t.Logf("Warning: CPU %d has empty ModelName", i)
		}

		// Cores should be positive
		if info.Cores <= 0 {
			t.Logf("Warning: CPU %d has non-positive core count: %d", i, info.Cores)
		}

		// Frequency should be reasonable (but could be 0 if not available)
		if info.Frequency < 0 {
			t.Errorf("CPU %d has negative frequency: %f", i, info.Frequency)
		}

		// Usage should be between 0 and 100
		if info.Usage < 0 || info.Usage > 100 {
			t.Logf("Warning: CPU %d usage outside expected range: %f", i, info.Usage)
		}

		// Temperature could be 0 if not available
		if info.Temperature < -273 { // Absolute zero check
			t.Errorf("CPU %d has impossible temperature: %f", i, info.Temperature)
		}
	}
}

func TestIsCPUTemp(t *testing.T) {
	testCases := []struct {
		sensor   string
		expected bool
		name     string
	}{
		{"coretemp", true, "Intel coretemp"},
		{"k10temp", true, "AMD k10temp"},
		{"cpu_thermal", true, "Generic cpu_thermal"},
		{"cpu-thermal", true, "Generic cpu-thermal with dash"},
		{"cpu temperature", true, "Generic cpu temperature"},
		{"CORETEMP", true, "Uppercase coretemp"},
		{"CPU_THERMAL", true, "Uppercase cpu_thermal"},
		{"Core 0", false, "Core number (not a sensor type)"},
		{"gpu_thermal", false, "GPU thermal sensor"},
		{"nvme", false, "NVMe sensor"},
		{"acpi", false, "ACPI sensor"},
		{"", false, "Empty string"},
		{"random_sensor", false, "Random sensor name"},
		{"memory_temp", false, "Memory temperature"},
		{"motherboard", false, "Motherboard sensor"},
		{"Package id 0", false, "Package sensor (specific)"},
		{"CPU Core Temp", false, "CPU Core Temp variant"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsCPUTemp(tc.sensor)
			if result != tc.expected {
				t.Errorf("Expected IsCPUTemp(%q) to return %t, got %t", tc.sensor, tc.expected, result)
			}
		})
	}
}

func TestIsCPUTempCaseInsensitive(t *testing.T) {
	// Test case insensitivity more thoroughly
	testCases := []string{
		"coretemp",
		"CORETEMP",
		"CoreTemp",
		"k10temp",
		"K10TEMP",
		"cpu_thermal",
		"CPU_THERMAL",
		"Cpu_Thermal",
	}

	for _, sensor := range testCases {
		t.Run(sensor, func(t *testing.T) {
			if !IsCPUTemp(sensor) {
				t.Errorf("Expected IsCPUTemp(%q) to return true", sensor)
			}
		})
	}
}

func TestIsCPUTempPartialMatch(t *testing.T) {
	// Test partial matches within longer strings
	testCases := []struct {
		sensor   string
		expected bool
	}{
		{"acpi_coretemp", true},
		{"sys_k10temp_input", true},
		{"platform_cpu_thermal", true},
		{"hwmon_cpu-thermal_temp1", true},
		{"some_cpu temperature_sensor", true},
		{"coretemp_is_good", true},
		{"not_core_temp", false}, // Should not match "coretemp"
		{"k10_temp", false},      // Should not match "k10temp"
		{"cpu_temp", false},      // Should not match "cpu_thermal"
	}

	for _, tc := range testCases {
		t.Run(tc.sensor, func(t *testing.T) {
			result := IsCPUTemp(tc.sensor)
			if result != tc.expected {
				t.Errorf("Expected IsCPUTemp(%q) to return %t, got %t", tc.sensor, tc.expected, result)
			}
		})
	}
}

func TestCollectCPUInfoWithValidContext(t *testing.T) {
	// Test with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cpuInfo, err := CollectCPUInfoWithContext(ctx)

	// This test may fail on systems without CPU monitoring capabilities
	if err != nil {
		t.Logf("CollectCPUInfoWithContext failed (may be expected on some systems): %v", err)

		// Verify error type is appropriate
		if !IsSystemError(err) && !IsSensorError(err) && !IsTimeout(err) {
			t.Errorf("Expected system, sensor, or timeout error, got %T: %v", err, err)
		}
		return
	}

	if len(cpuInfo) == 0 {
		t.Error("Expected at least one CPU info entry")
	}
}

func TestCPUInfoStructFields(t *testing.T) {
	// Test that CPUInfo has all expected fields and can be initialized
	info := CPUInfo{
		ModelName:   "Test CPU",
		Cores:       4,
		Frequency:   2400.0,
		Usage:       50.0,
		Temperature: 60.0,
	}

	// Test field access
	if info.ModelName != "Test CPU" {
		t.Error("ModelName field not accessible")
	}
	if info.Cores != 4 {
		t.Error("Cores field not accessible")
	}
	if info.Frequency != 2400.0 {
		t.Error("Frequency field not accessible")
	}
	if info.Usage != 50.0 {
		t.Error("Usage field not accessible")
	}
	if info.Temperature != 60.0 {
		t.Error("Temperature field not accessible")
	}
}

func TestCPUInfoArray(t *testing.T) {
	// Test working with arrays of CPUInfo
	cpuInfos := []CPUInfo{
		{ModelName: "CPU 1", Cores: 4},
		{ModelName: "CPU 2", Cores: 8},
	}

	if len(cpuInfos) != 2 {
		t.Errorf("Expected 2 CPU infos, got %d", len(cpuInfos))
	}

	if cpuInfos[0].ModelName != "CPU 1" {
		t.Errorf("Expected first CPU model 'CPU 1', got %s", cpuInfos[0].ModelName)
	}

	if cpuInfos[1].Cores != 8 {
		t.Errorf("Expected second CPU cores 8, got %d", cpuInfos[1].Cores)
	}
}

// Benchmark tests
func BenchmarkIsCPUTemp(b *testing.B) {
	sensors := []string{
		"coretemp",
		"k10temp",
		"cpu_thermal",
		"random_sensor",
		"gpu_thermal",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sensor := sensors[i%len(sensors)]
		_ = IsCPUTemp(sensor)
	}
}

func BenchmarkCollectCPUInfo(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CollectCPUInfo()
	}
}

func BenchmarkCPUInfoCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CPUInfo{
			ModelName:   "Benchmark CPU",
			Cores:       8,
			Frequency:   3000.0,
			Usage:       50.0,
			Temperature: 65.0,
		}
	}
}

// Test error handling edge cases
func TestCollectCPUInfoErrorHandling(t *testing.T) {
	// Test with background context
	cpuInfo, err := CollectCPUInfo()

	if err != nil {
		// Validate error structure
		t.Logf("Got expected error: %v", err)

		// Check error implements error interface
		errorString := err.Error()
		if errorString == "" {
			t.Error("Error should have non-empty string representation")
		}

		// Check if it's a monitoring error
		var monErr *MonitoringError
		if errors.As(err, &monErr) {
			if monErr.Op == "" {
				t.Error("MonitoringError should have non-empty Op field")
			}
			if monErr.Type == "" {
				t.Error("MonitoringError should have non-empty Type field")
			}
		}
	} else {
		// If no error, validate returned data
		if len(cpuInfo) == 0 {
			t.Error("If no error, should return at least some CPU info")
		}
	}
}

func TestCPUTempSensorEdgeCases(t *testing.T) {
	// Test edge cases for temperature sensor detection
	testCases := []struct {
		sensor string
		desc   string
	}{
		{"", "empty string"},
		{" ", "single space"},
		{"   ", "multiple spaces"},
		{"\t", "tab character"},
		{"\n", "newline character"},
		{"CORETEMP_CORETEMP", "repeated sensor name"},
		{"core temp", "space instead of underscore"},
		{"core-temp", "dash instead of underscore"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Should not panic
			result := IsCPUTemp(tc.sensor)

			// Most of these should return false
			if tc.sensor == "core temp" || strings.Contains(strings.ToLower(tc.sensor), "coretemp") {
				// Some might legitimately match
				t.Logf("IsCPUTemp(%q) = %t", tc.sensor, result)
			} else {
				if result {
					t.Logf("Unexpected match for %q: %t", tc.sensor, result)
				}
			}
		})
	}
}

// Example tests
func ExampleIsCPUTemp() {
	sensors := []string{"coretemp", "k10temp", "gpu_thermal", "random_sensor"}

	for _, sensor := range sensors {
		isCPU := IsCPUTemp(sensor)
		if isCPU {
			println(sensor, "is a CPU temperature sensor")
		}
	}
	// Output varies by system
}

func ExampleCPUInfo() {
	info := CPUInfo{
		ModelName:   "Intel Core i7-9700K",
		Cores:       8,
		Frequency:   3600.0,
		Usage:       45.5,
		Temperature: 65.2,
	}

	println("CPU:", info.ModelName)
	println("Cores:", info.Cores)
	println("Usage:", info.Usage, "%")
}
