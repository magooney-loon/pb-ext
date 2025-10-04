package internal

import (
	"fmt"
	"os"
	"path/filepath"
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
	PrintBuildSummary(duration, true)
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

// printProductionSummary displays a detailed summary of the production build
func printProductionSummary(outputDir string, duration time.Duration) {
	fmt.Printf("\n%sProduction Build Summary%s\n", Bold, Reset)
	fmt.Printf("%s%s%s\n", Gray, strings.Repeat("─", 24), Reset)

	// List generated files
	fmt.Printf("\n%sGenerated Files:%s\n", Gray, Reset)

	// Check for server binary
	binaryPaths := []string{"pb-deployer", "pb-deployer.exe"}
	for _, binary := range binaryPaths {
		binaryPath := filepath.Join(outputDir, binary)
		if _, err := os.Stat(binaryPath); err == nil {
			fmt.Printf("  %s✓%s %s\n", Green, Reset, binary)
			break
		}
	}

	// Check for frontend assets
	pbPublicPath := filepath.Join(outputDir, "pb_public")
	if _, err := os.Stat(pbPublicPath); err == nil {
		fmt.Printf("  %s✓%s pb_public/ (frontend assets)\n", Green, Reset)
	}

	// Check for metadata files
	metadataFiles := []string{
		"build-info.txt",
		"package-metadata.json",
	}
	for _, file := range metadataFiles {
		filePath := filepath.Join(outputDir, file)
		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("  %s✓%s %s\n", Green, Reset, file)
		}
	}

	// Check for test reports
	reportsDir := filepath.Join(outputDir, "test-reports")
	if _, err := os.Stat(reportsDir); err == nil {
		fmt.Printf("  %s✓%s test-reports/ (test results)\n", Green, Reset)
	}

	// Check for archive
	entries, err := os.ReadDir(outputDir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".zip") {
				fmt.Printf("  %s✓%s %s\n", Green, Reset, entry.Name())
				break
			}
		}
	}

	fmt.Printf("\n%sDeployment Ready:%s %s%s%s\n",
		Gray, Reset, Green, outputDir, Reset)
	fmt.Printf("%sTotal Time:%s %s%v%s\n",
		Gray, Reset, Cyan, duration.Round(time.Millisecond), Reset)

	fmt.Println()
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
	binaryPaths := []string{"pb-deployer", "pb-deployer.exe"}
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
