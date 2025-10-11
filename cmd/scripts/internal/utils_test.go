package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// captureOutput captures stdout during function execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestPrintBanner tests the PrintBanner function
func TestPrintBanner(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		contains  []string
	}{
		{
			name:      "Development operation",
			operation: "DEVELOPMENT",
			contains:  []string{"▲", AppName, "v1.0.0", "development"},
		},
		{
			name:      "Production operation",
			operation: "PRODUCTION",
			contains:  []string{"▲", AppName, "v1.0.0", "production"},
		},
		{
			name:      "Testing operation",
			operation: "TESTING",
			contains:  []string{"▲", AppName, "v1.0.0", "testing"},
		},
		{
			name:      "Empty operation",
			operation: "",
			contains:  []string{"▲", AppName, "v1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintBanner(tt.operation)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintBanner output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestPrintHeader tests the PrintHeader function
func TestPrintHeader(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "Simple header",
			title:    "TEST HEADER",
			expected: "TEST HEADER",
		},
		{
			name:     "Header with emojis",
			title:    "🚀 PRODUCTION MODE",
			expected: "🚀 PRODUCTION MODE",
		},
		{
			name:     "Empty header",
			title:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintHeader(tt.title)
			})

			if !strings.Contains(output, tt.expected) {
				t.Errorf("PrintHeader output should contain '%s', got: %s", tt.expected, output)
			}
		})
	}
}

