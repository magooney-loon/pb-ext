package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	// Define command line flags
	installDeps := flag.Bool("install", false, "Install frontend dependencies")
	buildOnly := flag.Bool("build-only", false, "Build frontend without running the server")
	runOnly := flag.Bool("run-only", false, "Run the server without building the frontend")
	production := flag.Bool("production", false, "Create a production build in dist folder")
	distDir := flag.String("dist", "dist", "Output directory for production build")
	flag.Parse()

	// Get the root directory of the project
	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Handle production build
	if *production {
		productionBuild(rootDir, *installDeps, *distDir)
		return
	}

	// If not in run-only mode, build the frontend
	if !*runOnly {
		buildFrontend(rootDir, *installDeps)
	}

	// Run the server unless in build-only mode
	if !*buildOnly {
		runServer(rootDir)
	}
}

func productionBuild(rootDir string, installDeps bool, distDir string) {
	fmt.Println("Creating production build...")

	// Create output directory
	outputDir := filepath.Join(rootDir, distDir)
	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("Failed to clean dist directory: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create dist directory: %v", err)
	}

	// Build the frontend
	fmt.Println("Building frontend for production...")
	frontendDir := filepath.Join(rootDir, "frontend")

	// Check if frontend directory exists
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		log.Fatalf("Frontend directory not found at %s. Please make sure it exists.", frontendDir)
	}

	// Check if package.json exists
	packageJSON := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		log.Fatalf("package.json not found at %s. Please make sure it exists.", packageJSON)
	}

	// Install dependencies if needed
	if installDeps {
		fmt.Println("Installing frontend dependencies...")
		cmd := exec.Command("npm", "install")
		cmd.Dir = frontendDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to install frontend dependencies: %v", err)
		}
	}

	// Build the frontend
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build frontend: %v", err)
	}

	// Create pb_public directory inside dist
	pbPublicDir := filepath.Join(outputDir, "pb_public")
	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		log.Fatalf("Failed to create pb_public directory: %v", err)
	}

	// Find the build output directory
	buildDir := filepath.Join(frontendDir, "build")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		buildDir = filepath.Join(frontendDir, "static")
		if _, err := os.Stat(buildDir); os.IsNotExist(err) {
			buildDir = filepath.Join(frontendDir, "dist")
			if _, err := os.Stat(buildDir); os.IsNotExist(err) {
				log.Fatalf("Could not find frontend build directory. Please check your frontend build configuration.")
			}
		}
	}

	// Copy build files to pb_public
	fmt.Println("Copying frontend build to pb_public...")

	// Simple file copy function using exec.Command
	cpCmd := exec.Command("cp", "-r", buildDir+"/.", pbPublicDir)
	cpCmd.Stdout = os.Stdout
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		log.Fatalf("Failed to copy frontend build to pb_public: %v", err)
	}

	// Build the server binary
	fmt.Println("Building server binary...")

	// Determine binary name based on OS
	binaryName := "myapp"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	outputPath := filepath.Join(outputDir, binaryName)

	buildCmd := exec.Command("go", "build", "-o", outputPath, filepath.Join(rootDir, "cmd/server/main.go"))
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		log.Fatalf("Failed to build server binary: %v", err)
	}

	fmt.Printf("Production build completed. Files are in the '%s' directory.\n", distDir)
}

func buildFrontend(rootDir string, installDeps bool) {
	// Build frontend
	fmt.Println("Building frontend...")
	frontendDir := filepath.Join(rootDir, "frontend")

	// Check if frontend directory exists
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		log.Fatalf("Frontend directory not found at %s. Please make sure it exists.", frontendDir)
	}

	// Check if package.json exists
	packageJSON := filepath.Join(frontendDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		log.Fatalf("package.json not found at %s. Please make sure it exists.", packageJSON)
	}

	// Install dependencies if needed
	if installDeps {
		fmt.Println("Installing frontend dependencies...")
		cmd := exec.Command("npm", "install")
		cmd.Dir = frontendDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to install frontend dependencies: %v", err)
		}
	}

	// Build the frontend
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build frontend: %v", err)
	}

	// Ensure pb_public directory exists and is empty
	pbPublicDir := filepath.Join(rootDir, "pb_public")
	if err := os.RemoveAll(pbPublicDir); err != nil {
		log.Fatalf("Failed to clean pb_public directory: %v", err)
	}
	if err := os.MkdirAll(pbPublicDir, 0755); err != nil {
		log.Fatalf("Failed to create pb_public directory: %v", err)
	}

	// Find the build output directory
	// SvelteKit's build directory could be different depending on configuration
	// Try both 'build' and 'static' directories
	buildDir := filepath.Join(frontendDir, "build")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		buildDir = filepath.Join(frontendDir, "static")
		if _, err := os.Stat(buildDir); os.IsNotExist(err) {
			buildDir = filepath.Join(frontendDir, "dist")
			if _, err := os.Stat(buildDir); os.IsNotExist(err) {
				log.Fatalf("Could not find frontend build directory. Please check your frontend build configuration.")
			}
		}
	}

	// Copy build files to pb_public
	fmt.Println("Copying frontend build to pb_public...")

	// Simple file copy function using exec.Command
	cpCmd := exec.Command("cp", "-r", buildDir+"/.", pbPublicDir)
	cpCmd.Stdout = os.Stdout
	cpCmd.Stderr = os.Stderr
	if err := cpCmd.Run(); err != nil {
		log.Fatalf("Failed to copy frontend build to pb_public: %v", err)
	}
}

func runServer(rootDir string) {
	// Run the server
	fmt.Println("Starting server...")
	serverCmd := exec.Command("go", "run", filepath.Join(rootDir, "cmd/server/main.go"), "serve")
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	if err := serverCmd.Run(); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
