package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// TestResult represents the result of running tests for a package
type TestResult struct {
	Package     string
	Passed      int
	Failed      int
	Skipped     int
	Duration    time.Duration
	Success     bool
	Output      []string
	FailedTests []string
}

// TestSuite represents the complete test suite results
type TestSuite struct {
	Results     []TestResult
	TotalPassed int
	TotalFailed int
	TotalTests  int
	Duration    time.Duration
	Success     bool
}

// getTestPackages discovers test packages by walking the project directory
func getTestPackages() []string {
	var testPackages []string

	fmt.Printf("  🔍 Scanning for tests\n")

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files (but not the root directory ".")
		if strings.HasPrefix(filepath.Base(path), ".") && path != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip vendor, node_modules, and other non-source directories
		if info.IsDir() {
			name := filepath.Base(path)
			if name == "vendor" || name == "node_modules" || name == "dist" ||
				name == "pb_data" || name == "pb_public" || name == "frontend" {
				return filepath.SkipDir
			}

		}

		// Check if this is a Go test file
		if strings.HasSuffix(path, "_test.go") && !info.IsDir() {
			dir := filepath.Dir(path)

			// Convert to relative path with ./ prefix for go test
			if dir == "." {
				dir = "."
			} else if !strings.HasPrefix(dir, "./") {
				dir = "./" + dir
			}

			// Add to list if not already present
			found := false
			for _, pkg := range testPackages {
				if pkg == dir {
					found = true
					break
				}
			}
			if !found {
				testPackages = append(testPackages, dir)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("   %sError walking directory: %v%s\n", Red, err, Reset)
		return []string{}
	}

	// Sort packages for consistent output
	sort.Strings(testPackages)

	fmt.Printf("  %s•%s Found %d package(s)\n", Gray, Reset, len(testPackages))
	fmt.Println()

	return testPackages
}

// runTestSuite executes all test packages with formatted output
func runTestSuite(packages []string) TestSuite {
	suite := TestSuite{
		Results: make([]TestResult, 0, len(packages)),
		Success: true,
	}

	start := time.Now()

	fmt.Printf("\n📦 %sRunning Tests%s\n", Bold, Reset)
	fmt.Println()

	for i, pkg := range packages {
		result := runTestPackage(pkg, i+1, len(packages))
		suite.Results = append(suite.Results, result)

		suite.TotalPassed += result.Passed
		suite.TotalFailed += result.Failed
		suite.TotalTests += result.Passed + result.Failed + result.Skipped

		if !result.Success {
			suite.Success = false
		}
	}

	suite.Duration = time.Since(start)
	return suite
}

// runTestPackage executes tests for a specific package
func runTestPackage(packagePath string, current, total int) TestResult {
	result := TestResult{
		Package:     packagePath,
		Output:      []string{},
		FailedTests: []string{},
	}

	fmt.Printf("├─ %s[%d/%d]%s %s%s%s\n",
		Dim, current, total, Reset,
		Bold, packagePath, Reset)

	start := time.Now()

	cmd := exec.Command("go", "test", "-v", packagePath)
	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	if err != nil {
		result.Success = false
	} else {
		result.Success = true
	}

	parseTestOutput(string(output), &result)

	if result.Success {
		fmt.Printf("│  %s✓%s %sPassed%s %s(%dms)%s\n",
			Green, Reset, Green, Reset,
			Gray, result.Duration.Milliseconds(), Reset)

		if result.Passed > 0 {
			fmt.Printf("│  %s%d test(s) passed%s\n",
				Gray, result.Passed, Reset)
		}
	} else {
		fmt.Printf("│  %s✗%s %sFailed%s %s(%dms)%s\n",
			Red, Reset, Red, Reset,
			Gray, result.Duration.Milliseconds(), Reset)

		if result.Failed > 0 {
			fmt.Printf("│  %s%d test(s) failed, %d passed%s\n",
				Red, result.Failed, result.Passed, Reset)
		}
	}

	if len(result.FailedTests) > 0 {
		for _, failedTest := range result.FailedTests {
			fmt.Printf("│  %s└─ %s%s\n", Red, failedTest, Reset)
		}
	}

	// Show skipped tests if any
	if result.Skipped > 0 {
		fmt.Printf("│  %s%d test(s) skipped%s\n",
			Yellow, result.Skipped, Reset)
	}

	fmt.Println("│")
	return result
}

// parseTestOutput parses test output to extract results
func parseTestOutput(output string, result *TestResult) {
	lines := strings.Split(output, "\n")

	testPassRegex := regexp.MustCompile(`^\s*--- PASS: (\w+)`)
	testFailRegex := regexp.MustCompile(`^\s*--- FAIL: (\w+)`)
	testSkipRegex := regexp.MustCompile(`^\s*--- SKIP: (\w+)`)

	for _, line := range lines {
		result.Output = append(result.Output, line)

		if matches := testPassRegex.FindStringSubmatch(line); len(matches) > 1 {
			result.Passed++
		} else if matches := testFailRegex.FindStringSubmatch(line); len(matches) > 1 {
			result.Failed++
			result.FailedTests = append(result.FailedTests, matches[1])
		} else if matches := testSkipRegex.FindStringSubmatch(line); len(matches) > 1 {
			result.Skipped++
		} else if strings.Contains(line, "FAIL") && strings.Contains(line, "exit status") {
			result.Success = false
		}
	}

	if result.Failed > 0 {
		result.Success = false
	}
}

// printTestSummary prints the final test summary
func printTestSummary(suite TestSuite) {
	fmt.Println()

	// Header with visual separation
	if suite.Success {
		fmt.Printf("  %s🧪 Test Suite Completed Successfully%s\n", Bold+Green, Reset)
	} else {
		fmt.Printf("  %s🚨 Test Suite Failed%s\n", Bold+Red, Reset)
	}
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Println()

	// Test metrics in a clean table format
	fmt.Printf("  %s📊 Test Results%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)

	// Calculate success rate
	successRate := float64(suite.TotalPassed) / float64(suite.TotalTests) * 100
	if suite.TotalTests == 0 {
		successRate = 0
	}

	fmt.Printf("    %sTotal Tests     %s%d%s\n", Gray, Reset, suite.TotalTests, Reset)
	fmt.Printf("    %sPassed          %s%s%d%s\n", Gray, Reset, Green, suite.TotalPassed, Reset)

	if suite.TotalFailed > 0 {
		fmt.Printf("    %sFailed          %s%s%d%s\n", Gray, Reset, Red, suite.TotalFailed, Reset)
	}

	fmt.Printf("    %sSuccess Rate    %s%.1f%%%s\n", Gray, Reset, successRate, Reset)
	fmt.Printf("    %sDuration        %s%dms%s\n", Gray, Reset, suite.Duration.Milliseconds(), Reset)
	fmt.Printf("    %sPackages        %s%d%s\n", Gray, Reset, len(suite.Results), Reset)

	if !suite.Success {
		fmt.Println()
		fmt.Printf("  %s🚨 Failed Test Packages%s\n", Bold+Red, Reset)
		fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
		for _, result := range suite.Results {
			if !result.Success {
				fmt.Printf("    %s▸ %s%-30s%s %s%d failures%s\n",
					Red, Bold, result.Package, Reset, Red, result.Failed, Reset)
			}
		}
	}

	// Success footer
	if suite.Success {
		fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", Green, Reset)
		fmt.Printf("%s✅ All tests passed! Your code is ready for deployment.%s\n", Bold+Green, Reset)
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", Green, Reset)
	} else {
		fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", Red, Reset)
		fmt.Printf("%s❌ Test failures detected. Please review and fix before deployment.%s\n", Bold+Red, Reset)
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", Red, Reset)
	}
}

// RunTestSuiteAndGenerateReport runs the full test suite and generates reports
func RunTestSuiteAndGenerateReport(rootDir, outputDir string) error {
	PrintStep("🧪", "Initializing comprehensive test suite...")

	reportsDir := filepath.Join(outputDir, "test-reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("failed to create test reports directory: %w", err)
	}

	// Get test packages using auto-discovery
	packages := getTestPackages()
	if len(packages) == 0 {
		PrintWarning("No test packages discovered in project")
		return nil
	}

	PrintStep("🔍", "Discovered %d test packages for execution", len(packages))

	// Run test suite with formatted output
	suite := runTestSuite(packages)

	// Print formatted summary
	printTestSummary(suite)

	// Generate reports
	testStatus := "PASSED"
	var testErr error
	if !suite.Success {
		testStatus = "FAILED"
		testErr = fmt.Errorf("test suite failed")
		PrintInfo("Generating reports")
	}

	// Combine all output for reporting
	var allOutput strings.Builder
	var allErrors strings.Builder
	for _, result := range suite.Results {
		for _, line := range result.Output {
			allOutput.WriteString(line + "\n")
		}
	}

	// Generate all test reports
	PrintStep("📊", "Generating reports")

	if err := GenerateTestSummary(rootDir, reportsDir, suite.Duration, testStatus, testErr, allOutput.String(), allErrors.String()); err != nil {
		PrintWarning("Failed to generate test summary: %v", err)
	}

	if err := GenerateTestReport(rootDir, reportsDir, testStatus, testErr, allOutput.String(), allErrors.String(), suite.Duration); err != nil {
		PrintWarning("Failed to generate detailed test report: %v", err)
	}

	if err := GenerateHTMLCoverageReport(rootDir, reportsDir, packages); err != nil {
		PrintWarning("Failed to generate HTML coverage report: %v", err)
	}

	PrintSuccess("Reports: test-summary.txt, test-report.json, coverage.html")

	if !suite.Success {
		PrintError("Test suite failed in %v", suite.Duration.Round(time.Millisecond))
		PrintInfo("Failed: %d, Passed: %d, Total: %d", suite.TotalFailed, suite.TotalPassed, suite.TotalTests)
		PrintInfo("Reports generated in: %s", reportsDir)
		return fmt.Errorf("test suite failed: %w", testErr)
	} else {
		PrintSuccess("Test suite completed successfully in %v", suite.Duration.Round(time.Millisecond))
		return nil
	}
}

// TestOnlyMode runs only the test suite without other operations
func TestOnlyMode(rootDir, distDir string) error {
	fmt.Println()
	fmt.Printf("🧪 %sRunning Test Suite%s\n", Bold+Cyan, Reset)
	fmt.Printf("   %s%s%s\n", Gray, time.Now().Format("15:04:05"), Reset)
	fmt.Println()

	outputDir := filepath.Join(rootDir, distDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check prerequisites
	fmt.Printf("🔍 %sChecking prerequisites...%s\n", Gray, Reset)
	if err := checkGoTestAvailable(); err != nil {
		PrintError("Prerequisites check failed: %v", err)
		return err
	}
	fmt.Printf("✓  %sGo toolchain available%s\n", Green, Reset)
	fmt.Println()

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

// checkGoTestAvailable checks if Go toolchain is available
func checkGoTestAvailable() error {
	cmd := exec.Command("go", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go command not available: %v", err)
	}

	// Extract go version for display
	versionStr := strings.TrimSpace(string(output))
	if parts := strings.Fields(versionStr); len(parts) >= 3 {
		fmt.Printf("   %s%s%s\n", Gray, versionStr, Reset)
	}

	return nil
}

// GenerateTestSummary creates a human-readable test summary file
func GenerateTestSummary(rootDir, reportsDir string, duration time.Duration, status string, testErr error, testOutput, testErrors string) error {

	summaryPath := filepath.Join(reportsDir, "test-summary.txt")
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		return fmt.Errorf("failed to create test summary file: %w", err)
	}
	defer summaryFile.Close()

	// Write test summary
	fmt.Fprintf(summaryFile, "%s Test Suite Summary\n", AppName)
	fmt.Fprintf(summaryFile, "==============================\n\n")
	fmt.Fprintf(summaryFile, "Execution Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(summaryFile, "Duration: %v\n", duration.Round(time.Millisecond))
	fmt.Fprintf(summaryFile, "Root Directory: %s\n\n", rootDir)

	// Add Go version and system info
	goVersion := GetCommandOutput("go", "version")
	fmt.Fprintf(summaryFile, "Go Version: %s\n", goVersion)
	fmt.Fprintf(summaryFile, "Test Command: go run ./cmd/scripts/main.go --test-only\n\n")

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

	return nil
}

// GenerateTestReport creates a detailed JSON test report
func GenerateTestReport(rootDir, reportsDir string, status string, testErr error, testOutput, testErrors string, duration time.Duration) error {

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
    "command": "go run ./cmd/scripts/main.go --test-only",
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
func GenerateHTMLCoverageReport(rootDir, reportsDir string, packages []string) error {

	// Check if coverage.out exists
	coverageFile := filepath.Join(rootDir, "coverage.out")
	if _, err := os.Stat(coverageFile); os.IsNotExist(err) {
		if len(packages) == 0 {
			PrintInfo("No test packages found, skipping coverage")
			return nil
		}

		// Try to generate coverage for the discovered packages only
		args := append([]string{"test", "-coverprofile=coverage.out"}, packages...)
		cmd := exec.Command("go", args...)
		cmd.Dir = rootDir
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			// Clean up any partially created coverage file
			os.Remove(coverageFile)
			PrintWarning("Coverage generation failed, skipping HTML report")
			return nil
		}
	}

	// Generate HTML coverage report
	htmlReportPath := filepath.Join(reportsDir, "coverage.html")
	cmd := exec.Command("go", "tool", "cover", "-html=coverage.out", "-o", htmlReportPath)
	cmd.Dir = rootDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Clean up coverage file if HTML generation fails
		os.Remove(coverageFile)
		PrintWarning("HTML coverage report generation failed")
		return nil
	}

	// Generate coverage summary
	summaryCmd := exec.Command("go", "tool", "cover", "-func=coverage.out")
	summaryCmd.Dir = rootDir
	summaryOutput, err := summaryCmd.Output()
	if err == nil {
		summaryPath := filepath.Join(reportsDir, "coverage-summary.txt")
		if summaryFile, err := os.Create(summaryPath); err == nil {
			summaryFile.Write(summaryOutput)
			summaryFile.Close()
		}
	}

	// Move the coverage.out file to reports directory for reference
	reportsCoverageFile := filepath.Join(reportsDir, "coverage.out")
	if err := os.Rename(coverageFile, reportsCoverageFile); err != nil {
		// If rename fails, try copying instead
		if err := copyFile(coverageFile, reportsCoverageFile); err == nil {
			os.Remove(coverageFile) // Clean up original
		}
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
