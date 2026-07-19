package shared

import "fmt"

// ValidateUserAccount checks user account fields.
// Password/confirm checks are skipped when password is empty,
// allowing callers to validate username before password is entered.
// Shared by GUI and console installer.
func ValidateUserAccount(data UserAccountData) error {
	if data.Username == "" {
		return fmt.Errorf("Username is required")
	}
	if len(data.Username) < 3 {
		return fmt.Errorf("Username must be at least 3 characters")
	}
	if data.Password == "" {
		return nil
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

// ValidateDiskSelection checks that a disk is selected.
// Shared by GUI and console installer.
func ValidateDiskSelection(data DiskSelectionData) error {
	if data.DiskID == "" {
		return fmt.Errorf("Please select a disk to install to")
	}
	return nil
}
