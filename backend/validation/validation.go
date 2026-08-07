// Package validation provides centralized validation logic shared by the GUI
// and CLI. It delegates to backend/shared for disk/user validation and adds
// system-level validators.
package validation

import (
	"fmt"
	"os"

	"aestheticos-installer/backend/shared"
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

// ValidateMountedSystem checks that the target system is properly mounted
// at shared.MountRoot and contains a valid root filesystem.
func ValidateMountedSystem() error {
	mountRoot := shared.MountRoot
	if _, err := os.Stat(mountRoot); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("mount point %s does not exist", mountRoot)
		}
		return fmt.Errorf("cannot access mount point %s: %w", mountRoot, err)
	}
	return nil
}
