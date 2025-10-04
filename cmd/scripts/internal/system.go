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

	PrintStep("🔍", "Checking system requirements...")

	for _, req := range requirements {
		if !CheckCommand(req.command, req.args...) {
			PrintError("%s is not installed or not in PATH", req.name)
			return fmt.Errorf("%s is required but not found", req.name)
		}
		PrintSuccess("%s is available", req.name)
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