// TestPrintStep tests the PrintStep function
func TestPrintStep(t *testing.T) {
	tests := []struct {
		name     string
		emoji    string
		format   string
		args     []any
		contains []string
	}{
		{
			name:     "Simple step",
			emoji:    "🔍",
			format:   "Checking requirements...",
			args:     []any{},
			contains: []string{"🔍", "Checking requirements..."},
		},
		{
			name:     "Step with formatting",
			emoji:    "📦",
			format:   "Installing %s version %s",
			args:     []any{"package", "1.0.0"},
			contains: []string{"📦", "Installing package version 1.0.0"},
		},
		{
			name:     "Empty emoji",
			emoji:    "",
			format:   "No emoji step",
			args:     []any{},
			contains: []string{"No emoji step"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintStep(tt.emoji, tt.format, tt.args...)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintStep output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestPrintSuccess tests the PrintSuccess function
func TestPrintSuccess(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []any
		contains []string
	}{
		{
			name:     "Simple success",
			format:   "Build completed",
			args:     []any{},
			contains: []string{"✓", "Build completed"},
		},
		{
			name:     "Success with formatting",
			format:   "Processed %d files",
			args:     []any{42},
			contains: []string{"✓", "Processed 42 files"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintSuccess(tt.format, tt.args...)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintSuccess output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestPrintError tests the PrintError function
func TestPrintError(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []any
		contains []string
	}{
		{
			name:     "Simple error",
			format:   "Build failed",
			args:     []any{},
			contains: []string{"✗", "Build failed"},
		},
		{
			name:     "Error with formatting",
			format:   "Failed to process %s",
			args:     []any{"package.json"},
			contains: []string{"✗", "Failed to process package.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintError(tt.format, tt.args...)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintError output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestPrintWarning tests the PrintWarning function
func TestPrintWarning(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []any
		contains []string
	}{
		{
			name:     "Simple warning",
			format:   "Deprecated feature used",
			args:     []any{},
			contains: []string{"⚠", "Deprecated feature used"},
		},
		{
			name:     "Warning with formatting",
			format:   "Package %s is outdated",
			args:     []any{"lodash"},
			contains: []string{"⚠", "Package lodash is outdated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintWarning(tt.format, tt.args...)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintWarning output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestPrintInfo tests the PrintInfo function
func TestPrintInfo(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []any
		contains []string
	}{
		{
			name:     "Simple info",
			format:   "Starting server",
			args:     []any{},
			contains: []string{"•", "Starting server"},
		},
		{
			name:     "Info with formatting",
			format:   "Listening on port %d",
			args:     []any{8080},
			contains: []string{"•", "Listening on port 8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintInfo(tt.format, tt.args...)
			})

			if !strings.Contains(output, "•") {
				t.Errorf("PrintInfo output should contain '•', got: %s", output)
			}
		})
	}
}

// TestPrintBuildSummary tests the PrintBuildSummary function
func TestPrintBuildSummary(t *testing.T) {
	duration := 500 * time.Millisecond

	tests := []struct {
		name         string
		duration     time.Duration
		isProduction bool
		contains     []string
	}{
		{
			name:         "Development build summary",
			duration:     duration,
			isProduction: false,
			contains: []string{
				"Build Complete",
				"Type:", "Development",
				"Duration:", "500ms",
				"Target:", runtime.GOOS + "/" + runtime.GOARCH,
				"pb_public/", "development build",
			},
		},
		{
			name:         "Production build summary",
			duration:     duration,
			isProduction: true,
			contains: []string{
				"Build Complete",
				"Type:", "Production",
				"Duration:", "500ms",
				"Target:", runtime.GOOS + "/" + runtime.GOARCH,
				"dist/", "production build",
			},
		},
		{
			name:         "Zero duration",
			duration:     0,
			isProduction: false,
			contains: []string{
				"Build Complete",
				"Duration:", "0s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintBuildSummary(tt.duration, tt.isProduction)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintBuildSummary output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestPrintTestSummary tests the PrintTestSummary function
func TestPrintTestSummary(t *testing.T) {
	duration := 1200 * time.Millisecond

	tests := []struct {
		name     string
		duration time.Duration
		contains []string
	}{
		{
			name:     "Test summary",
			duration: duration,
			contains: []string{
				"Test Suite Complete",
				"Type:", "Testing",
				"Duration:", "1.2s",
				"Target:", runtime.GOOS + "/" + runtime.GOARCH,
				"Output:",
			},
		},
		{
			name:     "Zero duration test summary",
			duration: 0,
			contains: []string{
				"Test Suite Complete",
				"Duration:", "0s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintTestSummary(tt.duration)
			})

			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("PrintTestSummary output should contain '%s', got: %s", expected, output)
				}
			}
		})
	}
}

// TestConstants tests that constants are defined correctly
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "App name",
			constant: AppName,
			expected: "pb-cli",
		},
		{
			name:     "Reset color",
			constant: Reset,
			expected: "\033[0m",
		},
		{
			name:     "Red color",
			constant: Red,
			expected: "\033[31m",
		},
		{
			name:     "Green color",
			constant: Green,
			expected: "\033[32m",
		},
		{
			name:     "Yellow color",
			constant: Yellow,
			expected: "\033[33m",
		},
		{
			name:     "Blue color",
			constant: Blue,
			expected: "\033[34m",
		},
		{
			name:     "Purple color",
			constant: Purple,
			expected: "\033[35m",
		},
		{
			name:     "Cyan color",
			constant: Cyan,
			expected: "\033[36m",
		},
		{
			name:     "Gray color",
			constant: Gray,
			expected: "\033[37m",
		},
		{
			name:     "Bold color",
			constant: Bold,
			expected: "\033[1m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected constant %s to be %q, got %q", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

// TestPrintFunctionsWithEmptyInput tests print functions with empty input
func TestPrintFunctionsWithEmptyInput(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "PrintHeader with empty string",
			fn:   func() { PrintHeader("") },
		},
		{
			name: "PrintStep with empty format",
			fn:   func() { PrintStep("🔍", "") },
		},
		{
			name: "PrintSuccess with empty format",
			fn:   func() { PrintSuccess("") },
		},
		{
			name: "PrintError with empty format",
			fn:   func() { PrintError("") },
		},
		{
			name: "PrintWarning with empty format",
			fn:   func() { PrintWarning("") },
		},
		{
			name: "PrintInfo with empty format",
			fn:   func() { PrintInfo("") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure functions don't panic with empty input
			output := captureOutput(tt.fn)
			if output == "" {
				t.Log("Function produced no output (expected for empty input)")
			}
		})
	}
}

// TestPrintFunctionsWithSpecialCharacters tests functions with special characters
func TestPrintFunctionsWithSpecialCharacters(t *testing.T) {
	specialChars := []string{
		"Test with unicode: 🚀🔍✓",
		"Test with newlines: line1\nline2",
		"Test with tabs: \t\ttabbed",
		"Test with quotes: \"quoted\" and 'single'",
		"Test with backslashes: \\path\\to\\file",
	}

	for _, input := range specialChars {
		t.Run(fmt.Sprintf("Special chars: %s", input[:10]), func(t *testing.T) {
			// Test that functions handle special characters without panicking
			output := captureOutput(func() {
				PrintStep("🔍", "%s", input)
			})

			if !strings.Contains(output, "🔍") {
				t.Error("Output should contain the step emoji")
			}
		})
	}
}

// TestFormattingEdgeCases tests formatting edge cases
func TestFormattingEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
	}{
		{
			name:   "Too many arguments",
			format: "One arg: %s",
			args:   []any{"arg1", "arg2", "arg3"},
		},
		{
			name:   "Too few arguments",
			format: "Three args: %s %s %s",
			args:   []any{"arg1"},
		},
		{
			name:   "Wrong argument type",
			format: "Number: %d",
			args:   []any{"not a number"},
		},
		{
			name:   "Complex formatting",
			format: "Complex: %s %d %.2f %t",
			args:   []any{"string", 42, 3.14159, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure formatting doesn't panic even with mismatched args
			output := captureOutput(func() {
				PrintInfo(tt.format, tt.args...)
			})

			if !strings.Contains(output, "•") {
				t.Error("Output should contain the info symbol")
			}
		})
	}
}

// BenchmarkPrintBanner benchmarks the PrintBanner function
func BenchmarkPrintBanner(b *testing.B) {
	// Redirect stdout to discard during benchmark
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintBanner("DEVELOPMENT")
	}
}

// BenchmarkPrintStep benchmarks the PrintStep function
func BenchmarkPrintStep(b *testing.B) {
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStep("🔍", "Processing item %d", i)
	}
}

// BenchmarkPrintBuildSummary benchmarks the PrintBuildSummary function
func BenchmarkPrintBuildSummary(b *testing.B) {
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	duration := 500 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintBuildSummary(duration, false)
	}
}

// TestConcurrentPrinting tests concurrent access to print functions
func TestConcurrentPrinting(t *testing.T) {
	const numGoroutines = 10

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines printing concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			PrintStep("🔍", "Goroutine %d step", id)
			PrintSuccess("Goroutine %d success", id)
			PrintInfo("Goroutine %d info", id)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	t.Log("All concurrent print operations completed without panic")
}

// TestLongDuration tests duration formatting with very long durations
func TestLongDuration(t *testing.T) {
	longDuration := 1*time.Hour + 23*time.Minute + 45*time.Second + 123*time.Millisecond

	output := captureOutput(func() {
		PrintBuildSummary(longDuration, true)
	})

	// Should contain some representation of the duration
	if !strings.Contains(output, "Duration:") {
		t.Error("Output should contain duration information")
	}

	// The exact format may vary, but it should be present
	t.Logf("Long duration formatted as part of: %s", output)
}
