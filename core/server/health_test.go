package server

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"
)

func TestHealthResponseStruct(t *testing.T) {
	now := time.Now()
	serverStats := &ServerStats{
		StartTime:     now,
		TotalRequests: atomic.Uint64{},
	}
	systemStats := &monitoring.SystemStats{
		Hostname: "test-host",
		Platform: "linux",
	}

	response := HealthResponse{
		Status:        "healthy",
		ServerStats:   serverStats,
		SystemStats:   systemStats,
		LastCheckTime: now,
	}

	if response.Status != "healthy" {
		t.Errorf("Expected Status 'healthy', got %s", response.Status)
	}
	if response.ServerStats != serverStats {
		t.Errorf("Expected ServerStats to be set correctly")
	}
	if response.SystemStats != systemStats {
		t.Errorf("Expected SystemStats to be set correctly")
	}
	if response.LastCheckTime != now {
		t.Errorf("Expected LastCheckTime %v, got %v", now, response.LastCheckTime)
	}
}

func TestHealthResponseZeroValues(t *testing.T) {
	var response HealthResponse

	if response.Status != "" {
		t.Errorf("Expected empty Status, got %s", response.Status)
	}
	if response.ServerStats != nil {
		t.Errorf("Expected nil ServerStats, got %v", response.ServerStats)
	}
	if response.SystemStats != nil {
		t.Errorf("Expected nil SystemStats, got %v", response.SystemStats)
	}
	if !response.LastCheckTime.IsZero() {
		t.Errorf("Expected zero LastCheckTime, got %v", response.LastCheckTime)
	}
}

