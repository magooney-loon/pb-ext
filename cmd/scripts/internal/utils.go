package internal

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Color constants for terminal output
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
)

// PrintBanner displays the application banner with operation type
func PrintBanner(operation string) {
	fmt.Printf("\n%s▲ pb-deployer%s %sv1.0.0%s\n", Bold, Reset, Gray, Reset)
	fmt.Printf("%s%s%s\n\n", Gray, strings.ToLower(operation), Reset)
}

// PrintHeader displays a section header
func PrintHeader(title string) {
	fmt.Printf("\n%s%s%s\n", Bold, title, Reset)
}

// PrintStep displays a step with emoji and message
func PrintStep(emoji, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", emoji, message)
}

// PrintSuccess displays a success message
func PrintSuccess(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s✓%s %s\n", Green, Reset, message)
}

// PrintError displays an error message
func PrintError(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s✗ Error:%s %s\n", Red, Reset, message)
}

// PrintWarning displays a warning message
func PrintWarning(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%s⚠ Warning:%s %s\n", Yellow, Reset, message)
}

// PrintInfo displays an info message
func PrintInfo(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("%sℹ%s %s\n", Cyan, Reset, message)
}

// PrintBuildSummary displays a summary of the build process
func PrintBuildSummary(duration time.Duration, isProduction bool) {
	buildType := "Development"
	if isProduction {
		buildType = "Production"
	}

	fmt.Printf("\n%sBuild Complete%s\n", Bold, Reset)
	fmt.Printf("%s%s%s\n", Gray, strings.Repeat("─", 14), Reset)

	fmt.Printf("\n%sType:%s     %s%s%s\n", Gray, Reset, Green, buildType, Reset)
	fmt.Printf("%sDuration:%s %s%s%s\n", Gray, Reset, Cyan, duration.Round(time.Millisecond), Reset)
	fmt.Printf("%sTarget:%s   %s%s/%s%s\n", Gray, Reset, Purple, runtime.GOOS, runtime.GOARCH, Reset)

	fmt.Printf("\n%sOutput:%s\n", Gray, Reset)
	if isProduction {
		fmt.Printf("  %sdist/%s production build\n", Green, Reset)
	} else {
		fmt.Printf("  %spb_public/%s development build\n", Green, Reset)
	}
}

// PrintTestSummary displays a summary of the test process
func PrintTestSummary(duration time.Duration) {
	fmt.Printf("\n%sTest Suite Complete%s\n", Bold, Reset)
	fmt.Printf("%s%s%s\n", Gray, strings.Repeat("─", 19), Reset)

	fmt.Printf("\n%sType:%s     %sTesting%s\n", Gray, Reset, Green, Reset)
	fmt.Printf("%sDuration:%s %s%s%s\n", Gray, Reset, Cyan, duration.Round(time.Millisecond), Reset)
	fmt.Printf("%sTarget:%s   %s%s/%s%s\n", Gray, Reset, Purple, runtime.GOOS, runtime.GOARCH, Reset)

	fmt.Printf("\n%sOutput:%s\n", Gray, Reset)
	fmt.Printf("  %stest-summary.txt%s report\n", Green, Reset)
	fmt.Printf("  %stest-report.json%s detailed data\n", Green, Reset)
}

// ShowHelp displays the help information
func ShowHelp() {
	fmt.Printf("\n%s▲ pb-deployer%s %sv1.0.0%s\n", Bold, Reset, Gray, Reset)
	fmt.Printf("%sModern deployment automation tool%s\n\n", Gray, Reset)

	fmt.Printf("%sUSAGE:%s\n", Bold, Reset)
	fmt.Printf("  go run ./cmd/scripts [options]\n\n")

	fmt.Printf("%sOPTIONS:%s\n", Bold, Reset)
	fmt.Printf("  %s--help%s          Show this help message\n", Green, Reset)
	fmt.Printf("  %s--install%s       Install all project dependencies (Go + npm)\n", Green, Reset)
	fmt.Printf("  %s--production%s    Create production build with all assets\n", Green, Reset)
	fmt.Printf("  %s--build-only%s    Build frontend without running server\n", Green, Reset)
	fmt.Printf("  %s--run-only%s      Run server without building frontend\n", Green, Reset)
	fmt.Printf("  %s--test-only%s     Run test suite and generate reports\n", Green, Reset)
	fmt.Printf("  %s--dist DIR%s      Specify output directory (default: dist)\n", Green, Reset)

	fmt.Printf("\n%sEXAMPLES:%s\n", Bold, Reset)
	fmt.Printf("  %s# Development mode (default)%s\n", Gray, Reset)
	fmt.Printf("  go run ./cmd/scripts\n\n")

	fmt.Printf("  %s# Install dependencies and build%s\n", Gray, Reset)
	fmt.Printf("  go run ./cmd/scripts --install\n\n")

	fmt.Printf("  %s# Production build%s\n", Gray, Reset)
	fmt.Printf("  go run ./cmd/scripts --production --install\n\n")

	fmt.Printf("  %s# Build only (no server)%s\n", Gray, Reset)
	fmt.Printf("  go run ./cmd/scripts --build-only\n\n")

	fmt.Printf("  %s# Run tests only%s\n", Gray, Reset)
	fmt.Printf("  go run ./cmd/scripts --test-only\n\n")

	fmt.Printf("  %s# Custom dist directory%s\n", Gray, Reset)
	fmt.Printf("  go run ./cmd/scripts --production --dist release\n\n")

	fmt.Printf("%sMORE INFO:%s\n", Bold, Reset)
	fmt.Printf("  Documentation: %shttps://github.com/your-org/pb-deployer%s\n", Cyan, Reset)
	fmt.Printf("  Report issues: %shttps://github.com/your-org/pb-deployer/issues%s\n", Cyan, Reset)
	fmt.Println()
}
