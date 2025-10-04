package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunTestSuiteAndGenerateReport runs the full test suite and generates reports
func RunTestSuiteAndGenerateReport(rootDir, outputDir string) error {
	PrintStep("🧪", "Running test suite...")

	reportsDir := filepath.Join(outputDir, "test-reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("failed to create test reports directory: %w", err)
	}

	start := time.Now()

	// Try multiple test execution strategies
	testOutput, testErrors, testErr, duration := executeTestsWithFallback(rootDir, start)

	// Always generate reports regardless of test outcome
	testStatus := "PASSED"
	if testErr != nil {
		testStatus = "FAILED"
		PrintWarning("Test suite failed but generating reports anyway")
	}

	// Generate test summary with result information
	if err := GenerateTestSummary(rootDir, reportsDir, duration, testStatus, testErr, testOutput, testErrors); err != nil {
		PrintWarning("Failed to generate test summary: %v", err)
	}

	// Generate detailed test report with result information
	if err := GenerateTestReport(rootDir, reportsDir, testStatus, testErr, testOutput, testErrors, duration); err != nil {
		PrintWarning("Failed to generate detailed test report: %v", err)
	}

	// Generate enhanced test analysis
	if err := GenerateEnhancedTestReport(rootDir, reportsDir, duration, testErr, testOutput, testErrors); err != nil {
		PrintWarning("Failed to generate enhanced test analysis: %v", err)
	}

	// Print appropriate completion message with analysis
	analysis := AnalyzeTestResults(testOutput, testErrors)

	if testErr != nil {
		PrintError("Test suite failed in %v", duration.Round(time.Millisecond))
		if analysis["totalTests"].(int) > 0 {
			PrintInfo("Failed: %d, Passed: %d, Total: %d",
				analysis["failedTests"].(int),
				analysis["passedTests"].(int),
				analysis["totalTests"].(int))
		}
		PrintInfo("Reports generated in: %s", reportsDir)
		return fmt.Errorf("test suite failed: %w", testErr)
	} else {
		PrintSuccess("Test suite completed successfully in %v", duration.Round(time.Millisecond))
		if analysis["totalTests"].(int) > 0 {
			PrintInfo("Passed: %d, Total: %d",
				analysis["passedTests"].(int),
				analysis["totalTests"].(int))
		}
		return nil
	}
}

