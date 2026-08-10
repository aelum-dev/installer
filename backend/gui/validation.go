package gui

import (
	"aestheticos-installer/backend/shared"
	"aestheticos-installer/backend/validation"
)

// ValidateUserAccount validates user account data (no state mutation).
func (a *App) ValidateUserAccount(data shared.UserAccountData) error {
	return validation.ValidateUserAccount(data)
}

// ValidateDiskSelection validates disk selection data (no state mutation).
func (a *App) ValidateDiskSelection(data shared.DiskSelectionData) error {
	return validation.ValidateDiskSelection(data)
}

// SaveUserAccount saves user account data to app state.
func (a *App) SaveUserAccount(data shared.UserAccountData) {
	a.userAccount = data
}

// SaveDiskSelection saves disk selection data to app state and syncs filesystem.
func (a *App) SaveDiskSelection(data shared.DiskSelectionData) {
	a.diskSelection = data
	a.fileSystem = data.FileSystem
}
