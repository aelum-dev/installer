package app

import "context"

// Language represents a selectable language.
type Language struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

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

// UserAccountData holds user account form fields.
type UserAccountData struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// DiskSelectionData holds disk selection info.
type DiskSelectionData struct {
	DiskID string `json:"diskId"`
}

// App struct
type App struct {
	ctx             context.Context
	currentPage     PageID
	startPage       PageID
	dryRun          bool
	userAccount     UserAccountData
	diskSelection   DiskSelectionData
	selectedLanguage string
}
