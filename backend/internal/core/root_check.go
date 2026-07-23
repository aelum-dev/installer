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
	fmt.Printf("[DEBUG] CheckRoot called with helperPath: %s\n", helperPath)

	if helperPath == "" {
		fmt.Printf("[DEBUG] CheckRoot: helperPath is empty\n")
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_found",
			Message:    "Helper program not found. Please make sure aestheticos-helper is installed.",
		}
	}

	// Check if helper binary exists and is executable
	info, err := os.Stat(helperPath)
	if os.IsNotExist(err) {
		fmt.Printf("[DEBUG] CheckRoot: helper not found at %s: %v\n", helperPath, err)
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_found",
			Message:    "Helper program not found. Please make sure aestheticos-helper is installed.",
		}
	}
	if err != nil {
		fmt.Printf("[DEBUG] CheckRoot: error stat'ing helper: %v\n", err)
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_found",
			Message:    "Helper program not found. Please make sure aestheticos-helper is installed.",
		}
	}

	// Check if helper is executable
	if info.Mode()&0111 == 0 {
		fmt.Printf("[DEBUG] CheckRoot: helper not executable, mode: %o\n", info.Mode().Perm())
		return RootCheckResult{
			Authorized: false,
			Error:      "helper_not_executable",
			Message:    "Helper program is not executable. Please check file permissions.",
		}
	}
	fmt.Printf("[DEBUG] CheckRoot: helper found and executable at %s\n", helperPath)

	// Test if sudo can run the helper without a password prompt
	cmd := exec.Command("sudo", "-n", helperPath, "run", "id")
	fmt.Printf("[DEBUG] CheckRoot: running command: %v\n", cmd.Args)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	fmt.Printf("[DEBUG] CheckRoot: command output: %q\n", outputStr)
	fmt.Printf("[DEBUG] CheckRoot: command error: %v\n", err)

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
		fmt.Printf("[DEBUG] CheckRoot: password required detected\n")
		return RootCheckResult{
			Authorized: false,
			Error:      "sudo_password_required",
			Message:    "Please configure sudo to run without a password for the helper program.",
		}
	}

	// Other error
	fmt.Printf("[DEBUG] CheckRoot: other error\n")
	return RootCheckResult{
		Authorized: false,
		Error:      "sudo_failed",
		Message:    fmt.Sprintf("Could not verify sudo configuration. Error: %s", err.Error()),
	}
}
