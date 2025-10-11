package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProductionBuild orchestrates the entire production build process
func ProductionBuild(rootDir string, installDeps bool, distDir string) error {
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

	PrintSection("Build Complete")
	PrintSubItem("✓", fmt.Sprintf("Production build finished (%v)", duration.Round(time.Millisecond)))
	PrintSubItem("i", fmt.Sprintf("Output: %s", outputDir))

	// Show deployment integration info
	printDeploymentIntegration()

	return nil
}

// prepareOutputDirectory cleans and creates the output directory
func prepareOutputDirectory(outputDir string) error {
	PrintSection("Prepare Build")
	PrintBuildStep("Cleaning output directory", outputDir)

	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean dist directory: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	PrintSubItem("✓", "Output directory ready")
	return nil
}

// printDeploymentIntegration displays VPS deployment automation info
func printDeploymentIntegration() {
	fmt.Printf("\n%s[>]%s %sDEPLOYMENT INTEGRATION%s\n\n", Cyan, Reset, Bold, Reset)

	fmt.Printf("%s```%s\n", Gray, Reset)
	fmt.Printf("$ git clone https://github.com/magooney-loon/pb-deployer\n")
	fmt.Printf("$ cd pb-deployer && go run cmd/scripts/main.go --install\n")
	fmt.Printf("%s```%s\n", Gray, Reset)
}

// printProductionSummary displays a summary of the production build
func printProductionSummary(outputDir string, duration time.Duration) {
	fmt.Printf("=> Production build completed in %v\n", duration.Round(time.Millisecond))
	fmt.Printf("   Output: %s\n", outputDir)
}

// listProductionFiles lists the files created in the production build
func listProductionFiles(outputDir string) {
	// This function is now unused - info is shown during build process
}

// printArchiveInfo displays archive information
func printArchiveInfo(outputDir string) {
	// This function is now unused - info is shown when archive is created
}

// countFilesInDir counts files in a directory recursively
func countFilesInDir(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// calculateOriginalSize estimates original size before compression
func calculateOriginalSize(outputDir, archiveName string) float64 {
	totalSize := int64(0)

	// Walk through all files except the archive itself
	filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() != archiveName {
			totalSize += info.Size()
		}
		return nil
	})

	return float64(totalSize) / (1024 * 1024) // Convert to MB
}

// ValidateProductionBuild performs validation checks on the production build
func ValidateProductionBuild(outputDir string) error {
	PrintStep("Validating production build...")

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
			PrintSuccess(fmt.Sprintf("Server binary found: %s", binary))
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
			PrintSuccess(fmt.Sprintf("Metadata file found: %s", file))
		} else {
			PrintWarning("Optional file missing: %s", file)
		}
	}

	PrintSuccess("Production build validation completed")
	return nil
}

// CleanProductionBuild removes old production build artifacts
func CleanProductionBuild(rootDir, distDir string) error {
	PrintStep("Cleaning previous production builds...")

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
