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

// Minimum disk size: 8 GiB (8 × 1024³ bytes). Arch Linux requires at least ~4GB.
const minDiskSizeBytes = 8 * 1024 * 1024 * 1024

// ValidateDiskSelection checks that a disk is selected, meets minimum size,
// and has a valid filesystem type.
// Layout is auto-determined based on filesystem (ext4→default, btrfs→btrfs-subvolumes).
// Shared by GUI and console installer.
func ValidateDiskSelection(data DiskSelectionData) error {
	if data.DiskID == "" {
		return fmt.Errorf("Please select a disk to install to")
	}
	var sizeBytes uint64
	fmt.Sscanf(data.Size, "%d", &sizeBytes)
	if sizeBytes < minDiskSizeBytes {
		return fmt.Errorf("Selected disk is too small. Minimum 8 GB required")
	}
	if data.FileSystem == "" {
		return fmt.Errorf("Please select a filesystem")
	}
	if data.FileSystem != "ext4" && data.FileSystem != "btrfs" {
		return fmt.Errorf("Unsupported filesystem: %s (supported: ext4, btrfs)", data.FileSystem)
	}
	return nil
}
