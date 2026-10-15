// Package validation provides centralized validation logic shared by the GUI
// and CLI. It delegates to backend/shared for disk/user validation and adds
// system-level validators.
package validation

import (
	"aelum-installer/backend/shared"
)

// ValidateUserAccount validates user account data.
func ValidateUserAccount(data shared.UserAccountData) error {
	return shared.ValidateUserAccount(data)
}

// ValidateDiskSelection validates disk selection data.
func ValidateDiskSelection(data shared.DiskSelectionData) error {
	return shared.ValidateDiskSelection(data)
}

// ValidateDiskPath checks that a disk path is valid and safe to use.
func ValidateDiskPath(path string) error {
	return shared.ValidateDiskPath(path)
}

// ValidateDiskSize checks that a disk meets the minimum size requirement.
func ValidateDiskSize(diskPath string) error {
	return shared.ValidateDiskSize(diskPath)
}
