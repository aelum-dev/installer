package gui

import (
	"aestheticos-installer/backend/internal/core"
)

// CheckRoot verifies that the aestheticos-helper can be executed via sudo without a password.
// This is called by the GUI to ensure the user has properly configured sudo
// before allowing installation to proceed.
func (a *App) CheckRoot() core.RootCheckResult {
	helper := core.FindHelper()

	logger.Info("Checking root privileges with helper: %s", helper)
	result := core.CheckRoot(helper)

	if result.Authorized {
		logger.Info("Root check passed: sudo can run helper without password")
	} else {
		logger.Error("Root check failed: %s", result.Message)
	}

	return result
}
