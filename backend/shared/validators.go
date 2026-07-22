package shared

import "fmt"

// ValidateUserAccount checks user account fields.
// Password/confirm checks are skipped when password is empty,
// allowing callers to validate username before password is entered.
// Shared by GUI and console installer.
func ValidateUserAccount(data UserAccountData) error {
	if data.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(data.Username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if data.Password == "" {
		return nil
	}
	if len(data.Password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	if data.ConfirmPassword == "" {
		return fmt.Errorf("please confirm your password")
	}
	if data.Password != data.ConfirmPassword {
		return fmt.Errorf("passwords do not match")
	}
	return nil
}

// Minimum disk size: 8 GiB (8 × 1024³ bytes). Arch Linux requires at least ~4GB.
const minDiskSizeBytes = 8 * 1024 * 1024 * 1024

// ValidateDiskSelection checks that a disk is selected, meets minimum size,
// and has a valid filesystem type.
// Layout is auto-determined based on filesystem (ext4→ext4, btrfs→btrfs).
// Shared by GUI and console installer.
func ValidateDiskSelection(data DiskSelectionData) error {
	if data.DiskID == "" {
		return fmt.Errorf("please select a disk to install to")
	}
	var sizeBytes uint64
	fmt.Sscanf(data.Size, "%d", &sizeBytes)
	if sizeBytes < minDiskSizeBytes {
		return fmt.Errorf("selected disk is too small. minimum 8 GB required")
	}
	if data.FileSystem == "" {
		return fmt.Errorf("please select a filesystem")
	}
	if data.FileSystem != "ext4" && data.FileSystem != "btrfs" {
		return fmt.Errorf("unsupported filesystem: %s (supported: ext4, btrfs)", data.FileSystem)
	}
	return nil
}
