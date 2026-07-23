package gui

import (
	"context"
	"fmt"
	"os/exec"

	"aestheticos-installer/backend/internal/core"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// RebootAndClose initiates a system reboot and closes the application window.
// In dry-run mode, it only logs the action without rebooting.
func (a *App) RebootAndClose() error {
	if a.dryRun {
		wailsRuntime.LogInfo(a.ctx, "Rebooting...")
		wailsRuntime.Quit(a.ctx)
		return nil
	}

	// Use aestheticos-helper to reboot (requires root via sudo)
	helper := core.FindHelper()
	cmd := exec.Command("sudo", helper, "reboot")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reboot: %w", err)
	}

	// Close the application window after initiating reboot
	wailsRuntime.Quit(a.ctx)
	return nil
}
