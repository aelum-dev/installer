package gui

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"aestheticos-installer/backend/install"
)

// NewApp creates a new App application struct
func NewApp(dryRun bool) *App {
	return &App{currentPage: PageWelcome, dryRun: dryRun}
}

// StartUp is called when the app starts. The context is saved.
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
// If an installation is in progress, sends SIGINT to the CLI and waits for
// cleanup before allowing the window to close. Then unmounts any leftover
// mounts under /mnt via sudo.
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.installationRunning.Load() {
		a.CancelInstallation()
		a.WaitForCleanup()
		install.GetLogger().Close()
	}

	// GUI runs as a normal user, so spawn the CLI via sudo for root privileges.
	cliBinary := findCLIBinary()
	cmd := exec.Command("sudo", cliBinary, "--cleanup")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

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
