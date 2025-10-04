package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
)

// Mock atomic type for testing
type MockAtomic struct {
	value uint64
}

func (a *MockAtomic) Add(delta uint64) {
	a.value += delta
}

func (a *MockAtomic) Load() uint64 {
	return a.value
}

func (a *MockAtomic) Store(val uint64) {
	a.value = val
}

// Standalone template functions for testing
var testTemplateFuncs = map[string]interface{}{
	"multiply": func(a, b float64) float64 {
		return a * b
	},
	"divide": func(a, b interface{}) float64 {
		var af, bf float64

		switch v := a.(type) {
		case float64:
			af = v
		case uint64:
			af = float64(v)
		default:
			return 0
		}

		switch v := b.(type) {
		case float64:
			bf = v
		case uint64:
			bf = float64(v)
		default:
			return 0
		}

		if bf == 0 {
			return 0
		}
		return af / bf
	},
	"divideFloat64": func(a interface{}, b float64) float64 {
		if b == 0 {
			return 0
		}

		var af float64
		switch v := a.(type) {
		case float64:
			af = v
		case uint64:
			af = float64(v)
		case int64:
			af = float64(v)
		case int:
			af = float64(v)
		default:
			return 0
		}

		return af / b
	},
	"int64": func(v interface{}) int64 {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case float64:
			return int64(val)
		case time.Duration:
			return int64(val)
		default:
			return 0
		}
	},
	"errorRate": func(errors, total uint64) float64 {
		if total == 0 {
			return 0
		}
		return float64(errors) * 100 / float64(total)
	},
	"avgCPUUsage": func(cpus []monitoring.CPUInfo) float64 {
		if len(cpus) == 0 {
			return 0
		}
		var total float64
		for _, cpu := range cpus {
			total += cpu.Usage
		}
		return total / float64(len(cpus))
	},
	"formatBytes": func(bytes uint64) string {
		const unit = 1024
		if bytes < unit {
			return fmt.Sprintf("%d B", bytes)
		}
		div, exp := uint64(unit), 0
		for n := bytes / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
	},
	"formatTime": func(t time.Time) string {
		return t.Format("15:04:05")
	},
	"inc": func(i int) int {
		return i + 1
	},
}

