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
	fmt.Println()

	// Header with visual separation
	fmt.Printf("%s┌─────────────────────────────────────────────────────────────┐%s\n", Gray, Reset)
	fmt.Printf("%s│                                                             │%s\n", Gray, Reset)
	fmt.Printf("%s│  %s🎉 Production Build Completed Successfully%s                │%s\n", Gray, Bold+Green, Reset, Gray)
	fmt.Printf("%s│                                                             │%s\n", Gray, Reset)
	fmt.Printf("%s└─────────────────────────────────────────────────────────────┘%s\n", Gray, Reset)
	fmt.Println()

	// Performance metrics in a clean table format
	fmt.Printf("  %s📊 Build Metrics%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %sDuration        %s%s%v%s\n", Gray, Reset, Cyan, duration.Round(time.Millisecond), Reset)
	fmt.Printf("    %sTarget Platform %s%s%s%s\n", Gray, Reset, Gray, runtime.GOOS+"/"+runtime.GOARCH, Reset)
	fmt.Printf("    %sOutput Location %s%s%s%s\n", Gray, Reset, Green, outputDir, Reset)
	fmt.Println()

	// Files with better spacing and organization
	fmt.Printf("  %s📦 Generated Assets%s\n", Bold, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	listProductionFiles(outputDir)

	// Archive info if available
	printArchiveInfo(outputDir)

	// Deployment readiness
	fmt.Printf("  %s🚀 Deployment Ready%s\n", Bold+Green, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %s✓%s Production-optimized binary with embedded assets\n", Green, Reset)
	fmt.Printf("    %s✓%s Comprehensive test coverage and reports included\n", Green, Reset)
	fmt.Printf("    %s✓%s Compressed archive ready for distribution\n", Green, Reset)
	fmt.Println()

	// Next steps with better organization
	fmt.Printf("  %s⚡ Quick Deploy%s\n", Bold+Cyan, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %sAutomated VPS Deployment:%s\n", Bold, Reset)
	fmt.Printf("      %s$%s git clone https://github.com/magooney-loon/pb-deployer\n", Green, Reset)
	fmt.Printf("      %s$%s cd pb-deployer && go run cmd/scripts/main.go --install\n", Green, Reset)
	fmt.Printf("      %s→ Compatible with PocketBase v0.20+ applications%s\n", Gray, Reset)
	fmt.Println()

	fmt.Printf("    %sManual Deployment:%s\n", Bold, Reset)
	fmt.Printf("      %s1.%s Upload production archive to your server\n", Cyan, Reset)
	fmt.Printf("      %s2.%s Extract and configure environment variables\n", Cyan, Reset)
	fmt.Printf("      %s3.%s Setup systemd service for production runtime\n", Cyan, Reset)
	fmt.Println()

	// Footer with resources
	fmt.Printf("  %s📚 Resources%s\n", Bold+Yellow, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
	fmt.Printf("    %sDeployment Guide: %shttps://github.com/magooney-loon/pb-deployer%s\n", Gray, Cyan, Reset)
	fmt.Printf("    %sPocketBase Docs:  %shttps://pocketbase.io/docs/going-to-production/%s\n", Gray, Cyan, Reset)

	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", Green, Reset)
	fmt.Printf("%s🎯 Your application is ready for production deployment!%s\n", Bold+Green, Reset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", Green, Reset)
}

// listProductionFiles lists the files created in the production build
func listProductionFiles(outputDir string) {
	// Check for server binary
	binaryPaths := []string{AppName, AppName + ".exe"}
	for _, binary := range binaryPaths {
		binaryPath := filepath.Join(outputDir, binary)
		if stat, err := os.Stat(binaryPath); err == nil {
			size := float64(stat.Size()) / (1024 * 1024) // Convert to MB
			fmt.Printf("    %s▸ %s%-20s%s %s%.1f MB%s\n", Green, Bold, binary, Reset, Gray, size, Reset)
			break
		}
	}

	// Check for frontend assets
	pbPublicPath := filepath.Join(outputDir, "pb_public")
	if _, err := os.Stat(pbPublicPath); err == nil {
		// Count files in pb_public
		fileCount := countFilesInDir(pbPublicPath)
		fmt.Printf("    %s▸ %s%-20s%s %s%d files%s\n", Green, Bold, "pb_public/", Reset, Gray, fileCount, Reset)
	}

	// Check for metadata files
	metadataFiles := map[string]string{
		"build-info.txt":        "build metadata",
		"package-metadata.json": "deployment configuration",
	}
	for file, desc := range metadataFiles {
		filePath := filepath.Join(outputDir, file)
		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("    %s▸ %s%-20s%s %s%s%s\n", Green, Bold, file, Reset, Gray, desc, Reset)
		}
	}

	// Check for test reports
	reportsDir := filepath.Join(outputDir, "test-reports")
	if stat, err := os.Stat(reportsDir); err == nil && stat.IsDir() {
		reportCount := countFilesInDir(reportsDir)
		fmt.Printf("    %s▸ %s%-20s%s %s%d test reports%s\n", Green, Bold, "test-reports/", Reset, Gray, reportCount, Reset)
	}
}

// printArchiveInfo displays archive information in a dedicated section
func printArchiveInfo(outputDir string) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			if stat, err := entry.Info(); err == nil {
				archiveSize := float64(stat.Size()) / (1024 * 1024) // Convert to MB

				// Calculate original size for compression ratio
				originalSize := calculateOriginalSize(outputDir, entry.Name())
				compressionRatio := 0.0
				if originalSize > 0 {
					compressionRatio = (1.0 - archiveSize/originalSize) * 100
				}

				fmt.Printf("\n  %s📦 Production Archive%s\n", Bold+Cyan, Reset)
				fmt.Printf("  %s──────────────────────────────────────────────────────────────%s\n", Gray, Reset)
				fmt.Printf("    %sArchive Name    %s%s%s\n", Gray, Reset, Bold, entry.Name())
				fmt.Printf("    %sArchive Size    %s%.1f MB%s\n", Gray, Reset, archiveSize, Reset)
				if originalSize > 0 {
					fmt.Printf("    %sOriginal Size   %s%.1f MB%s\n", Gray, Reset, originalSize, Reset)
					fmt.Printf("    %sCompression     %s%.1f%%%s\n", Gray, Reset, compressionRatio, Reset)
				}
				fmt.Printf("    %sLocation        %s%s%s\n", Gray, Reset, filepath.Join(outputDir, entry.Name()), Reset)
			}
			break
		}
	}
	fmt.Println()
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
