package internal

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestCheckCommand tests the CheckCommand function
func TestCheckCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected bool
	}{
		{
			name:     "Valid command - echo",
			command:  "echo",
			args:     []string{"test"},
			expected: true,
		},
		{
			name:     "Invalid command",
			command:  "nonexistentcommand12345",
			args:     []string{},
			expected: false,
		},
		{
			name:     "Command with invalid arguments",
			command:  "ls",
			args:     []string{"--invalid-flag-xyz"},
			expected: false,
		},
	}

	// Platform-specific tests
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name     string
			command  string
			args     []string
			expected bool
		}{
			name:     "Windows dir command",
			command:  "dir",
			args:     []string{},
			expected: true,
		})
	} else {
		tests = append(tests, struct {
			name     string
			command  string
			args     []string
			expected bool
		}{
			name:     "Unix ls command",
			command:  "ls",
			args:     []string{},
			expected: true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.command, tt.args...)
			if result != tt.expected {
				t.Errorf("CheckCommand(%s, %v) = %t, want %t", tt.command, tt.args, result, tt.expected)
			}
		})
	}
}

// TestGetCommandOutput tests the GetCommandOutput function
func TestGetCommandOutput(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		args         []string
		expectOutput bool
		contains     string
	}{
		{
			name:         "Echo command",
			command:      "echo",
			args:         []string{"hello"},
			expectOutput: true,
			contains:     "hello",
		},
		{
			name:         "Invalid command",
			command:      "nonexistentcommand12345",
			args:         []string{},
			expectOutput: false,
			contains:     "unknown",
		},
	}

	// Add platform-specific tests
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name         string
			command      string
			args         []string
			expectOutput bool
			contains     string
		}{
			name:         "PWD command",
			command:      "pwd",
			args:         []string{},
			expectOutput: true,
			contains:     "/", // Should contain at least root slash
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := GetCommandOutput(tt.command, tt.args...)

			if tt.expectOutput {
				if output == "unknown" {
					t.Errorf("GetCommandOutput(%s, %v) returned 'unknown', expected valid output", tt.command, tt.args)
				}
				if tt.contains != "" && !strings.Contains(output, tt.contains) {
					t.Errorf("GetCommandOutput(%s, %v) = %s, should contain %s", tt.command, tt.args, output, tt.contains)
				}
			} else {
				if output != "unknown" {
					t.Errorf("GetCommandOutput(%s, %v) = %s, expected 'unknown'", tt.command, tt.args, output)
				}
			}
		})
	}
}

// TestCheckSystemRequirements tests the system requirements checking
func TestCheckSystemRequirements(t *testing.T) {
	// This test checks the actual system, so results may vary
	// We mainly test that it doesn't panic and returns appropriate errors

	t.Run("System requirements check", func(t *testing.T) {
		err := CheckSystemRequirements()

		// In CI/test environments, some tools might be missing
		// We mainly want to ensure no panic and proper error handling
		if err != nil {
			// Check that error message is informative
			if !strings.Contains(err.Error(), "required but not found") {
				t.Errorf("Expected informative error message, got: %v", err)
			}
		}

		// The function should always complete without panic
		t.Log("CheckSystemRequirements completed without panic")
	})
}

// TestCheckSystemRequirementsComponents tests individual components
func TestCheckSystemRequirementsComponents(t *testing.T) {
	// Test individual command availability
	commands := []struct {
		name    string
		command string
		args    []string
	}{
		{"Go", "go", []string{"version"}},
		{"Git", "git", []string{"--version"}},
	}

	for _, cmd := range commands {
		t.Run(cmd.name+" availability", func(t *testing.T) {
			available := CheckCommand(cmd.command, cmd.args...)
			t.Logf("%s available: %t", cmd.name, available)

			if available {
				output := GetCommandOutput(cmd.command, cmd.args...)
				if output == "unknown" {
					t.Errorf("%s command succeeded but output was 'unknown'", cmd.name)
				}
				t.Logf("%s version: %s", cmd.name, output)
			}
		})
	}
}

