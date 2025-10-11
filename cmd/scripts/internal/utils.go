package internal

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// Application constants
const (
	AppName = "pb-cli"
	Version = "1.0.0"
)

// Color constants (minimal set)
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
	Bold   = "\033[1m"
)

// PrintVersion displays application version with triangle logo
func PrintVersion() {
	fmt.Printf("%s▲ %s%s %sv%s%s %s(%s/%s)%s\n",
		Cyan, Bold, AppName, Reset+Gray, Version, Reset, Gray, runtime.GOOS, runtime.GOARCH, Reset)
}

// PrintOperation displays the current operation
func PrintOperation(operation string) {
	fmt.Printf("%s[>]%s %s\n", Cyan, Reset, operation)
}

// PrintStep displays a processing step
func PrintStep(message string) {
	fmt.Printf("%s[·]%s %s\n", Gray, Reset, message)
}

// PrintSuccess displays success message
func PrintSuccess(message string) {
	fmt.Printf("%s[✓]%s %s\n", Green, Reset, message)
}

// PrintError displays error message to stderr
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s[✗]%s %s\n", Red, Reset, fmt.Sprintf(format, args...))
}

// PrintWarning displays warning message
func PrintWarning(format string, args ...interface{}) {
	fmt.Printf("%s[!]%s %s\n", Yellow, Reset, fmt.Sprintf(format, args...))
}

// PrintInfo displays info message
func PrintInfo(format string, args ...interface{}) {
	fmt.Printf("%s[i]%s %s\n", Gray, Reset, fmt.Sprintf(format, args...))
}

// PrintSection displays a section header with structured formatting
func PrintSection(title string) {
	fmt.Printf("\n%s[>]%s %s%s%s\n", Cyan, Reset, Bold, title, Reset)
}

// PrintSubItem displays a sub-item with indentation
func PrintSubItem(icon, message string) {
	fmt.Printf("    %s[%s]%s %s\n", Gray, icon, Reset, message)
}

// PrintTestResult displays a structured test result
func PrintTestResult(pkg string, passed, failed, skipped int, duration time.Duration, success bool) {
	status := "✓"
	color := Green
	if !success {
		status = "✗"
		color = Red
	}

	fmt.Printf("    %s[%s]%s %s %s(%dms)%s\n",
		color, status, Reset, pkg, Gray, duration.Milliseconds(), Reset)

	if passed > 0 || failed > 0 || skipped > 0 {
		var parts []string
		if passed > 0 {
			parts = append(parts, fmt.Sprintf("%s%d passed%s", Green, passed, Reset))
		}
		if failed > 0 {
			parts = append(parts, fmt.Sprintf("%s%d failed%s", Red, failed, Reset))
		}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("%s%d skipped%s", Yellow, skipped, Reset))
		}
		fmt.Printf("      %s(%s)%s\n", Gray, fmt.Sprintf("%s", strings.Join(parts, ", ")), Reset)
	}
}

// PrintBuildStep displays a build step with more detail
func PrintBuildStep(step, detail string) {
	fmt.Printf("%s[·]%s %s %s(%s)%s\n", Gray, Reset, step, Gray, detail, Reset)
}

// PrintBuildSummary displays build completion
func PrintBuildSummary(duration time.Duration, isProduction bool) {
	buildType := "dev"
	if isProduction {
		buildType = "prod"
	}

	fmt.Printf("%s[✓]%s Build complete %s(%s, %v)%s\n",
		Green, Reset, Gray, buildType, duration.Round(time.Millisecond), Reset)
}

// PrintTestSummary displays test completion
func PrintTestSummary(duration time.Duration) {
	fmt.Printf("%s[✓]%s Tests complete %s(%v)%s\n",
		Green, Reset, Gray, duration.Round(time.Millisecond), Reset)
	fmt.Printf("%s[i]%s Reports: test-summary.txt, test-report.json, coverage.html\n", Gray, Reset)
}

// ShowHelp displays usage information
func ShowHelp() {
	fmt.Printf("%s▲ %s%s%s %sv%s%s - PocketBase deployment automation\n\n",
		Cyan, Bold, AppName, Reset, Gray, Version, Reset)

	fmt.Printf("%sUSAGE:%s\n", Bold, Reset)
	fmt.Printf("  go run ./cmd/scripts [options]\n\n")

	fmt.Printf("%sOPTIONS:%s\n", Bold, Reset)
	fmt.Printf("  --help          Show this help\n")
	fmt.Printf("  --install       Install dependencies\n")
	fmt.Printf("  --production    Production build\n")
	fmt.Printf("  --build-only    Build assets only\n")
	fmt.Printf("  --run-only      Start server only\n")
	fmt.Printf("  --test-only     Run tests only\n")
	fmt.Printf("  --dist DIR      Output directory\n\n")

	fmt.Printf("%sEXAMPLES:%s\n", Bold, Reset)
	fmt.Printf("  go run ./cmd/scripts\n")
	fmt.Printf("  go run ./cmd/scripts --production\n")
	fmt.Printf("  go run ./cmd/scripts --test-only\n")
}

// PrintBanner displays app info and operation
func PrintBanner(operation string) {
	PrintVersion()
	PrintOperation(operation)
}

// PrintHeader displays section header
func PrintHeader(title string) {
	PrintOperation(title)
}
