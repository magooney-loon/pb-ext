package internal

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateProjectArchive creates a production build archive
func CreateProjectArchive(rootDir, outputDir string) error {
	PrintStep("📦", "Creating production build archive...")

	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("pb-deployer-production-%s.zip", timestamp)
	// Create zip file outside dist directory first to avoid infinite loop
	tempArchivePath := filepath.Join(rootDir, archiveName)

	distDir := filepath.Join(rootDir, "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return fmt.Errorf("dist directory not found - please run production build first")
	}

	file, err := os.Create(tempArchivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	var totalSize int64 = 0
	var fileCount int = 0

	err = filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the dist directory itself
		if path == distDir {
			return nil
		}

		// Get relative path from dist directory
		relPath, err := filepath.Rel(distDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Use forward slashes in zip files
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		if info.IsDir() {
			// Create directory entry in zip
			_, err := zipWriter.Create(relPath + "/")
			return err
		}

		// Add file to zip
		if err := addFileToZip(zipWriter, path, relPath); err != nil {
			return fmt.Errorf("failed to add file %s to zip: %w", path, err)
		}

		totalSize += info.Size()
		fileCount++
		return nil
	})

	if err != nil {
		os.Remove(tempArchivePath)
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Close zip writer to finalize the archive
	zipWriter.Close()
	file.Close()

	// Move the archive to the final location
	finalArchivePath := filepath.Join(outputDir, archiveName)
	if err := os.Rename(tempArchivePath, finalArchivePath); err != nil {
		// If rename fails, try copy and delete
		if copyErr := copyFile(tempArchivePath, finalArchivePath); copyErr != nil {
			os.Remove(tempArchivePath)
			return fmt.Errorf("failed to move archive: %w", err)
		}
		os.Remove(tempArchivePath)
	}

	// Get final archive size
	archiveInfo, err := os.Stat(finalArchivePath)
	if err != nil {
		PrintWarning("Could not get archive size information")
	} else {
		archiveSize := archiveInfo.Size()
		compressionRatio := float64(archiveSize) / float64(totalSize) * 100

		PrintSuccess("Production archive created successfully")
		PrintInfo("Archive: %s", archiveName)
		PrintInfo("Location: %s", finalArchivePath)
		PrintInfo("Files: %d", fileCount)
		PrintInfo("Original size: %s", formatBytes(totalSize))
		PrintInfo("Archive size: %s", formatBytes(archiveSize))
		PrintInfo("Compression: %.1f%%", 100.0-compressionRatio)
	}

	return nil
}

// GeneratePackageMetadata creates metadata files for the package
func GeneratePackageMetadata(rootDir, outputDir string) error {
	PrintStep("📋", "Generating package metadata...")

	goVersion := GetCommandOutput("go", "version")
	nodeVersion := GetCommandOutput("node", "--version")
	npmVersion := GetCommandOutput("npm", "--version")
	gitCommit := GetCommandOutput("git", "rev-parse", "HEAD")
	gitBranch := GetCommandOutput("git", "rev-parse", "--abbrev-ref", "HEAD")
	gitTag := GetCommandOutput("git", "describe", "--tags", "--exact-match")

	buildTime := time.Now().UTC().Format(time.RFC3339)

	// Create build info file
	buildInfoPath := filepath.Join(outputDir, "build-info.txt")
	buildInfoFile, err := os.Create(buildInfoPath)
	if err != nil {
		return fmt.Errorf("failed to create build info file: %w", err)
	}
	defer buildInfoFile.Close()

	fmt.Fprintf(buildInfoFile, "pb-deployer Production Build\n")
	fmt.Fprintf(buildInfoFile, "============================\n\n")
	fmt.Fprintf(buildInfoFile, "Build Time: %s\n", buildTime)
	fmt.Fprintf(buildInfoFile, "Build Type: Production\n\n")

	fmt.Fprintf(buildInfoFile, "Environment:\n")
	fmt.Fprintf(buildInfoFile, "  Go Version: %s\n", goVersion)
	fmt.Fprintf(buildInfoFile, "  Node.js: %s\n", nodeVersion)
	fmt.Fprintf(buildInfoFile, "  npm: %s\n", npmVersion)

	fmt.Fprintf(buildInfoFile, "\nGit Information:\n")
	fmt.Fprintf(buildInfoFile, "  Branch: %s\n", gitBranch)
	fmt.Fprintf(buildInfoFile, "  Commit: %s\n", gitCommit)
	if gitTag != "unknown" && gitTag != "" {
		fmt.Fprintf(buildInfoFile, "  Tag: %s\n", gitTag)
	}

	fmt.Fprintf(buildInfoFile, "\nContents:\n")
	fmt.Fprintf(buildInfoFile, "  - pb-deployer server binary\n")
	fmt.Fprintf(buildInfoFile, "  - Frontend static files (pb_public/)\n")
	fmt.Fprintf(buildInfoFile, "  - Build metadata and reports\n")

	// Create JSON metadata
	metadataPath := filepath.Join(outputDir, "package-metadata.json")
	metadataFile, err := os.Create(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer metadataFile.Close()

	jsonMetadata := fmt.Sprintf(`{
  "name": "pb-deployer",
  "version": "1.0.0",
  "buildTime": "%s",
  "buildType": "production",
  "environment": {
    "go": "%s",
    "node": "%s",
    "npm": "%s"
  },
  "git": {
    "branch": "%s",
    "commit": "%s",
    "tag": "%s"
  },
  "contents": [
    "server binary",
    "frontend assets",
    "build metadata"
  ]
}`, buildTime, goVersion, nodeVersion, npmVersion, gitBranch, gitCommit, gitTag)

	if _, err := metadataFile.WriteString(jsonMetadata); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	PrintSuccess("Package metadata generated")
	PrintInfo("Build info: %s", buildInfoPath)
	PrintInfo("JSON metadata: %s", metadataPath)
	return nil
}

// ValidateArchive performs basic validation on a created archive
func ValidateArchive(archivePath string) error {
	PrintStep("✅", "Validating archive...")

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	// Try to open the zip file
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer reader.Close()

	fileCount := len(reader.File)
	if fileCount == 0 {
		return fmt.Errorf("archive is empty")
	}

	// Check for required files
	requiredFiles := []string{"pb-deployer", "pb-deployer.exe"}
	hasServerBinary := false

	for _, file := range reader.File {
		for _, required := range requiredFiles {
			if strings.HasSuffix(file.Name, required) {
				hasServerBinary = true
				break
			}
		}
	}

	if !hasServerBinary {
		PrintWarning("Server binary not found in archive")
	}

	PrintSuccess("Archive validated successfully")
	PrintInfo("Files in archive: %d", fileCount)
	return nil
}

// addFileToZip adds a file to the zip archive
func addFileToZip(zipWriter *zip.Writer, filePath, zipPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
