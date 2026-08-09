package gui

import (
	"os/exec"

	"aestheticos-installer/backend/install"
)

// CheckRoot verifies that sudo is available and can run commands.
// This is called by the GUI to ensure the user has sudo access
// before allowing installation to proceed.
func (a *App) CheckRoot() install.RootCheckResult {
	install.GetLogger().Info("Checking sudo availability...")

	// Check if sudo is available and can run a simple command
	cmd := exec.Command("sudo", "-n", "true")
	if err := cmd.Run(); err != nil {
		return install.RootCheckResult{
			Authorized: false,
			Message:    "The installer needs sudo access. Please add your user to the sudoers file.",
		}
	}

	install.GetLogger().Info("Root check passed: sudo is available")
	return install.RootCheckResult{
		Authorized: true,
		Message:    "",
	}
}
