package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// FrontendType represents the type of frontend setup
type FrontendType int

const (
	FrontendTypeNone FrontendType = iota
	FrontendTypeStatic
	FrontendTypeNpm
)

// DetectFrontendType determines what kind of frontend setup exists
func DetectFrontendType(frontendDir string) FrontendType {
	// Check if frontend directory exists
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return FrontendTypeNone
	}

	// Check if package.json exists (npm-based frontend)
	packageJSON := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(packageJSON); err == nil {
		return FrontendTypeNpm
	}

	// Frontend directory exists but no package.json (static files)
	return FrontendTypeStatic
}

// ValidateFrontendSetup checks if the frontend directory and package.json exist
func ValidateFrontendSetup(frontendDir string) error {
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found at %s", frontendDir)
	}

	packageJSON := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found at %s", packageJSON)
	}

	return nil
}

// CopyStaticFiles copies static files from frontend directory to pb_public
func CopyStaticFiles(rootDir, frontendDir string) error {
	pbPublicDir := filepath.Join(rootDir, "pb_public")
	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		return fmt.Errorf("failed to create pb_public directory: %w", err)
	}

	// Copy all files from frontend to pb_public
	err := filepath.Walk(frontendDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(frontendDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(pbPublicDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})

	if err != nil {
		return fmt.Errorf("failed to copy static files: %w", err)
	}

	return nil
}

// BuildFrontend builds the frontend for development
func BuildFrontend(rootDir string, installDeps bool) error {
	frontendDir := filepath.Join(rootDir, "frontend")
	frontendType := DetectFrontendType(frontendDir)

	switch frontendType {
	case FrontendTypeNone:
		PrintSubItem("i", "No frontend found, skipping")
		return nil

	case FrontendTypeStatic:
		PrintSection("Build Assets")
		PrintBuildStep("Copying static files", "frontend → pb_public")
		return CopyStaticFiles(rootDir, frontendDir)

	case FrontendTypeNpm:
		PrintSection("Build Assets")
		PrintBuildStep("Frontend build", "npm")

		if err := ValidateFrontendSetup(frontendDir); err != nil {
			return err
		}

		if installDeps {
			if err := InstallDependencies(rootDir, frontendDir); err != nil {
				return err
			}
		}

		if err := BuildFrontendCore(frontendDir); err != nil {
			return err
		}

		return CopyFrontendToPbPublic(rootDir, frontendDir)

	default:
		return fmt.Errorf("unknown frontend type")
	}
}

// BuildFrontendProduction builds the frontend for production
func BuildFrontendProduction(rootDir string, installDeps bool) error {
	return BuildFrontend(rootDir, installDeps)
}

// BuildFrontendCore runs the actual npm build process
func BuildFrontendCore(frontendDir string) error {
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm run build failed: %w", err)
	}

	duration := time.Since(start)
	PrintSubItem("✓", fmt.Sprintf("Frontend built (%v)", duration.Round(time.Millisecond)))
	return nil
}

// CopyFrontendToPbPublic copies the built frontend to the pb_public directory
func CopyFrontendToPbPublic(rootDir, frontendDir string) error {
	pbPublicDir := filepath.Join(rootDir, "pb_public")

	if err := os.RemoveAll(pbPublicDir); err != nil {
		return fmt.Errorf("failed to clean pb_public: %w", err)
	}

	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		return fmt.Errorf("failed to create pb_public: %w", err)
	}

	buildDir, err := FindBuildDirectory(frontendDir)
	if err != nil {
		return fmt.Errorf("failed to find frontend build: %w", err)
	}

	if err := copyDir(buildDir, pbPublicDir); err != nil {
		return fmt.Errorf("failed to copy frontend build: %w", err)
	}

	return nil
}

// CopyFrontendToDist copies the built frontend to the dist directory for production
func CopyFrontendToDist(rootDir, outputDir string) error {
	frontendDir := filepath.Join(rootDir, "frontend")
	frontendType := DetectFrontendType(frontendDir)

	pbPublicDir := filepath.Join(outputDir, "pb_public")
	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		return fmt.Errorf("failed to create dist pb_public: %w", err)
	}

	switch frontendType {
	case FrontendTypeNone:
		// Create an empty pb_public directory or copy from existing pb_public if it exists
		existingPbPublic := filepath.Join(rootDir, "pb_public")
		if _, err := os.Stat(existingPbPublic); err == nil {
			if err := copyDir(existingPbPublic, pbPublicDir); err != nil {
				return fmt.Errorf("failed to copy existing pb_public to dist: %w", err)
			}
		}
		return nil

	case FrontendTypeStatic:
		if err := copyDir(frontendDir, pbPublicDir); err != nil {
			return fmt.Errorf("failed to copy static frontend to dist: %w", err)
		}
		return nil

	case FrontendTypeNpm:
		buildDir, err := FindBuildDirectory(frontendDir)
		if err != nil {
			return fmt.Errorf("failed to find frontend build for dist: %w", err)
		}
		if err := copyDir(buildDir, pbPublicDir); err != nil {
			return fmt.Errorf("failed to copy frontend build to dist: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown frontend type")
	}
}

// BuildServerBinary builds the server binary for production
func BuildServerBinary(rootDir, outputDir string) error {
	PrintSection("Build Binary")

	binaryName := AppName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	outputPath := filepath.Join(outputDir, binaryName)

	PrintBuildStep("Compiling server", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))

	start := time.Now()
	cmd := exec.Command("go", "build",
		"-ldflags", "-s -w",
		"-o", outputPath,
		"./cmd/server")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("server binary build failed: %w", err)
	}

	duration := time.Since(start)

	// Get binary size
	if stat, err := os.Stat(outputPath); err == nil {
		size := float64(stat.Size()) / (1024 * 1024)
		PrintSubItem("✓", fmt.Sprintf("Binary built: %s (%.1f MB, %v)", binaryName, size, duration.Round(time.Millisecond)))
	} else {
		PrintSubItem("✓", fmt.Sprintf("Binary built (%v)", duration.Round(time.Millisecond)))
	}

	return nil
}

// FindBuildDirectory finds the frontend build output directory
func FindBuildDirectory(frontendDir string) (string, error) {
	possibleDirs := []string{"build", "dist", "static"}

	for _, dir := range possibleDirs {
		buildDir := filepath.Join(frontendDir, dir)
		if _, err := os.Stat(buildDir); err == nil {
			return buildDir, nil
		}
	}

	// Check if frontend directory has any files to copy
	entries, err := os.ReadDir(frontendDir)
	if err != nil {
		return "", fmt.Errorf("could not read frontend directory: %w", err)
	}

	hasFiles := false
	for _, entry := range entries {
		if !entry.IsDir() {
			hasFiles = true
			break
		}
	}

	if hasFiles {
		PrintInfo("Using frontend dir directly")
		return frontendDir, nil
	}

	return "", fmt.Errorf("no build directory found in %v and no files to copy in frontend directory", possibleDirs)
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate the destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}
