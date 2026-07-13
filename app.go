package main

import (
	"context"
	"fmt"
)

// PageID represents a page in the installer wizard.
type PageID int

const (
	PageWelcome       PageID = 0
	PageUserAccount   PageID = 1
	PageDiskSelection PageID = 2
	PageInstallation  PageID = 3
	PageFinish        PageID = 4
)

// UserAccountData holds user account form fields.
type UserAccountData struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DiskSelectionData holds disk selection info.
type DiskSelectionData struct {
	DiskID string `json:"diskId"`
}

// App struct
type App struct {
	ctx context.Context
	currentPage PageID
	userAccount UserAccountData
	diskSelection DiskSelectionData
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		currentPage: PageWelcome,
	}
}

// startup is called when the app starts. The context is saved
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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

// CanGoNext checks if the user can proceed to the next page.
// TODO: Re-enable validation once UI forms are implemented.
func (a *App) CanGoNext() bool {
	return a.currentPage < PageFinish
}

// CanGoBack checks if the user can go back to the previous page.
func (a *App) CanGoBack() bool {
	return a.currentPage > PageWelcome
}

// ValidateUserAccount saves and validates user account data.
func (a *App) ValidateUserAccount(data UserAccountData) error {
	err := a.validateUserAccount(data)
	if err == nil {
		a.userAccount = data
	}
	return err
}

// ValidateDiskSelection saves and validates disk selection.
func (a *App) ValidateDiskSelection(data DiskSelectionData) error {
	err := a.validateDiskSelection(data)
	if err == nil {
		a.diskSelection = data
	}
	return err
}

// validateUserAccount checks user account fields.
func (a *App) validateUserAccount(data UserAccountData) error {
	if data.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(data.Username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if data.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(data.Password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	return nil
}

// validateDiskSelection checks that a disk is selected.
func (a *App) validateDiskSelection(data DiskSelectionData) error {
	if data.DiskID == "" {
		return fmt.Errorf("please select a disk to install to")
	}
	return nil
}
