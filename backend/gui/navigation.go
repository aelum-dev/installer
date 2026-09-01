package gui

import (
	"context"
	"fmt"
	"os/exec"
	"sync/atomic"

	"aelum-installer/backend/shared"
)

// PageID represents a page in the installer wizard.
// Defined as a struct so Wails generates proper TypeScript model bindings.
type PageID struct {
	Value int
}

var (
	PageWelcome       = PageID{Value: 0}
	PageUserAccount   = PageID{Value: 1}
	PageDiskSelection = PageID{Value: 2}
	PageConfirmation  = PageID{Value: 3}
	PageInstallation  = PageID{Value: 4}
	PageFinish        = PageID{Value: 5}
)

// App struct
type App struct {
	ctx                 context.Context
	currentPage         PageID
	startPage           PageID
	dryRun              bool
	userAccount         shared.UserAccountData
	diskSelection       shared.DiskSelectionData
	hostname            string
	selectedLanguage    string
	region              shared.Region
	uiLocale            string // UI language (separate from installed system locale)
	fileSystem          string // root filesystem: "ext4" or "btrfs"
	manualMode          bool   // dualboot-safe manual partitioning enabled
	manualLayout        shared.ManualLayout
	installationRunning atomic.Bool

	// installCmd is the currently running CLI subprocess, or nil.
	installCmd *exec.Cmd
	// installCancel cancels the context for the current CLI subprocess.
	installCancel func()
	// installDone is closed when the CLI process has exited and cleanup is complete.
	installDone chan struct{}
}

// GetCurrentPage returns the current page index (0-based).
func (a *App) GetCurrentPage() int {
	return a.currentPage.Value
}

// SetCurrentPage sets the current page after validation.
func (a *App) SetCurrentPage(pageID PageID) error {
	if pageID.Value < PageWelcome.Value || pageID.Value > PageFinish.Value {
		return fmt.Errorf("invalid page ID: %d", pageID.Value)
	}
	a.currentPage = pageID
	return nil
}

// SetHostname sets the system hostname.
func (a *App) SetHostname(hostname string) {
	a.hostname = hostname
}

// GetHostname returns the system hostname.
func (a *App) GetHostname() string {
	return a.hostname
}

// GetUILocale returns the current UI locale.
func (a *App) GetUILocale() string {
	return a.uiLocale
}

// SetUILocale sets the UI locale.
func (a *App) SetUILocale(locale string) {
	a.uiLocale = locale
}
