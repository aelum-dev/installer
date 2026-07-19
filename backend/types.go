package app

import (
	"context"

	"aesthetic-os-installer/backend/shared"
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
	ctx             context.Context
	currentPage     PageID
	startPage       PageID
	dryRun          bool
	userAccount     shared.UserAccountData
	diskSelection   shared.DiskSelectionData
	selectedLanguage string
}
