package gui

import (
	"aelum-installer/backend/shared"
	"aelum-installer/backend/validation"
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

// ResetDiskSelection clears every answer the disk page owns: the selected disk,
// its filesystem, the encryption and LVM switches (including the held LUKS
// passphrase), and manual partitioning with its designed plan. The frontend
// calls it when leaving the disk page backwards — those answers belong to that
// visit. Returning from the Confirmation page must NOT call it: the person only
// stepped back to read the summary, and the wizard keeps their whole selection.
// Wails binding.
func (a *App) ResetDiskSelection() {
	a.diskSelection = shared.DiskSelectionData{}
	a.fileSystem = shared.DefaultFileSystem
	// SetLuks(false) zeroes the held passphrase; never clear the flag by hand.
	a.SetLuks(false)
	a.SetLvm(false)
	a.SetManualMode(false)
	a.manualLayout = shared.ManualLayout{}
}
