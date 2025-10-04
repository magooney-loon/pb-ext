package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RunServer starts the development server
func RunServer(rootDir string) error {
	PrintHeader("🚀 STARTING SERVER")

	cmd := exec.Command("go", "run", filepath.Join(rootDir, "cmd/server/main.go"), "serve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	PrintStep("🌐", "Server starting...")
	return cmd.Run()
}

// ValidateServerSetup checks if the server directory and files exist
func ValidateServerSetup(rootDir string) error {
	PrintStep("🔍", "Validating server setup...")

	serverDir := filepath.Join(rootDir, "cmd", "server")
	if _, err := os.Stat(serverDir); os.IsNotExist(err) {
		return fmt.Errorf("server directory not found at %s", serverDir)
	}

	serverMainFile := filepath.Join(serverDir, "main.go")
	if _, err := os.Stat(serverMainFile); os.IsNotExist(err) {
		return fmt.Errorf("server main file not found at %s", serverMainFile)
	}

	PrintSuccess("Server setup validated")
	return nil
}

// StartServerWithTimeout starts the server with a timeout mechanism
func StartServerWithTimeout(rootDir string, timeout time.Duration) error {
	PrintHeader("🚀 STARTING SERVER WITH TIMEOUT")

	if err := ValidateServerSetup(rootDir); err != nil {
		return err
	}

	cmd := exec.Command("go", "run", filepath.Join(rootDir, "cmd/server/main.go"), "serve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	PrintStep("🌐", "Server starting with %v timeout...", timeout)

	// Start the command in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	// Wait for either completion or timeout
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Errorf("server startup timed out after %v", timeout)
	}
}

// CheckServerHealth performs a basic health check on the server setup
func CheckServerHealth(rootDir string) error {
	PrintStep("❤️", "Checking server health...")

	// Validate server setup
	if err := ValidateServerSetup(rootDir); err != nil {
		return err
	}

	// Check if we can compile the server
	PrintStep("🔨", "Testing server compilation...")
	cmd := exec.Command("go", "build", "-o", "/tmp/pb-deployer-test", filepath.Join(rootDir, "cmd/server/main.go"))
	cmd.Dir = rootDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("server compilation failed: %w", err)
	}

	// Clean up test binary
	os.Remove("/tmp/pb-deployer-test")

	PrintSuccess("Server health check passed")
	return nil
}

// GetServerInfo returns information about the server configuration
func GetServerInfo(rootDir string) (map[string]string, error) {
	info := make(map[string]string)

	serverDir := filepath.Join(rootDir, "cmd", "server")
	info["serverDir"] = serverDir
	info["mainFile"] = filepath.Join(serverDir, "main.go")
	info["goVersion"] = GetCommandOutput("go", "version")

	// Check if server directory exists
	if _, err := os.Stat(serverDir); os.IsNotExist(err) {
		info["status"] = "not_found"
		return info, fmt.Errorf("server directory not found")
	}

	info["status"] = "ready"
	return info, nil
}

// PrepareServerEnvironment sets up the environment for server execution
func PrepareServerEnvironment(rootDir string) error {
	PrintStep("🔧", "Preparing server environment...")

	// Ensure pb_public directory exists (server expects this)
	pbPublicDir := filepath.Join(rootDir, "pb_public")
	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		PrintWarning("Failed to create pb_public directory: %v", err)
	}

	// Validate Go module is properly set up
	goModPath := filepath.Join(rootDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found - please ensure Go module is initialized")
	}

	PrintSuccess("Server environment prepared")
	return nil
}
