package gui

import (
	"os/exec"

	"aelum-installer/backend/install"
)

// CheckRoot verifies that passwordless sudo is available.
func (a *App) CheckRoot() install.RootCheckResult {
	install.GetLogger().Info("Checking sudo availability...")
	if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
		return install.RootCheckResult{
			Authorized: false,
			Message:    "The installer needs sudo access. Please add your user to the sudoers file.",
		}
	}
	install.GetLogger().Info("Root check passed: sudo is available")
	return install.RootCheckResult{Authorized: true}
}