func TestTemplateFuncMultiply(t *testing.T) {
	multiply := templateFuncs["multiply"].(func(float64, float64) float64)

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

func TestTemplateFuncDivide(t *testing.T) {
	divide := templateFuncs["divide"].(func(interface{}, interface{}) float64)

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

func TestTemplateFuncDivideFloat64(t *testing.T) {
	divideFloat64 := templateFuncs["divideFloat64"].(func(interface{}, float64) float64)

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

func TestTemplateFuncInt64(t *testing.T) {
	int64Func := templateFuncs["int64"].(func(interface{}) int64)

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

func TestTemplateFuncErrorRate(t *testing.T) {
	errorRate := templateFuncs["errorRate"].(func(uint64, uint64) float64)

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

func TestTemplateFuncAvgCPUUsage(t *testing.T) {
	avgCPUUsage := templateFuncs["avgCPUUsage"].(func([]monitoring.CPUInfo) float64)

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

func TestTemplateFuncFormatBytes(t *testing.T) {
	formatBytes := templateFuncs["formatBytes"].(func(uint64) string)

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

func TestTemplateFuncFormatTime(t *testing.T) {
	formatTime := templateFuncs["formatTime"].(func(time.Time) string)

	testTime := time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC)
	expected := "15:30:45"

	result := formatTime(testTime)
	if result != expected {
		t.Errorf("formatTime() = %s, expected %s", result, expected)
	}
}

func TestTemplateFuncInc(t *testing.T) {
	inc := templateFuncs["inc"].(func(int) int)

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

func TestTemplateFuncGetDiskTemp(t *testing.T) {
	getDiskTemp := templateFuncs["getDiskTemp"].(func(*monitoring.SystemStats) float64)

	// Test with HasTempData = false
	stats := &monitoring.SystemStats{HasTempData: false}
	result := getDiskTemp(stats)
	if result != 0 {
		t.Errorf("Expected 0 for HasTempData=false, got %f", result)
	}

	// Test with HasTempData = true (will return 0 since we can't mock host.SensorsTemperatures easily)
	stats.HasTempData = true
	result = getDiskTemp(stats)
	// We expect 0 because there likely aren't any real disk temperature sensors in test environment
	if result < 0 {
		t.Errorf("Expected non-negative temperature, got %f", result)
	}
}

func TestTemplateFuncGetSystemTemp(t *testing.T) {
	getSystemTemp := templateFuncs["getSystemTemp"].(func(*monitoring.SystemStats) float64)

	// Test with HasTempData = false
	stats := &monitoring.SystemStats{HasTempData: false}
	result := getSystemTemp(stats)
	if result != 0 {
		t.Errorf("Expected 0 for HasTempData=false, got %f", result)
	}

	// Test with HasTempData = true
	stats.HasTempData = true
	result = getSystemTemp(stats)
	if result < 0 {
		t.Errorf("Expected non-negative temperature, got %f", result)
	}
}

func TestTemplateFuncGetAmbientTemp(t *testing.T) {
	getAmbientTemp := templateFuncs["getAmbientTemp"].(func(*monitoring.SystemStats) float64)

	// Test with HasTempData = false
	stats := &monitoring.SystemStats{HasTempData: false}
	result := getAmbientTemp(stats)
	if result != 0 {
		t.Errorf("Expected 0 for HasTempData=false, got %f", result)
	}

	// Test with HasTempData = true
	stats.HasTempData = true
	result = getAmbientTemp(stats)
	if result < 0 {
		t.Errorf("Expected non-negative temperature, got %f", result)
	}
}

func TestTemplateFuncHasDiskTemps(t *testing.T) {
	hasDiskTemps := templateFuncs["hasDiskTemps"].(func(*monitoring.SystemStats) bool)

	// Test with HasTempData = false
	stats := &monitoring.SystemStats{HasTempData: false}
	result := hasDiskTemps(stats)
	if result {
		t.Error("Expected false for HasTempData=false")
	}

	// Test with HasTempData = true
	stats.HasTempData = true
	result = hasDiskTemps(stats)
	// Result can be true or false depending on system, just check it doesn't panic
	_ = result
}

func TestTemplateFunctionsExist(t *testing.T) {
	expectedFunctions := []string{
		"multiply", "divide", "divideFloat64", "int64", "errorRate",
		"avgCPUUsage", "formatBytes", "getDiskTemp", "getSystemTemp",
		"getAmbientTemp", "hasDiskTemps", "formatTime", "inc",
	}

	for _, funcName := range expectedFunctions {
		if _, exists := templateFuncs[funcName]; !exists {
			t.Errorf("Expected template function %s to exist", funcName)
		}
	}
}

func TestTemplateFunctionsInTemplate(t *testing.T) {
	// Test that template functions can be used in actual templates
	tmplText := `
{{multiply 3.0 4.0}}
{{divide 10.0 2.0}}
{{errorRate 50 100}}
{{formatBytes 1024}}
{{inc 5}}
{{formatTime .Time}}
`

	tmpl, err := template.New("test").Funcs(templateFuncs).Parse(tmplText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := struct {
		Time time.Time
	}{
		Time: time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC),
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := buf.String()
	if len(result) == 0 {
		t.Error("Template execution resulted in empty output")
	}

	// Check for expected values in output
	expectedValues := []string{"12", "5", "50", "1.0 KB", "6", "15:30:45"}
	for _, expected := range expectedValues {
		if !bytes.Contains(buf.Bytes(), []byte(expected)) {
			t.Errorf("Expected output to contain '%s', got: %s", expected, result)
		}
	}
}

func TestTemplateParsingError(t *testing.T) {
	// Test what happens when template parsing fails
	invalidTemplate := `{{invalid_function 123}}`

	_, err := template.New("invalid").Funcs(templateFuncs).Parse(invalidTemplate)
	if err == nil {
		t.Error("Expected error when parsing invalid template")
	}
}

func TestTemplateFuncEdgeCases(t *testing.T) {
	// Test division by zero edge case
	divide := templateFuncs["divide"].(func(interface{}, interface{}) float64)
	result := divide(10.0, 0.0)
	if result != 0.0 {
		t.Errorf("Expected 0.0 for division by zero, got %f", result)
	}

	// Test avgCPUUsage with nil slice
	avgCPUUsage := templateFuncs["avgCPUUsage"].(func([]monitoring.CPUInfo) float64)
	result = avgCPUUsage(nil)
	if result != 0.0 {
		t.Errorf("Expected 0.0 for nil slice, got %f", result)
	}

	// Test formatBytes edge cases
	formatBytes := templateFuncs["formatBytes"].(func(uint64) string)
	result_str := formatBytes(1023)
	if result_str != "1023 B" {
		t.Errorf("Expected '1023 B', got %s", result_str)
	}
}

// Benchmark tests
func BenchmarkTemplateFuncMultiply(b *testing.B) {
	multiply := templateFuncs["multiply"].(func(float64, float64) float64)
	for i := 0; i < b.N; i++ {
		_ = multiply(3.5, 2.0)
	}
}

func BenchmarkTemplateFuncFormatBytes(b *testing.B) {
	formatBytes := templateFuncs["formatBytes"].(func(uint64) string)
	for i := 0; i < b.N; i++ {
		_ = formatBytes(1024 * 1024 * 1024)
	}
}

func BenchmarkTemplateFuncErrorRate(b *testing.B) {
	errorRate := templateFuncs["errorRate"].(func(uint64, uint64) float64)
	for i := 0; i < b.N; i++ {
		_ = errorRate(50, 100)
	}
}

func BenchmarkTemplateExecution(b *testing.B) {
	tmplText := `{{multiply 3.0 4.0}} {{formatBytes 1024}}`
	tmpl, err := template.New("bench").Funcs(templateFuncs).Parse(tmplText)
	if err != nil {
		b.Fatalf("Failed to parse template: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, nil)
	}
}

// Example usage
func Example() {
	multiply := templateFuncs["multiply"].(func(float64, float64) float64)
	result := multiply(3.0, 4.0)
	fmt.Printf("3.0 * 4.0 = %.1f", result)
	// Output: 3.0 * 4.0 = 12.0
}

func TestHealthResponseJSONStructure(t *testing.T) {
	// Test that HealthResponse has the expected structure for JSON serialization
	response := HealthResponse{
		Status:        "healthy",
		LastCheckTime: time.Now(),
	}

	// This test primarily checks that the struct is properly structured
	if response.Status == "" {
		t.Error("Status should not be empty")
	}

	// Test that all fields can be set
	response.ServerStats = &ServerStats{}
	response.SystemStats = &monitoring.SystemStats{}

	if response.ServerStats == nil {
		t.Error("ServerStats should be settable")
	}
	if response.SystemStats == nil {
		t.Error("SystemStats should be settable")
	}
}

func TestMockHealthData(t *testing.T) {
	// Test creating mock health data for testing purposes
	mockServerStats := &ServerStats{
		StartTime: time.Now().Add(-time.Hour),
	}
	mockServerStats.TotalRequests.Store(1000)
	mockServerStats.TotalErrors.Store(10)
	mockServerStats.ActiveConnections.Store(5)

	mockSystemStats := &monitoring.SystemStats{
		Hostname:    "test-server",
		Platform:    "linux",
		OS:          "ubuntu",
		HasTempData: true,
	}

	healthData := HealthResponse{
		Status:        "healthy",
		ServerStats:   mockServerStats,
		SystemStats:   mockSystemStats,
		LastCheckTime: time.Now(),
	}

	// Validate the mock data
	if healthData.Status != "healthy" {
		t.Error("Expected healthy status")
	}
	if healthData.ServerStats.TotalRequests.Load() != 1000 {
		t.Error("Expected 1000 total requests")
	}
	if healthData.SystemStats.(*monitoring.SystemStats).Hostname != "test-server" {
		t.Error("Expected test-server hostname")
	}
}
