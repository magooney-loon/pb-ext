package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/magooney-loon/pb-ext/cmd/scripts/internal"
)

func main() {
	// Parse command line flags
	installDeps := flag.Bool("install", false, "Install project dependencies")
	buildOnly := flag.Bool("build-only", false, "Build frontend without running the server")
	runOnly := flag.Bool("run-only", false, "Run the server without building the frontend")
	production := flag.Bool("production", false, "Create a production build in dist folder")
	testOnly := flag.Bool("test-only", false, "Run test suite and generate reports only")
	distDir := flag.String("dist", "dist", "Output directory for production build")
	help := flag.Bool("help", false, "Show help and usage information")
	flag.Parse()

	// Show help if requested
	if *help {
		internal.ShowHelp()
		return
	}

	// Determine operation type for banner
	operation := "DEVELOPMENT"
	if *production {
		operation = "PRODUCTION"
	} else if *testOnly {
		operation = "TESTING"
	}
	internal.PrintBanner(operation)

	// Get root directory
	rootDir, err := os.Getwd()
	if err != nil {
		internal.PrintError("Failed to get current directory: %v", err)
		os.Exit(1)
	}

	// Execute the appropriate operation
	start := time.Now()

	switch {
	case *testOnly:
		err = handleTestOnlyMode(rootDir, *distDir)
	case *production:
		err = handleProductionMode(rootDir, *installDeps, *distDir)
	case *buildOnly:
		err = handleBuildOnlyMode(rootDir, *installDeps)
	case *runOnly:
		err = handleRunOnlyMode(rootDir)
	default:
		err = handleDevelopmentMode(rootDir, *installDeps)
	}

	if err != nil {
		internal.PrintError("%v", err)
		os.Exit(1)
	}

	// Print completion summary for non-server modes
	if !*runOnly && !isServerMode() {
		duration := time.Since(start)
		if *production {
			internal.PrintBuildSummary(duration, true)
		} else if *testOnly {
			internal.PrintTestSummary(duration)
		} else {
			internal.PrintBuildSummary(duration, false)
		}
	}
}

// handleTestOnlyMode runs only the test suite
func handleTestOnlyMode(rootDir, distDir string) error {
	internal.PrintHeader("🧪 TEST MODE")

	if err := internal.CheckSystemRequirements(); err != nil {
		return fmt.Errorf("system requirements not met: %w", err)
	}

	return internal.TestOnlyMode(rootDir, distDir)
}

// handleProductionMode creates a complete production build
func handleProductionMode(rootDir string, installDeps bool, distDir string) error {
	internal.PrintHeader("🚀 PRODUCTION MODE")

	return internal.ProductionBuild(rootDir, installDeps, distDir)
}

// handleBuildOnlyMode builds the frontend without starting the server
func handleBuildOnlyMode(rootDir string, installDeps bool) error {
	internal.PrintHeader("🔨 BUILD MODE")

	if err := internal.CheckSystemRequirements(); err != nil {
		return fmt.Errorf("system requirements not met: %w", err)
	}

	return internal.BuildFrontend(rootDir, installDeps)
}

// handleRunOnlyMode starts the server without building
func handleRunOnlyMode(rootDir string) error {
	internal.PrintHeader("🚀 RUN MODE")

	if err := internal.CheckSystemRequirements(); err != nil {
		return fmt.Errorf("system requirements not met: %w", err)
	}

	if err := internal.ValidateServerSetup(rootDir); err != nil {
		return fmt.Errorf("server setup validation failed: %w", err)
	}

	if err := internal.PrepareServerEnvironment(rootDir); err != nil {
		return fmt.Errorf("server environment preparation failed: %w", err)
	}

	return internal.RunServer(rootDir)
}

// handleDevelopmentMode is the default mode - build frontend and start server
func handleDevelopmentMode(rootDir string, installDeps bool) error {
	internal.PrintHeader("🛠️ DEVELOPMENT MODE")

	if err := internal.CheckSystemRequirements(); err != nil {
		return fmt.Errorf("system requirements not met: %w", err)
	}

	// Build frontend first
	if err := internal.BuildFrontend(rootDir, installDeps); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Prepare and start server
	if err := internal.ValidateServerSetup(rootDir); err != nil {
		return fmt.Errorf("server setup validation failed: %w", err)
	}

	if err := internal.PrepareServerEnvironment(rootDir); err != nil {
		return fmt.Errorf("server environment preparation failed: %w", err)
	}

	internal.PrintSuccess("Build completed successfully")
	internal.PrintInfo("Starting development server...")

	return internal.RunServer(rootDir)
}

// isServerMode checks if we're in a mode that starts the server
func isServerMode() bool {
	runOnly := flag.Lookup("run-only").Value.String() == "true"
	production := flag.Lookup("production").Value.String() == "true"
	buildOnly := flag.Lookup("build-only").Value.String() == "true"
	testOnly := flag.Lookup("test-only").Value.String() == "true"

	// Server runs in default mode (development) and run-only mode
	return runOnly || (!production && !buildOnly && !testOnly)
}
