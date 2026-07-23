package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RootCheckResult contains the result of a root privilege check.
type RootCheckResult struct {
	Authorized bool   `json:"authorized"`
	Error      string `json:"error,omitempty"`
	Message    string `json:"message"`
	Output     string `json:"output,omitempty"`
}

// CheckRoot verifies that the aestheticos-helper can be executed via sudo without a password.
// This is used by both CLI and GUI to ensure proper setup before installation.
func CheckRoot(helperPath string) RootCheckResult {
	if helperPath == "" {
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_found",
			Message:    "Helper program not found. Please make sure aestheticos-helper is installed.",
		}
	}

	// Check if helper binary exists and is executable
	info, err := os.Stat(helperPath)
	if os.IsNotExist(err) {
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_found",
			Message:    "Helper program not found. Please make sure aestheticos-helper is installed.",
		}
	}
	if err != nil {
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_found",
			Message:    "Helper program not found. Please make sure aestheticos-helper is installed.",
		}
	}

	// Check if helper is executable
	if info.Mode()&0111 == 0 {
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_executable",
			Message:    "Helper program is not executable. Please check file permissions.",
		}
	}

	// Test if sudo can run the helper without a password prompt
	cmd := exec.Command("sudo", "-n", helperPath, "run", "id")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// If command succeeded, sudo is configured correctly
	if err == nil {
		return RootCheckResult{
			Authorized: true,
			Message:    "Ready to install",
			Output:     outputStr,
		}
	}

	// Command failed - check if it's a password prompt issue
	if strings.Contains(outputStr, "a password is required") ||
		strings.Contains(outputStr, "sorry, a password is required") {
		return RootCheckResult{
			Authorized: false,
			Error:      "sudo_password_required",
			Message:    "Please configure sudo to run without a password for the helper program.",
		}
	}

	// Other error
	return RootCheckResult{
		Authorized: false,
		Error:      "sudo_failed",
		Message:    fmt.Sprintf("Could not verify sudo configuration. Error: %s", err.Error()),
	}
}
