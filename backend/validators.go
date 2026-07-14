package app

import "fmt"

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
		return fmt.Errorf("Username is required")
	}
	if len(data.Username) < 3 {
		return fmt.Errorf("Username must be at least 3 characters")
	}
	if data.Password == "" {
		return fmt.Errorf("Password is required")
	}
	if len(data.Password) < 6 {
		return fmt.Errorf("Password must be at least 6 characters")
	}
	if data.ConfirmPassword == "" {
		return fmt.Errorf("Please confirm your password")
	}
	if data.Password != data.ConfirmPassword {
		return fmt.Errorf("Passwords do not match")
	}
	return nil
}

// validateDiskSelection checks that a disk is selected.
func (a *App) validateDiskSelection(data DiskSelectionData) error {
	if data.DiskID == "" {
		return fmt.Errorf("Please select a disk to install to")
	}
	return nil
}