// TestOnlyMode runs only the test suite without other operations
func TestOnlyMode(rootDir, distDir string) error {
	fmt.Printf("\n🧪 %sRunning Tests%s\n", Bold+Cyan, Reset)
	fmt.Println()

	outputDir := filepath.Join(rootDir, distDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Validate test environment first
	if err := ValidateTestEnvironment(rootDir); err != nil {
		PrintWarning("Test environment validation failed: %v", err)
		// Still try to run tests in case they exist elsewhere
	}

	if err := RunTestSuiteAndGenerateReport(rootDir, outputDir); err != nil {
		return fmt.Errorf("test suite failed: %w", err)
	}

	return nil
}

// GenerateTestSummary creates a human-readable test summary file
func GenerateTestSummary(rootDir, reportsDir string, duration time.Duration, status string, testErr error, testOutput, testErrors string) error {
	PrintStep("📊", "Generating test summary...")

	summaryPath := filepath.Join(reportsDir, "test-summary.txt")
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		return fmt.Errorf("failed to create test summary file: %w", err)
	}
	defer summaryFile.Close()

	// Write test summary
	fmt.Fprintf(summaryFile, "pb-deployer Test Suite Summary\n")
	fmt.Fprintf(summaryFile, "==============================\n\n")
	fmt.Fprintf(summaryFile, "Execution Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(summaryFile, "Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Fprintf(summaryFile, "Root Directory: %s\n\n", rootDir)

	// Add Go version and system info
	goVersion := GetCommandOutput("go", "version")
	fmt.Fprintf(summaryFile, "Go Version: %s\n", goVersion)
	fmt.Fprintf(summaryFile, "Test Command: go run ./cmd/tests\n\n")

	fmt.Fprintf(summaryFile, "Status: %s\n", status)
	if testErr != nil {
		fmt.Fprintf(summaryFile, "Error: %s\n", testErr.Error())
	}
	fmt.Fprintf(summaryFile, "Reports Directory: %s\n\n", reportsDir)

	// Include test output in summary
	if testOutput != "" {
		fmt.Fprintf(summaryFile, "Test Output:\n")
		fmt.Fprintf(summaryFile, "%s%s\n", strings.Repeat("-", 40), "\n")
		fmt.Fprintf(summaryFile, "%s\n", testOutput)
		fmt.Fprintf(summaryFile, "%s\n\n", strings.Repeat("-", 40))
	}

	if testErrors != "" {
		fmt.Fprintf(summaryFile, "Error Output:\n")
		fmt.Fprintf(summaryFile, "%s%s\n", strings.Repeat("-", 40), "\n")
		fmt.Fprintf(summaryFile, "%s\n", testErrors)
		fmt.Fprintf(summaryFile, "%s\n", strings.Repeat("-", 40))
	}

	PrintSuccess("Test summary saved to: %s", summaryPath)
	return nil
}

// GenerateTestReport creates a detailed JSON test report
func GenerateTestReport(rootDir, reportsDir string, status string, testErr error, testOutput, testErrors string, duration time.Duration) error {
	PrintStep("📋", "Generating detailed test report...")

	reportPath := filepath.Join(reportsDir, "test-report.json")
	reportFile, err := os.Create(reportPath)
	if err != nil {
		return fmt.Errorf("failed to create test report file: %w", err)
	}
	defer reportFile.Close()

	// Generate JSON report with metadata
	errorMessage := ""
	if testErr != nil {
		errorMessage = strings.ReplaceAll(testErr.Error(), `"`, `\"`)
	}

	// Escape JSON strings
	escapedOutput := strings.ReplaceAll(strings.ReplaceAll(testOutput, `"`, `\"`), "\n", "\\n")
	escapedErrors := strings.ReplaceAll(strings.ReplaceAll(testErrors, `"`, `\"`), "\n", "\\n")

	report := fmt.Sprintf(`{
  "timestamp": "%s",
  "rootDirectory": "%s",
  "reportsDirectory": "%s",
  "environment": {
    "goVersion": "%s",
    "nodeVersion": "%s",
    "npmVersion": "%s"
  },
  "testSuite": {
    "command": "go run ./cmd/tests",
    "status": "%s",
    "error": "%s",
    "output": "%s",
    "errorOutput": "%s",
    "duration": "%s"
  }
}`, time.Now().Format(time.RFC3339),
		rootDir,
		reportsDir,
		GetCommandOutput("go", "version"),
		GetCommandOutput("node", "--version"),
		GetCommandOutput("npm", "--version"),
		strings.ToLower(status),
		errorMessage,
		escapedOutput,
		escapedErrors,
		duration.String())

	if _, err := reportFile.WriteString(report); err != nil {
		return fmt.Errorf("failed to write test report: %w", err)
	}

	PrintSuccess("Detailed test report saved to: %s", reportPath)
	return nil
}

// AnalyzeTestResults analyzes test output to extract meaningful information
func AnalyzeTestResults(testOutput, testErrors string) map[string]interface{} {
	results := make(map[string]interface{})

	// Initialize counters
	results["totalTests"] = 0
	results["passedTests"] = 0
	results["failedTests"] = 0
	results["skippedTests"] = 0
	results["coverage"] = "unknown"
	results["hasOutput"] = len(testOutput) > 0
	results["hasErrors"] = len(testErrors) > 0

	// Analyze output for common Go test patterns
	if testOutput != "" {
		lines := strings.Split(testOutput, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)

			// Look for test result indicators
			if strings.Contains(line, "PASS:") {
				results["passedTests"] = results["passedTests"].(int) + 1
				results["totalTests"] = results["totalTests"].(int) + 1
			} else if strings.Contains(line, "FAIL:") {
				results["failedTests"] = results["failedTests"].(int) + 1
				results["totalTests"] = results["totalTests"].(int) + 1
			} else if strings.Contains(line, "SKIP:") {
				results["skippedTests"] = results["skippedTests"].(int) + 1
				results["totalTests"] = results["totalTests"].(int) + 1
			}

			// Look for coverage information
			if strings.Contains(line, "coverage:") && strings.Contains(line, "%") {
				// Extract coverage percentage
				parts := strings.Split(line, "coverage:")
				if len(parts) > 1 {
					coveragePart := strings.TrimSpace(parts[1])
					if idx := strings.Index(coveragePart, "%"); idx > 0 {
						results["coverage"] = strings.TrimSpace(coveragePart[:idx+1])
					}
				}
			}
		}
	}

	// Determine overall status
	if results["totalTests"].(int) == 0 {
		results["status"] = "no_tests"
	} else if results["failedTests"].(int) > 0 {
		results["status"] = "failed"
	} else {
		results["status"] = "passed"
	}

	return results
}

