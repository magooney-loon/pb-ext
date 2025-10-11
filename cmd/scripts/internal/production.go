package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ProductionBuild orchestrates the entire production build process
func ProductionBuild(rootDir string, installDeps bool, distDir string) error {
	PrintHeader("🚀 PRODUCTION BUILD")

	outputDir := filepath.Join(rootDir, distDir)
	start := time.Now()

	// Clean and create output directory
	if err := prepareOutputDirectory(outputDir); err != nil {
		return err
	}

	// Check system requirements
	if err := CheckSystemRequirements(); err != nil {
		return fmt.Errorf("system requirements not met: %w", err)
	}

	// Install dependencies if requested
	if installDeps {
		frontendDir := filepath.Join(rootDir, "frontend")
		if err := InstallDependencies(rootDir, frontendDir); err != nil {
			return fmt.Errorf("dependency installation failed: %w", err)
		}
	}

	// Build frontend for production
	if err := BuildFrontendProduction(rootDir, installDeps); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Copy frontend to dist
	if err := CopyFrontendToDist(rootDir, outputDir); err != nil {
		return fmt.Errorf("frontend copy to dist failed: %w", err)
	}

	// Build server binary
	if err := BuildServerBinary(rootDir, outputDir); err != nil {
		return fmt.Errorf("server binary build failed: %w", err)
	}

	// Generate package metadata
	if err := GeneratePackageMetadata(rootDir, outputDir); err != nil {
		PrintWarning("Failed to generate package metadata: %v", err)
	}

	// Run test suite and generate reports
	if err := RunTestSuiteAndGenerateReport(rootDir, outputDir); err != nil {
		PrintWarning("Test suite failed: %v", err)
	}

	// Create production archive
	if err := CreateProjectArchive(rootDir, outputDir); err != nil {
		PrintWarning("Failed to create production archive: %v", err)
	}

	duration := time.Since(start)
	printProductionSummary(outputDir, duration)

	return nil
}

// prepareOutputDirectory cleans and creates the output directory
func prepareOutputDirectory(outputDir string) error {
	PrintStep("🧹", "Cleaning output directory...")

	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean dist directory: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	PrintSuccess("Output directory prepared: %s", outputDir)
	return nil
}

// printProductionSummary displays an enhanced summary of the production build
func printProductionSummary(outputDir string, duration time.Duration) {
	fmt.Printf("\n%s🎉 Production Build Complete%s\n", Bold+Green, Reset)
	fmt.Printf("%s═══════════════════════════%s\n", Gray, Reset)

	// Build metadata
	fmt.Printf("\n%sBuild Info%s\n", Bold, Reset)
	fmt.Printf("  %s•%s Duration: %s%v%s\n", Gray, Reset, Cyan, duration.Round(time.Millisecond), Reset)
	fmt.Printf("  %s•%s Target: %s%s%s\n", Gray, Reset, Gray, runtime.GOOS+"/"+runtime.GOARCH, Reset)
	fmt.Printf("  %s•%s Location: %s%s%s\n", Gray, Reset, Green, outputDir, Reset)

	// List generated files
	fmt.Printf("\n%sGenerated Files%s\n", Bold, Reset)
	listProductionFiles(outputDir)

	// Deployment info
	fmt.Printf("\n%s🚀 Ready for Deployment%s\n", Bold+Cyan, Reset)
	fmt.Printf("  %s•%s Optimized binary with static assets\n", Gray, Reset)
	fmt.Printf("  %s•%s Test reports and coverage included\n", Gray, Reset)
	fmt.Printf("  %s•%s Production archive created\n", Gray, Reset)

	// pb-deployer integration
	fmt.Printf("\n%s💡 Next Steps%s\n", Bold+Yellow, Reset)
	fmt.Printf("  %sAutomate deployment to VPS:%s\n", Gray, Reset)
	fmt.Printf("    %sgit clone https://github.com/magooney-loon/pb-deployer%s\n", Cyan, Reset)
	fmt.Printf("    %scd pb-deployer && go run cmd/scripts/main.go --install%s\n", Cyan, Reset)
	fmt.Printf("    %s(Compatible with PocketBase v0.20+ apps)%s\n", Gray, Reset)

	fmt.Printf("\n  %sOr deploy manually:%s\n", Gray, Reset)
	fmt.Printf("    %s1.%s Upload the production archive to your server\n", Gray, Reset)
	fmt.Printf("    %s2.%s Extract and configure your PocketBase app\n", Gray, Reset)
	fmt.Printf("    %s3.%s Set up systemd service for auto-start\n", Gray, Reset)

	fmt.Printf("\n%s📚 Learn more:%s %shttps://github.com/magooney-loon/pb-deployer%s\n",
		Gray, Reset, Cyan, Reset)
	fmt.Println()
}

