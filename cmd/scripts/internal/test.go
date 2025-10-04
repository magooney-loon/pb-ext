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

	// Generate Go's built-in HTML coverage report
	if err := GenerateHTMLCoverageReport(rootDir, reportsDir); err != nil {
		PrintWarning("Failed to generate HTML coverage report: %v", err)
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

// ValidateTestEnvironment checks if the test environment is properly set up
// executeTestsWithFallback tries multiple strategies to execute tests
func executeTestsWithFallback(rootDir string, start time.Time) (string, string, error, time.Duration) {
	strategies := []struct {
		name string
		cmd  func() *exec.Cmd
	}{
		{
			name: "go test ./... with coverage",
			cmd:  func() *exec.Cmd { return exec.Command("go", "test", "-coverprofile=coverage.out", "./...") },
		},
		{
			name: "go test ./... with coverage (verbose)",
			cmd:  func() *exec.Cmd { return exec.Command("go", "test", "-v", "-coverprofile=coverage.out", "./...") },
		},
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

// GenerateHTMLCoverageReport generates Go's built-in HTML coverage report
func GenerateHTMLCoverageReport(rootDir, reportsDir string) error {
	PrintStep("🌐", "Generating HTML coverage report...")

	// Check if coverage.out exists
	coverageFile := filepath.Join(rootDir, "coverage.out")
	if _, err := os.Stat(coverageFile); os.IsNotExist(err) {
		PrintWarning("No coverage.out file found, attempting to generate coverage...")

		// Try to generate coverage if it doesn't exist
		cmd := exec.Command("go", "test", "-coverprofile=coverage.out", "./...")
		cmd.Dir = rootDir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			PrintWarning("Failed to generate coverage data: %v", err)
			return nil
		}
		PrintInfo("Generated coverage data")
	}

	// Generate HTML coverage report
	htmlReportPath := filepath.Join(reportsDir, "coverage.html")
	cmd := exec.Command("go", "tool", "cover", "-html=coverage.out", "-o", htmlReportPath)
	cmd.Dir = rootDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate HTML coverage report: %w, stderr: %s", err, stderr.String())
	}

	PrintSuccess("HTML coverage report saved to: %s", htmlReportPath)

	// Generate coverage summary
	summaryCmd := exec.Command("go", "tool", "cover", "-func=coverage.out")
	summaryCmd.Dir = rootDir
	summaryOutput, err := summaryCmd.Output()
	if err == nil {
		summaryPath := filepath.Join(reportsDir, "coverage-summary.txt")
		if summaryFile, err := os.Create(summaryPath); err == nil {
			summaryFile.Write(summaryOutput)
			summaryFile.Close()
			PrintInfo("Coverage summary saved to: %s", summaryPath)
		}
	}

	// Copy the coverage.out file to reports directory for reference
	reportsCoverageFile := filepath.Join(reportsDir, "coverage.out")
	if err := os.Rename(coverageFile, reportsCoverageFile); err != nil {
		// If rename fails, try copying instead
		if err := copyFile(coverageFile, reportsCoverageFile); err != nil {
			PrintWarning("Failed to copy coverage.out to reports directory: %v", err)
		} else {
			PrintInfo("Coverage data copied to: %s", reportsCoverageFile)
			os.Remove(coverageFile) // Clean up original
		}
	} else {
		PrintInfo("Coverage data saved to: %s", reportsCoverageFile)
	}

	return nil
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
