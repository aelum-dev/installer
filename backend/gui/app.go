package gui

import (
	"context"
	"fmt"
	"os/exec"

	"aestheticos-installer/backend/internal/core"
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
// Returns false to allow the window to close. If an installation is in progress,
// triggers the installer's sync.Once cleanup so mounts are torn down before exit.
// The installer's defer will also call runCleanup, but sync.Once ensures it
// only runs once — whichever path gets there first wins.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.installationRunning.Load() && a.installer != nil {
		a.installer.RunCleanup()
	}
	return false
}

// GetInstallationRunning reports whether an installation is currently in progress.
func (a *App) GetInstallationRunning() bool {
	return a.installationRunning.Load()
}

// GetInstaller returns the currently running installer, or nil if none.
func (a *App) GetInstaller() *core.Installer {
	return a.installer
}

// GetDiskID returns the currently selected disk ID.
func (a *App) GetDiskID() string {
	return a.diskSelection.DiskID
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