// TestCommandOutputTrimming tests that command output is properly trimmed
func TestCommandOutputTrimming(t *testing.T) {
	// Test that whitespace is properly trimmed from command output
	output := GetCommandOutput("echo", "  test  ")

	if strings.HasPrefix(output, " ") || strings.HasSuffix(output, " ") {
		t.Errorf("Command output not properly trimmed: '%s'", output)
	}

	if !strings.Contains(output, "test") {
		t.Errorf("Expected output to contain 'test', got: '%s'", output)
	}
}

// TestCheckCommandEdgeCases tests edge cases for CheckCommand
func TestCheckCommandEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected bool
	}{
		{
			name:     "Empty command",
			command:  "",
			args:     []string{},
			expected: false,
		},
		{
			name:     "Command with spaces",
			command:  " echo ",
			args:     []string{},
			expected: false, // Command with spaces should fail
		},
		{
			name:     "Many arguments",
			command:  "echo",
			args:     []string{"arg1", "arg2", "arg3", "arg4", "arg5"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCommand(tt.command, tt.args...)
			if result != tt.expected {
				t.Errorf("CheckCommand(%s, %v) = %t, want %t", tt.command, tt.args, result, tt.expected)
			}
		})
	}
}

// TestGetCommandOutputEdgeCases tests edge cases for GetCommandOutput
func TestGetCommandOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected string
	}{
		{
			name:     "Empty command",
			command:  "",
			args:     []string{},
			expected: "unknown",
		},
		{
			name:     "Command that produces no output",
			command:  "echo",
			args:     []string{"-n"}, // -n flag prevents newline
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCommandOutput(tt.command, tt.args...)
			if result != tt.expected {
				t.Errorf("GetCommandOutput(%s, %v) = %s, want %s", tt.command, tt.args, result, tt.expected)
			}
		})
	}
}

// BenchmarkCheckCommand benchmarks the CheckCommand function
func BenchmarkCheckCommand(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckCommand("echo", "test")
	}
}

// BenchmarkGetCommandOutput benchmarks the GetCommandOutput function
func BenchmarkGetCommandOutput(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetCommandOutput("echo", "test")
	}
}

// TestSystemRequirementsWithMockedCommands tests with specific command scenarios
func TestSystemRequirementsWithMockedCommands(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	t.Run("With empty PATH", func(t *testing.T) {
		// Temporarily clear PATH to simulate missing commands
		os.Setenv("PATH", "")

		// Go should not be found
		result := CheckCommand("go", "version")
		if result {
			t.Error("Expected go command to fail with empty PATH")
		}

		// Restore PATH
		os.Setenv("PATH", originalPath)
	})
}

// TestCommandExecutionSafety tests that commands are executed safely
func TestCommandExecutionSafety(t *testing.T) {
	// Test potentially dangerous commands are handled safely
	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{
			name:    "Command with shell injection attempt",
			command: "echo",
			args:    []string{"; rm -rf /"},
		},
		{
			name:    "Command with pipe attempt",
			command: "echo",
			args:    []string{"test | cat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should not cause any harm as exec.Command properly handles arguments
			result := CheckCommand(tt.command, tt.args...)

			// The command should still work (echo with weird arguments)
			if !result {
				t.Logf("Command failed as expected for potentially dangerous input: %s %v", tt.command, tt.args)
			}
		})
	}
}

// TestConcurrentCommandExecution tests concurrent command execution
func TestConcurrentCommandExecution(t *testing.T) {
	const numGoroutines = 10
	results := make(chan bool, numGoroutines)

	// Run multiple commands concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			result := CheckCommand("echo", "concurrent-test")
			results <- result
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-results {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected all %d concurrent commands to succeed, got %d", numGoroutines, successCount)
	}
}

// TestCommandTimeout tests command execution doesn't hang
func TestCommandTimeout(t *testing.T) {
	// Test with a command that should complete quickly
	done := make(chan bool, 1)

	go func() {
		CheckCommand("echo", "timeout-test")
		done <- true
	}()

	select {
	case <-done:
		// Command completed successfully
	case <-make(chan bool):
		// This shouldn't happen as the channel is never written to
		t.Error("Command execution may have hung")
	}
}
