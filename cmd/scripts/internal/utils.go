package internal

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Application constants
const (
	AppName = "pb-cli"
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
	Dim    = "\033[2m"
)

// PrintBanner displays the application banner with operation type
func PrintBanner(operation string) {
	fmt.Println()
	fmt.Printf("%s┌─────────────────────────────────────────────────────────────┐%s\n", Cyan, Reset)
	fmt.Printf("%s│                                                             │%s\n", Cyan, Reset)
	fmt.Printf("%s│  %s▲ %s%s %sv1.0.0%s - %s%s%s                           │%s\n", Cyan, Bold, AppName, Reset, Gray, Reset, Bold, operation, Reset, Cyan)
	fmt.Printf("%s│                                                             │%s\n", Cyan, Reset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────┘%s\n", Cyan, Reset)
	fmt.Println()
}

// PrintHeader displays a section header
func PrintHeader(title string) {
	fmt.Printf("\n  %s%s%s\n", Bold+Cyan, title, Reset)
	fmt.Printf("  %s%s%s\n", Gray, strings.Repeat("─", len(title)+10), Reset)
}

// PrintStep displays a step with emoji and message
func PrintStep(emoji, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("    %s %s%s%s\n", emoji, Bold, message, Reset)
}

// PrintSuccess displays a success message
func PrintSuccess(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("    %s✓%s %s\n", Green, Reset, message)
}

// PrintError displays an error message
func PrintError(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("    %s✗%s %s\n", Red, Reset, message)
}

// PrintWarning displays a warning message
func PrintWarning(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("    %s⚠%s  %s\n", Yellow, Reset, message)
}

// PrintInfo displays an info message
func PrintInfo(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Printf("    %s▸%s %s\n", Gray, Reset, message)
}

// PrintBuildSummary displays a summary of the build process
func PrintBuildSummary(duration time.Duration, isProduction bool) {
	buildType := "Development"
	outputDir := "pb_public/"
	if isProduction {
		buildType = "Production"
		outputDir = "dist/"
	}

	fmt.Println()
	fmt.Printf("  %s🎯 Build Completed Successfully%s\n", Bold+Green, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Println()

	fmt.Printf("  %s📊 Build Metrics%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %sBuild Type      %s%s%s\n", Gray, Reset, Green, buildType)
	fmt.Printf("    %sDuration        %s%s%s\n", Gray, Reset, Cyan, duration.Round(time.Millisecond))
	fmt.Printf("    %sTarget Platform %s%s%s\n", Gray, Reset, Purple, runtime.GOOS+"/"+runtime.GOARCH)
	fmt.Printf("    %sOutput Location %s%s%s%s\n", Gray, Reset, Bold, outputDir, Reset)

	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", Green, Reset)
	fmt.Printf("%s✨ Build process completed successfully!%s\n", Bold+Green, Reset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", Green, Reset)
}

// PrintTestSummary displays a summary of the test process
func PrintTestSummary(duration time.Duration) {
	fmt.Println()
	fmt.Printf("  %s🧪 Test Suite Completed%s\n", Bold+Green, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Println()

	fmt.Printf("  %s📊 Test Metrics%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %sSuite Type      %s%sTesting%s\n", Gray, Reset, Green, Reset)
	fmt.Printf("    %sDuration        %s%s%s\n", Gray, Reset, Cyan, duration.Round(time.Millisecond))
	fmt.Printf("    %sTarget Platform %s%s%s\n", Gray, Reset, Purple, runtime.GOOS+"/"+runtime.GOARCH)
	fmt.Println()

	fmt.Printf("  %s📄 Generated Reports%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %s▸ %s%-20s%s %stext summary%s\n", Green, Bold, "test-summary.txt", Reset, Gray, Reset)
	fmt.Printf("    %s▸ %s%-20s%s %sdetailed JSON data%s\n", Green, Bold, "test-report.json", Reset, Gray, Reset)
	fmt.Printf("    %s▸ %s%-20s%s %sHTML coverage report%s\n", Green, Bold, "coverage.html", Reset, Gray, Reset)

	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", Green, Reset)
	fmt.Printf("%s✅ All tests completed successfully!%s\n", Bold+Green, Reset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", Green, Reset)
}

// ShowHelp displays the help information
func ShowHelp() {
	fmt.Println()
	fmt.Printf("%s┌─────────────────────────────────────────────────────────────┐%s\n", Cyan, Reset)
	fmt.Printf("%s│                                                             │%s\n", Cyan, Reset)
	fmt.Printf("%s│  %s▲ %s%s %sv1.0.0%s - Modern Deployment Automation        │%s\n", Cyan, Bold, AppName, Reset, Gray, Reset, Cyan)
	fmt.Printf("%s│                                                             │%s\n", Cyan, Reset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────┘%s\n", Cyan, Reset)
	fmt.Println()

	fmt.Printf("  %s⚡ Usage%s\n", Bold+Yellow, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %s$%s go run ./cmd/scripts [options]\n", Green, Reset)
	fmt.Println()

	fmt.Printf("  %s🛠️  Core Options%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %s--help%s          Show this help message\n", Cyan, Reset)
	fmt.Printf("    %s--install%s       Install all project dependencies (Go + npm)\n", Cyan, Reset)
	fmt.Printf("    %s--production%s    Create optimized production build\n", Cyan, Reset)
	fmt.Printf("    %s--build-only%s    Build frontend assets only\n", Cyan, Reset)
	fmt.Printf("    %s--run-only%s      Start development server only\n", Cyan, Reset)
	fmt.Printf("    %s--test-only%s     Execute test suite with coverage reports\n", Cyan, Reset)
	fmt.Printf("    %s--dist DIR%s      Custom output directory (default: dist)\n", Cyan, Reset)
	fmt.Println()

	fmt.Printf("  %s🚀 Quick Start Examples%s\n", Bold+Green, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)

	examples := []struct {
		desc    string
		command string
	}{
		{"Development mode (default)", "go run ./cmd/scripts"},
		{"Install dependencies first", "go run ./cmd/scripts --install"},
		{"Full production build", "go run ./cmd/scripts --production --install"},
		{"Frontend build only", "go run ./cmd/scripts --build-only"},
		{"Run comprehensive tests", "go run ./cmd/scripts --test-only"},
		{"Custom output directory", "go run ./cmd/scripts --production --dist release"},
	}

	for _, ex := range examples {
		fmt.Printf("    %s# %s%s\n", Gray, ex.desc, Reset)
		fmt.Printf("    %s$%s %s\n\n", Green, Reset, ex.command)
	}

	fmt.Printf("  %s🌐 Deployment Integration%s\n", Bold+Cyan, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %sAutomated VPS deployment with pb-deployer:%s\n", Bold, Reset)
	fmt.Printf("      %s$%s git clone https://github.com/magooney-loon/pb-deployer\n", Green, Reset)
	fmt.Printf("      %s$%s cd pb-deployer && go run cmd/scripts/main.go --install\n", Green, Reset)
	fmt.Println()

	fmt.Printf("    %s✨ pb-deployer Features:%s\n", Bold, Reset)
	fmt.Printf("      %s▸%s Automated server provisioning and security hardening\n", Green, Reset)
	fmt.Printf("      %s▸%s Zero-downtime deployments with intelligent rollback\n", Green, Reset)
	fmt.Printf("      %s▸%s Production-ready systemd service management\n", Green, Reset)
	fmt.Printf("      %s▸%s Full compatibility with PocketBase v0.20+ applications\n", Green, Reset)

	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", Cyan, Reset)
	fmt.Printf("%s📚 Documentation: https://github.com/magooney-loon/pb-deployer%s\n", Bold, Reset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", Cyan, Reset)
}
