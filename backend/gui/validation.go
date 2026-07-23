package gui

import (
	"aestheticos-installer/backend/internal/shared"
)

// --- GUI methods (validate only, no state mutation) ---

// ValidateUserAccount validates user account data.
func (a *App) ValidateUserAccount(data shared.UserAccountData) error {
	return shared.ValidateUserAccount(data)
}

// ValidateDiskSelection validates disk selection data.
func (a *App) ValidateDiskSelection(data shared.DiskSelectionData) error {
	return shared.ValidateDiskSelection(data)
}

// --- GUI methods (state mutation) ---

// SaveUserAccount saves user account data to app state.
func (a *App) SaveUserAccount(data shared.UserAccountData) {
	a.userAccount = data
}

// SaveDiskSelection saves disk selection data to app state and syncs filesystem.
func (a *App) SaveDiskSelection(data shared.DiskSelectionData) {
	a.diskSelection = data
	a.fileSystem = data.FileSystem
}
