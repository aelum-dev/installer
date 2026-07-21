package gui

import (
	"fmt"

	"aesthetic-os-installer/backend/core"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// CheckRoot verifies that the aestheticos-helper can be executed via sudo without a password.
// This is called by the GUI to ensure the user has properly configured sudo
// before allowing installation to proceed.
func (a *App) CheckRoot() core.RootCheckResult {
	helper := core.FindHelper()
	
	fmt.Printf("[DEBUG GUI] CheckRoot: FindHelper() returned: %s\n", helper)
	wailsRuntime.LogInfo(a.ctx, fmt.Sprintf("Checking root privileges with helper: %s", helper))
	result := core.CheckRoot(helper)
	
	fmt.Printf("[DEBUG GUI] CheckRoot: result authorized=%v, error=%s, message=%s\n", result.Authorized, result.Error, result.Message)
	
	if result.Authorized {
		wailsRuntime.LogInfo(a.ctx, "Root check passed: sudo can run helper without password")
	} else {
		wailsRuntime.LogWarning(a.ctx, fmt.Sprintf("Root check failed: %s", result.Message))
	}
	
	return result
}


