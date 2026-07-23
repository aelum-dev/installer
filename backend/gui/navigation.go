package gui

import (
	"context"
	"fmt"

	"aestheticos-installer/backend/internal/shared"
)

// PageID represents a page in the installer wizard.
type PageID int

const (
	PageWelcome       PageID = 0
	PageUserAccount   PageID = 1
	PageDiskSelection PageID = 2
	PageConfirmation  PageID = 3
	PageInstallation  PageID = 4
	PageFinish        PageID = 5
)

// App struct
type App struct {
	ctx              context.Context
	currentPage      PageID
	startPage        PageID
	dryRun           bool
	userAccount      shared.UserAccountData
	diskSelection    shared.DiskSelectionData
	hostname         string
	selectedLanguage string
	region           shared.Region
	fileSystem       string // root filesystem: "ext4" or "btrfs"
}

// GetCurrentPage returns the current page index (0-based).
func (a *App) GetCurrentPage() int {
	return int(a.currentPage)
}

// SetCurrentPage sets the current page after validation.
func (a *App) SetCurrentPage(pageID PageID) error {
	if pageID < PageWelcome || pageID > PageFinish {
		return fmt.Errorf("invalid page ID: %d", pageID)
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
