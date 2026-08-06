package gui

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"aestheticos-installer/backend/internal/core"
	"aestheticos-installer/backend/internal/install"
)

// NewApp creates a new App application struct
func NewApp(dryRun bool) *App {
	return &App{
		currentPage: PageWelcome,
		dryRun:      dryRun,
	}
}

// StartUp is called when the app starts. The context is saved
func (a *App) StartUp(ctx context.Context) {
	a.ctx = ctx
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
// If an installation is in progress, it runs cleanup before allowing the window to close.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.installationRunning.Load() {
		helper := core.FindHelper()
		if helper != "" && a.diskSelection.DiskID != "" {
			if err := install.CleanupMounts(helper, logger, a.diskSelection.DiskID); err != nil {
				fmt.Fprintf(os.Stderr, "Cleanup warning: %v\n", err)
			}
		}
	}
	return true
}

// RebootAndClose initiates a system reboot and closes the application window.
// In dry-run mode, it only logs the action without rebooting.
func (a *App) RebootAndClose() error {
	if a.dryRun {
		logger.Info("Rebooting...")
		return nil
	}

	// Use aestheticos-helper to reboot (requires root via sudo)
	helper := core.FindHelper()
	cmd := exec.Command("sudo", helper, "run", "reboot")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reboot: %w", err)
	}

	logger.Info("Rebooting...")
	return nil
}