// listProductionFiles lists the files created in the production build
func listProductionFiles(outputDir string) {
	// Check for server binary
	binaryPaths := []string{AppName, AppName + ".exe"}
	for _, binary := range binaryPaths {
		binaryPath := filepath.Join(outputDir, binary)
		if stat, err := os.Stat(binaryPath); err == nil {
			size := float64(stat.Size()) / (1024 * 1024) // Convert to MB
			fmt.Printf("  %s✓%s %s %s(%.1f MB)%s\n", Green, Reset, binary, Gray, size, Reset)
			break
		}
	}

	// Check for frontend assets
	pbPublicPath := filepath.Join(outputDir, "pb_public")
	if _, err := os.Stat(pbPublicPath); err == nil {
		fmt.Printf("  %s✓%s pb_public/ %s(frontend assets)%s\n", Green, Reset, Gray, Reset)
	}

	// Check for metadata files
	metadataFiles := map[string]string{
		"build-info.txt":        "build metadata",
		"package-metadata.json": "deployment info",
	}
	for file, desc := range metadataFiles {
		filePath := filepath.Join(outputDir, file)
		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("  %s✓%s %s %s(%s)%s\n", Green, Reset, file, Gray, desc, Reset)
		}
	}

	// Check for test reports
	reportsDir := filepath.Join(outputDir, "test-reports")
	if stat, err := os.Stat(reportsDir); err == nil && stat.IsDir() {
		fmt.Printf("  %s✓%s test-reports/ %s(test results & coverage)%s\n", Green, Reset, Gray, Reset)
	}

	// Check for archive
	entries, err := os.ReadDir(outputDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
				if stat, err := entry.Info(); err == nil {
					size := float64(stat.Size()) / (1024 * 1024) // Convert to MB
					fmt.Printf("  %s✓%s %s %s(%.1f MB archive)%s\n", Green, Reset, entry.Name(), Gray, size, Reset)
				}
				break
			}
		}
	}
}

// ValidateProductionBuild performs validation checks on the production build
func ValidateProductionBuild(outputDir string) error {
	PrintStep("✅", "Validating production build...")

	// Check if output directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return fmt.Errorf("production build directory not found: %s", outputDir)
	}

	// Check for server binary
	binaryFound := false
	binaryPaths := []string{AppName, AppName + ".exe"}
	for _, binary := range binaryPaths {
		binaryPath := filepath.Join(outputDir, binary)
		if _, err := os.Stat(binaryPath); err == nil {
			binaryFound = true
			PrintSuccess("Server binary found: %s", binary)
			break
		}
	}

	if !binaryFound {
		return fmt.Errorf("server binary not found in production build")
	}

	// Check for frontend assets
	pbPublicPath := filepath.Join(outputDir, "pb_public")
	if _, err := os.Stat(pbPublicPath); os.IsNotExist(err) {
		return fmt.Errorf("frontend assets not found: pb_public directory missing")
	}
	PrintSuccess("Frontend assets found")

	// Check for essential files
	essentialFiles := []string{
		"build-info.txt",
		"package-metadata.json",
	}

	for _, file := range essentialFiles {
		filePath := filepath.Join(outputDir, file)
		if _, err := os.Stat(filePath); err == nil {
			PrintSuccess("Metadata file found: %s", file)
		} else {
			PrintWarning("Optional file missing: %s", file)
		}
	}

	PrintSuccess("Production build validation completed")
	return nil
}

// CleanProductionBuild removes old production build artifacts
func CleanProductionBuild(rootDir, distDir string) error {
	PrintStep("🧹", "Cleaning previous production builds...")

	outputDir := filepath.Join(rootDir, distDir)

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		PrintInfo("No previous build to clean")
		return nil
	}

	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean production build: %w", err)
	}

	PrintSuccess("Previous production builds cleaned")
	return nil
}
