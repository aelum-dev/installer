package gui

import (
	"aesthetic-os-installer/backend/shared"
)

// --- GUI methods (delegate to shared + save state) ---

// ValidateUserAccount saves and validates user account data.
func (a *App) ValidateUserAccount(data shared.UserAccountData) error {
	err := shared.ValidateUserAccount(data)
	if err == nil {
		a.userAccount = data
	}
	return err
}

// ValidateDiskSelection saves and validates disk selection.
func (a *App) ValidateDiskSelection(data shared.DiskSelectionData) error {
	err := shared.ValidateDiskSelection(data)
	if err == nil {
		a.diskSelection = data
	}
	return err
}