// GenerateEnhancedTestReport creates a comprehensive test report with analysis
func GenerateEnhancedTestReport(rootDir, reportsDir string, duration time.Duration, testErr error, testOutput, testErrors string) error {
	PrintStep("📊", "Generating enhanced test analysis...")

	analysis := AnalyzeTestResults(testOutput, testErrors)

	// Create enhanced report file
	reportPath := filepath.Join(reportsDir, "test-analysis.txt")
	reportFile, err := os.Create(reportPath)
	if err != nil {
		return fmt.Errorf("failed to create enhanced test report: %w", err)
	}
	defer reportFile.Close()

	fmt.Fprintf(reportFile, "pb-deployer Test Analysis Report\n")
	fmt.Fprintf(reportFile, "================================\n\n")
	fmt.Fprintf(reportFile, "Execution Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(reportFile, "Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Fprintf(reportFile, "Overall Status: %s\n\n", strings.ToUpper(analysis["status"].(string)))

	// Test Statistics
	fmt.Fprintf(reportFile, "Test Statistics:\n")
	fmt.Fprintf(reportFile, "  Total Tests: %d\n", analysis["totalTests"].(int))
	fmt.Fprintf(reportFile, "  Passed: %d\n", analysis["passedTests"].(int))
	fmt.Fprintf(reportFile, "  Failed: %d\n", analysis["failedTests"].(int))
	fmt.Fprintf(reportFile, "  Skipped: %d\n", analysis["skippedTests"].(int))
	fmt.Fprintf(reportFile, "  Coverage: %s\n\n", analysis["coverage"].(string))

	// Error Information
	if testErr != nil {
		fmt.Fprintf(reportFile, "Exit Code: Non-zero (test failure)\n")
		fmt.Fprintf(reportFile, "Error Message: %s\n\n", testErr.Error())
	} else {
		fmt.Fprintf(reportFile, "Exit Code: 0 (success)\n\n")
	}

	// Recommendations
	fmt.Fprintf(reportFile, "Recommendations:\n")
	if analysis["failedTests"].(int) > 0 {
		fmt.Fprintf(reportFile, "  • Review failed tests and fix issues\n")
		fmt.Fprintf(reportFile, "  • Check error output for specific failure reasons\n")
	}
	if analysis["totalTests"].(int) == 0 {
		fmt.Fprintf(reportFile, "  • No tests found - consider adding test cases\n")
	}
	if analysis["coverage"].(string) == "unknown" {
		fmt.Fprintf(reportFile, "  • Consider running tests with coverage: go test -cover\n")
	}
	if analysis["totalTests"].(int) > 0 && analysis["failedTests"].(int) == 0 {
		fmt.Fprintf(reportFile, "  • All tests passing - consider adding more test coverage\n")
	}

	PrintSuccess("Enhanced test analysis saved to: %s", reportPath)
	return nil
}

// ValidateTestEnvironment checks if the test environment is properly set up
// executeTestsWithFallback tries multiple strategies to execute tests
func executeTestsWithFallback(rootDir string, start time.Time) (string, string, error, time.Duration) {
	strategies := []struct {
		name string
		cmd  func() *exec.Cmd
	}{
		{
			name: "go run ./cmd/tests",
			cmd:  func() *exec.Cmd { return exec.Command("go", "run", "./cmd/tests") },
		},
		{
			name: "go test ./...",
			cmd:  func() *exec.Cmd { return exec.Command("go", "test", "./...") },
		},
		{
			name: "go test .",
			cmd:  func() *exec.Cmd { return exec.Command("go", "test", ".") },
		},
	}

	for i, strategy := range strategies {
		PrintStep("🔄", "Trying strategy %d: %s", i+1, strategy.name)

		var stdout, stderr bytes.Buffer
		cmd := strategy.cmd()
		cmd.Dir = rootDir

		// Use MultiWriter to write to both buffer and console simultaneously
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

		testErr := cmd.Run()
		duration := time.Since(start)

		testOutput := stdout.String()
		testErrors := stderr.String()

		// If command executed (even if tests failed), use this result
		if testErr == nil || testOutput != "" || testErrors != "" {
			if testErr != nil {
				PrintInfo("Tests executed but some failed")
			} else {
				PrintInfo("Tests executed successfully")
			}
			return testOutput, testErrors, testErr, duration
		}

		PrintWarning("Strategy failed: %s", strategy.name)
	}

	// All strategies failed - return error with duration
	duration := time.Since(start)
	return "", "No test execution strategy succeeded",
		fmt.Errorf("all test execution strategies failed"), duration
}

// ValidateTestEnvironment checks if the test environment is properly set up
func ValidateTestEnvironment(rootDir string) error {
	PrintStep("🔍", "Validating test environment...")

	// Check if tests directory exists
	testsDir := filepath.Join(rootDir, "cmd", "tests")
	if _, err := os.Stat(testsDir); os.IsNotExist(err) {
		PrintWarning("Dedicated tests directory not found at %s", testsDir)

		// Check for standard Go test files
		if hasGoTestFiles(rootDir) {
			PrintInfo("Found standard Go test files")
			return nil
		}

		return fmt.Errorf("no test files found")
	}

	// Check if main test file exists
	testMainFile := filepath.Join(testsDir, "main.go")
	if _, err := os.Stat(testMainFile); os.IsNotExist(err) {
		return fmt.Errorf("test main file not found at %s", testMainFile)
	}

	PrintSuccess("Test environment validated")
	return nil
}

// hasGoTestFiles checks if the project contains standard Go test files
func hasGoTestFiles(rootDir string) bool {
	found := false
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}
		if strings.HasSuffix(info.Name(), "_test.go") {
			found = true
			return fmt.Errorf("found test file") // Stop walking
		}
		return nil
	})
	return found
}

// RunQuickTests runs a subset of tests for quick feedback
func RunQuickTests(rootDir string) error {
	PrintStep("⚡", "Running quick tests...")

	cmd := exec.Command("go", "test", "-short", "./...")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("quick tests failed: %w", err)
	}

	duration := time.Since(start)
	PrintSuccess("Quick tests completed in %v", duration.Round(time.Millisecond))
	return nil
}
