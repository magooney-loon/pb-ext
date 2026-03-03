package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckSystemRequirements verifies that all required tools are installed
func CheckSystemRequirements() error {
	requirements := []struct {
		name    string
		command string
		args    []string
	}{
		{"Go", "go", []string{"version"}},
		{"Node.js", "node", []string{"--version"}},
		{"npm", "npm", []string{"--version"}},
		{"Git", "git", []string{"--version"}},
	}

	PrintSection("System Requirements")

	for _, req := range requirements {
		if !CheckCommand(req.command, req.args...) {
			PrintSubItem("✗", fmt.Sprintf("%s not found", req.name))
			return fmt.Errorf("%s required", req.name)
		}
		version := GetCommandOutput(req.command, req.args...)
		// Clean up version output
		if req.name == "Go" && strings.Contains(version, "go version") {
			parts := strings.Fields(version)
			if len(parts) >= 3 {
				version = parts[2] // Extract just the version number
			}
		}
		if req.name == "Node.js" || req.name == "npm" {
			version = strings.TrimPrefix(version, "v") // Remove 'v' prefix
		}
		PrintSubItem("✓", fmt.Sprintf("%s ready (%s)", req.name, version))
	}

	return nil
}

// CheckCommand runs a command and returns true if it succeeds
func CheckCommand(command string, args ...string) bool {
	cmd := exec.Command(command, args...)
	return cmd.Run() == nil
}

// GetCommandOutput runs a command and returns its output as a string
func GetCommandOutput(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
