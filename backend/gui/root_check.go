package gui

import (
	"fmt"

	"aesthetic-os-installer/backend/internal/core"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// CheckRoot verifies that the aestheticos-helper can be executed via sudo without a password.
// This is called by the GUI to ensure the user has properly configured sudo
// before allowing installation to proceed.
func (a *App) CheckRoot() core.RootCheckResult {
	helper := core.FindHelper()

	wailsRuntime.LogInfo(a.ctx, "Checking root privileges with helper: "+helper)
	result := core.CheckRoot(helper)

	if result.Authorized {
		wailsRuntime.LogInfo(a.ctx, "Root check passed: sudo can run helper without password")
	} else {
		wailsRuntime.LogWarning(a.ctx, fmt.Sprintf("Root check failed: %s", result.Message))
	}
	
	return result
}


