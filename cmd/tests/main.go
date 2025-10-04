package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func getTestPackages() []string {
	return []string{
		// Put test dirs here
		"./core/logging",
		"./core/monitoring",
		"./core/server",
	}
}

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
)

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

type TestSuite struct {
	Results     []TestResult
	TotalPassed int
	TotalFailed int
	TotalTests  int
	Duration    time.Duration
	Success     bool
}

func main() {
	printHeader()

	if err := checkPrerequisites(); err != nil {
		printError("Prerequisites check failed", err.Error())
		os.Exit(1)
	}

	packages := getTestPackages()
	if len(packages) == 0 {
		printWarning("No test packages found")
		os.Exit(0)
	}

	suite := runTestSuite(packages)

	printSummary(suite)

	if suite.Success {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}

func printHeader() {
	fmt.Println()
	fmt.Printf("🧪 %sRunning Test Suite%s\n", Bold+Cyan, Reset)
	fmt.Printf("   %s%s%s\n", Gray, time.Now().Format("15:04:05"), Reset)
	fmt.Println()
}

func checkPrerequisites() error {
	fmt.Printf("🔍 %sChecking prerequisites...%s\n", Gray, Reset)

	if err := checkGoTestAvailable(); err != nil {
		return err
	}

	fmt.Printf("✓  %sGo toolchain available%s\n", Green, Reset)
	fmt.Println()
	return nil
}

// runTestSuite executes all test packages
func runTestSuite(packages []string) TestSuite {
	suite := TestSuite{
		Results: make([]TestResult, 0, len(packages)),
		Success: true,
	}

	start := time.Now()

	fmt.Printf("📦 %sRunning %d test package(s)%s\n", Bold, len(packages), Reset)
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

func printSummary(suite TestSuite) {
	fmt.Println()

	if suite.Success {
		fmt.Printf("✅ %sAll tests passed!%s\n", Bold+Green, Reset)
	} else {
		fmt.Printf("❌ %sTest suite failed%s\n", Bold+Red, Reset)
	}

	fmt.Println()
	fmt.Printf("📊 %sSummary%s\n", Bold, Reset)
	fmt.Printf("   %sTotal:%s     %d\n", Gray, Reset, suite.TotalTests)
	fmt.Printf("   %sPassed:%s    %s%d%s\n", Gray, Reset, Green, suite.TotalPassed, Reset)

	if suite.TotalFailed > 0 {
		fmt.Printf("   %sFailed:%s    %s%d%s\n", Gray, Reset, Red, suite.TotalFailed, Reset)
	}

	fmt.Printf("   %sDuration:%s  %s%dms%s\n", Gray, Reset, Gray, suite.Duration.Milliseconds(), Reset)
	fmt.Printf("   %sPackages:%s  %d\n", Gray, Reset, len(suite.Results))

	if !suite.Success {
		fmt.Println()
		fmt.Printf("🚨 %sFailed Packages:%s\n", Bold+Red, Reset)
		for _, result := range suite.Results {
			if !result.Success {
				fmt.Printf("   %s• %s%s %s(%d failures)%s\n",
					Red, result.Package, Reset,
					Gray, result.Failed, Reset)
			}
		}
	}

	fmt.Println()
}

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

func printError(title, message string) {
	fmt.Printf("❌ %s%s%s\n", Bold+Red, title, Reset)
	fmt.Printf("   %s%s%s\n", Red, message, Reset)
	fmt.Println()
}

func printWarning(message string) {
	fmt.Printf("⚠️  %s%s%s\n", Yellow, message, Reset)
	fmt.Println()
}
