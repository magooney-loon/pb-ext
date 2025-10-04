package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// ValidateFrontendSetup checks if the frontend directory and package.json exist
func ValidateFrontendSetup(frontendDir string) error {
	PrintStep("🔍", "Validating frontend setup...")

	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found at %s", frontendDir)
	}

	packageJSON := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found at %s", packageJSON)
	}

	PrintSuccess("Frontend setup validated")
	return nil
}

// BuildFrontend builds the frontend for development
func BuildFrontend(rootDir string, installDeps bool) error {
	PrintHeader("🔨 FRONTEND BUILD")

	frontendDir := filepath.Join(rootDir, "frontend")

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
}

// BuildFrontendProduction builds the frontend for production
func BuildFrontendProduction(rootDir string, installDeps bool) error {
	PrintStep("🏗️", "Building frontend for production...")
	return BuildFrontend(rootDir, installDeps)
}

// BuildFrontendCore runs the actual npm build process
func BuildFrontendCore(frontendDir string) error {
	PrintStep("⚙️", "Building frontend...")

	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm run build failed: %w", err)
	}

	duration := time.Since(start)
	PrintSuccess("Frontend built successfully in %v", duration.Round(time.Millisecond))
	return nil
}

// CopyFrontendToPbPublic copies the built frontend to the pb_public directory
func CopyFrontendToPbPublic(rootDir, frontendDir string) error {
	PrintStep("📂", "Copying frontend build to pb_public...")

	pbPublicDir := filepath.Join(rootDir, "pb_public")

	if err := os.RemoveAll(pbPublicDir); err != nil {
		return fmt.Errorf("failed to clean pb_public: %w", err)
	}

	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		return fmt.Errorf("failed to create pb_public: %w", err)
	}

	buildDir := FindBuildDirectory(frontendDir)
	if err := copyDir(buildDir, pbPublicDir); err != nil {
		return fmt.Errorf("failed to copy frontend build: %w", err)
	}

	PrintSuccess("Frontend copied to pb_public successfully")
	return nil
}

// CopyFrontendToDist copies the built frontend to the dist directory for production
func CopyFrontendToDist(rootDir, outputDir string) error {
	PrintStep("📁", "Copying frontend to dist...")

	pbPublicDir := filepath.Join(outputDir, "pb_public")
	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		return fmt.Errorf("failed to create dist pb_public: %w", err)
	}

	frontendDir := filepath.Join(rootDir, "frontend")
	buildDir := FindBuildDirectory(frontendDir)

	if err := copyDir(buildDir, pbPublicDir); err != nil {
		return fmt.Errorf("failed to copy frontend to dist: %w", err)
	}

	PrintSuccess("Frontend copied to dist successfully")
	return nil
}

// BuildServerBinary builds the server binary for production
func BuildServerBinary(rootDir, outputDir string) error {
	PrintStep("🏗️", "Building server binary...")

	binaryName := "pb-deployer"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	outputPath := filepath.Join(outputDir, binaryName)

	start := time.Now()
	cmd := exec.Command("go", "build",
		"-ldflags", "-s -w",
		"-o", outputPath,
		filepath.Join(rootDir, "cmd/server/main.go"))
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("server binary build failed: %w", err)
	}

	duration := time.Since(start)
	PrintSuccess("Server binary built successfully in %v", duration.Round(time.Millisecond))
	PrintInfo("Binary location: %s", outputPath)
	return nil
}

// FindBuildDirectory finds the frontend build output directory
func FindBuildDirectory(frontendDir string) string {
	possibleDirs := []string{"build", "dist", "static"}

	for _, dir := range possibleDirs {
		buildDir := filepath.Join(frontendDir, dir)
		if _, err := os.Stat(buildDir); err == nil {
			return buildDir
		}
	}

	log.Fatalf("Could not find frontend build directory in: %v", possibleDirs)
	return ""
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
