package gui

import (
	"context"
	"fmt"
	"os/exec"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"aestheticos-installer/backend/install"
	"aestheticos-installer/backend/shared"
)

// NewApp creates a new App application struct
func NewApp(dryRun bool) *App {
	return &App{currentPage: PageWelcome, dryRun: dryRun}
}

// StartUp is called when the app starts. The context is saved and root
// privileges are checked. If sudo is not available, dry-run mode is enabled
// automatically so the user can still explore the installer. Fake disks are
// also loaded in dry-run mode since lsblk requires no special privileges but
// the user might not have permission to read /dev blocks.
func (a *App) StartUp(ctx context.Context) {
	a.ctx = ctx

	// Check root privileges early. If not available, enable dry-run mode
	// so the user can still use the installer without real side effects.
	result := a.CheckRoot()
	if !result.Authorized {
		install.GetLogger().Info("No sudo access — enabling dry-run mode")
		a.dryRun = true
	}

	// Enable fake disks in dry-run mode so the user can explore the UI
	// without needing to read real block devices.
	if a.dryRun {
		shared.SetFakeDisks(true)
	}
}

// SetStartPage sets the page to start on (bypasses welcome page).
func (a *App) SetStartPage(pageID PageID) error {
	if pageID.Value < PageWelcome.Value || pageID.Value > PageFinish.Value {
		return fmt.Errorf("invalid start page ID: %d", pageID.Value)
	}
	a.startPage = pageID
	return nil
}

// GetStartPage returns the configured start page, or -1 if none.
func (a *App) GetStartPage() int {
	if a.startPage == PageWelcome {
		return -1
	}
	return a.startPage.Value
}

// ClearStartPage clears the start page so it won't be used again.
func (a *App) ClearStartPage() {
	a.startPage = PageWelcome
}

// GetDryRun returns whether the app is running in dry-run mode.
func (a *App) GetDryRun() bool {
	return a.dryRun
}

// OnBeforeClose is called by Wails when the user attempts to close the window.
// If an installation is in progress, sends SIGINT to the CLI and waits for
// cleanup before allowing the window to close. Skipped in dry-run mode.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.installationRunning.Load() {
		a.CancelInstallation()
		a.WaitForCleanup()
		install.GetLogger().Close()
	}

	return false
}

// GetInstallationRunning reports whether an installation is currently in progress.
func (a *App) GetInstallationRunning() bool {
	return a.installationRunning.Load()
}

// GetDiskID returns the currently selected disk ID.
func (a *App) GetDiskID() string {
	return a.diskSelection.DiskID
}

// RebootAndClose reboots the system. If an installation is in progress, waits
// for cleanup to finish first. In dry-run mode, only logs the action.
func (a *App) RebootAndClose() error {
	if a.dryRun {
		install.GetLogger().Info("Rebooting...")
		wailsRuntime.Quit(a.ctx)
		return nil
	}

	if a.installationRunning.Load() {
		a.CancelInstallation()
		a.WaitForCleanup()
	}

	cmd := exec.Command("sudo", "reboot")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reboot: %w", err)
	}

	install.GetLogger().Info("Rebooting...")
	return nil
}