func TestStandaloneTemplateFuncMultiply(t *testing.T) {
	multiply := testTemplateFuncs["multiply"].(func(float64, float64) float64)

	testCases := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"positive numbers", 3.5, 2.0, 7.0},
		{"zero multiplication", 5.0, 0.0, 0.0},
		{"negative numbers", -2.5, 4.0, -10.0},
		{"both negative", -3.0, -4.0, 12.0},
		{"decimal precision", 0.5, 0.4, 0.2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := multiply(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("multiply(%f, %f) = %f, expected %f", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncDivide(t *testing.T) {
	divide := testTemplateFuncs["divide"].(func(interface{}, interface{}) float64)

	testCases := []struct {
		name     string
		a, b     interface{}
		expected float64
	}{
		{"float64 division", 10.0, 2.0, 5.0},
		{"uint64 division", uint64(20), uint64(4), 5.0},
		{"mixed types", 15.0, uint64(3), 5.0},
		{"division by zero", 10.0, 0.0, 0.0},
		{"invalid type a", "invalid", 2.0, 0.0},
		{"invalid type b", 10.0, "invalid", 0.0},
		{"both invalid types", "invalid", "invalid", 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := divide(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("divide(%v, %v) = %f, expected %f", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncDivideFloat64(t *testing.T) {
	divideFloat64 := testTemplateFuncs["divideFloat64"].(func(interface{}, float64) float64)

	testCases := []struct {
		name     string
		a        interface{}
		b        float64
		expected float64
	}{
		{"float64 division", 10.0, 2.0, 5.0},
		{"uint64 division", uint64(20), 4.0, 5.0},
		{"int64 division", int64(15), 3.0, 5.0},
		{"int division", 12, 4.0, 3.0},
		{"division by zero", 10.0, 0.0, 0.0},
		{"invalid type", "invalid", 2.0, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := divideFloat64(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("divideFloat64(%v, %f) = %f, expected %f", tc.a, tc.b, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncInt64(t *testing.T) {
	int64Func := testTemplateFuncs["int64"].(func(interface{}) int64)

	testCases := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"int64 input", int64(42), 42},
		{"int input", 35, 35},
		{"float64 input", 3.14, 3},
		{"time.Duration input", time.Second, int64(time.Second)},
		{"invalid input", "invalid", 0},
		{"nil input", nil, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := int64Func(tc.input)
			if result != tc.expected {
				t.Errorf("int64(%v) = %d, expected %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncErrorRate(t *testing.T) {
	errorRate := testTemplateFuncs["errorRate"].(func(uint64, uint64) float64)

	testCases := []struct {
		name     string
		errors   uint64
		total    uint64
		expected float64
	}{
		{"no errors", 0, 100, 0.0},
		{"50% error rate", 50, 100, 50.0},
		{"100% error rate", 100, 100, 100.0},
		{"division by zero", 50, 0, 0.0},
		{"more errors than total", 150, 100, 150.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := errorRate(tc.errors, tc.total)
			if result != tc.expected {
				t.Errorf("errorRate(%d, %d) = %f, expected %f", tc.errors, tc.total, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncAvgCPUUsage(t *testing.T) {
	avgCPUUsage := testTemplateFuncs["avgCPUUsage"].(func([]monitoring.CPUInfo) float64)

	testCases := []struct {
		name     string
		cpus     []monitoring.CPUInfo
		expected float64
	}{
		{
			name:     "empty slice",
			cpus:     []monitoring.CPUInfo{},
			expected: 0.0,
		},
		{
			name: "single CPU",
			cpus: []monitoring.CPUInfo{
				{Usage: 50.0},
			},
			expected: 50.0,
		},
		{
			name: "multiple CPUs",
			cpus: []monitoring.CPUInfo{
				{Usage: 20.0},
				{Usage: 40.0},
				{Usage: 60.0},
				{Usage: 80.0},
			},
			expected: 50.0,
		},
		{
			name: "zero usage",
			cpus: []monitoring.CPUInfo{
				{Usage: 0.0},
				{Usage: 0.0},
			},
			expected: 0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := avgCPUUsage(tc.cpus)
			if result != tc.expected {
				t.Errorf("avgCPUUsage() = %f, expected %f", result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncFormatBytes(t *testing.T) {
	formatBytes := testTemplateFuncs["formatBytes"].(func(uint64) string)

	testCases := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"megabytes", 1024 * 1024, "1.0 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"terabytes", 1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{"mixed size", 1536, "1.5 KB"},
		{"large number", 5 * 1024 * 1024 * 1024, "5.0 GB"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatBytes(tc.bytes)
			if result != tc.expected {
				t.Errorf("formatBytes(%d) = %s, expected %s", tc.bytes, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFuncFormatTime(t *testing.T) {
	formatTime := testTemplateFuncs["formatTime"].(func(time.Time) string)

	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
	expected := "15:30:45"

	result := formatTime(testTime)
	if result != expected {
		t.Errorf("formatTime() = %s, expected %s", result, expected)
	}
}

func TestStandaloneTemplateFuncInc(t *testing.T) {
	inc := testTemplateFuncs["inc"].(func(int) int)

	testCases := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive number", 5, 6},
		{"zero", 0, 1},
		{"negative number", -3, -2},
		{"large number", 999, 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := inc(tc.input)
			if result != tc.expected {
				t.Errorf("inc(%d) = %d, expected %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestStandaloneTemplateFunctionsExist(t *testing.T) {
	expectedFunctions := []string{
		"multiply", "divide", "divideFloat64", "int64", "errorRate",
		"avgCPUUsage", "formatBytes", "formatTime", "inc",
	}

	for _, funcName := range expectedFunctions {
		if _, exists := testTemplateFuncs[funcName]; !exists {
			t.Errorf("Expected template function %s to exist", funcName)
		}
	}
}

// Benchmark tests
func BenchmarkStandaloneTemplateFuncMultiply(b *testing.B) {
	multiply := testTemplateFuncs["multiply"].(func(float64, float64) float64)
	for i := 0; i < b.N; i++ {
		_ = multiply(3.5, 2.0)
	}
}

func BenchmarkStandaloneTemplateFuncFormatBytes(b *testing.B) {
	formatBytes := testTemplateFuncs["formatBytes"].(func(uint64) string)
	for i := 0; i < b.N; i++ {
		_ = formatBytes(1024 * 1024 * 1024)
	}
}

func BenchmarkStandaloneTemplateFuncErrorRate(b *testing.B) {
	errorRate := testTemplateFuncs["errorRate"].(func(uint64, uint64) float64)
	for i := 0; i < b.N; i++ {
		_ = errorRate(50, 100)
	}
}

// Example usage would be:
// multiply := testTemplateFuncs["multiply"].(func(float64, float64) float64)
// result := multiply(3.0, 4.0)
// Output: 12.0

func TestStandaloneTemplateFuncEdgeCases(t *testing.T) {
	// Test division by zero edge case
	divide := testTemplateFuncs["divide"].(func(interface{}, interface{}) float64)
	result := divide(10.0, 0.0)
	if result != 0.0 {
		t.Errorf("Expected 0.0 for division by zero, got %f", result)
	}

	// Test avgCPUUsage with nil slice
	avgCPUUsage := testTemplateFuncs["avgCPUUsage"].(func([]monitoring.CPUInfo) float64)
	result = avgCPUUsage(nil)
	if result != 0.0 {
		t.Errorf("Expected 0.0 for nil slice, got %f", result)
	}

	// Test formatBytes edge cases
	formatBytes := testTemplateFuncs["formatBytes"].(func(uint64) string)
	result_str := formatBytes(1023)
	if result_str != "1023 B" {
		t.Errorf("Expected '1023 B', got %s", result_str)
	}
}

func TestStandaloneServerStats(t *testing.T) {
	// Test the ServerStats struct that would be used with templates
	stats := &ServerStats{
		StartTime: time.Now(),
	}

	// Test atomic operations
	stats.TotalRequests.Add(100)
	stats.TotalErrors.Add(5)
	stats.ActiveConnections.Add(3)

	if stats.TotalRequests.Load() != 100 {
		t.Errorf("Expected 100 requests, got %d", stats.TotalRequests.Load())
	}
	if stats.TotalErrors.Load() != 5 {
		t.Errorf("Expected 5 errors, got %d", stats.TotalErrors.Load())
	}
	if stats.ActiveConnections.Load() != 3 {
		t.Errorf("Expected 3 connections, got %d", stats.ActiveConnections.Load())
	}

	// Test error rate calculation using template function
	errorRate := testTemplateFuncs["errorRate"].(func(uint64, uint64) float64)
	rate := errorRate(stats.TotalErrors.Load(), stats.TotalRequests.Load())
	expectedRate := 5.0 // 5% error rate
	if rate != expectedRate {
		t.Errorf("Expected error rate %f, got %f", expectedRate, rate)
	}
}
