package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallDependencies installs both Go and npm dependencies
func InstallDependencies(rootDir, frontendDir string) error {
	PrintStep("📦", "Installing dependencies...")

	if err := InstallGoDependencies(rootDir); err != nil {
		return err
	}

	if err := InstallNpmDependencies(frontendDir); err != nil {
		return err
	}

	PrintSuccess("All dependencies installed successfully")
	return nil
}

// InstallGoDependencies installs and tidies Go module dependencies
func InstallGoDependencies(rootDir string) error {
	PrintStep("🏗️", "Installing Go dependencies...")

	PrintStep("🧹", "Tidying Go modules...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	PrintStep("📥", "Downloading Go dependencies...")
	cmd = exec.Command("go", "mod", "download")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod download failed: %w", err)
	}

	PrintSuccess("Go dependencies installed")
	return nil
}

// InstallNpmDependencies installs npm dependencies in the frontend directory
func InstallNpmDependencies(frontendDir string) error {
	PrintStep("📦", "Installing npm dependencies...")

	// Check if package-lock.json exists to decide between npm ci and npm install
	packageLockPath := filepath.Join(frontendDir, "package-lock.json")
	var cmd *exec.Cmd

	if _, err := os.Stat(packageLockPath); err == nil {
		PrintStep("🔒", "Using npm ci (package-lock.json found)...")
		cmd = exec.Command("npm", "ci")
	} else {
		PrintStep("🔧", "Using npm install...")
		cmd = exec.Command("npm", "install")
	}

	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm dependency installation failed: %w", err)
	}

	PrintSuccess("npm dependencies installed")
	return nil
}

// ValidateDependencies checks if all required dependencies are properly installed
func ValidateDependencies(rootDir, frontendDir string) error {
	PrintStep("🔍", "Validating dependencies...")

	// Check Go module file
	goModPath := filepath.Join(rootDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found at %s", goModPath)
	}

	// Check package.json
	packageJSONPath := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found at %s", packageJSONPath)
	}

	// Check node_modules exists after npm install
	nodeModulesPath := filepath.Join(frontendDir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		PrintWarning("node_modules directory not found - dependencies may not be installed")
		return fmt.Errorf("node_modules directory not found at %s", nodeModulesPath)
	}

	PrintSuccess("All dependencies validated")
	return nil
}
