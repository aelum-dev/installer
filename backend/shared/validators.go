package shared

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// UsernameFirstCharRegex validates the first character of a Linux login name.
// Must be a lowercase letter or underscore.
var UsernameFirstCharRegex = regexp.MustCompile(`^[a-z_]`)

// UsernameAllowedCharsRegex validates all allowed characters in a Linux login name.
// Lowercase letters, digits, underscores, and hyphens only.
var UsernameAllowedCharsRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ValidateUserAccount checks user account fields.
// Password/confirm checks are skipped when password is empty,
// allowing callers to validate username before password is entered.
// Shared by GUI and console installer.
func ValidateUserAccount(data UserAccountData) error {
	if data.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(data.Username) < MinUsernameLength {
		return fmt.Errorf("username must be at least %d characters", MinUsernameLength)
	}
	if len(data.Username) > MaxUsernameLength {
		return fmt.Errorf("username must be at most %d characters", MaxUsernameLength)
	}
	if !UsernameFirstCharRegex.MatchString(data.Username) {
		return fmt.Errorf("username must start with a lowercase letter or underscore")
	}
	if !UsernameAllowedCharsRegex.MatchString(data.Username) {
		return fmt.Errorf("username must contain only lowercase letters, numbers, hyphens, and underscores")
	}
	if ReservedUsernames[data.Username] {
		return fmt.Errorf("username %q is reserved", data.Username)
	}
	if data.Password == "" {
		return nil
	}
	if len(data.Password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if data.ConfirmPassword == "" {
		return fmt.Errorf("please confirm your password")
	}
	if data.Password != data.ConfirmPassword {
		return fmt.Errorf("passwords do not match")
	}
	return nil
}

// ValidateDiskPath checks that a disk path is valid and safe to use.
// It must start with /dev/ and contain only alphanumeric characters, slashes,
// hyphens, and underscores (no shell metacharacters).
func ValidateDiskPath(path string) error {
	if path == "" {
		return fmt.Errorf("disk path is required")
	}
	if !regexp.MustCompile(`^/dev/[a-zA-Z0-9_-]+$`).MatchString(path) {
		return fmt.Errorf("invalid disk path: %q (must be /dev/ followed by alphanumeric characters, hyphens, or underscores)", path)
	}
	return nil
}

// ValidateDiskSize checks that a disk meets the minimum size requirement.
// It queries lsblk to get the disk size in bytes and compares against MinDiskSizeBytes.
func ValidateDiskSize(diskPath string) error {
	cmd := exec.Command("lsblk", "-b", "-d", "-n", "-o", "SIZE", diskPath)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get disk size for %s: %w", diskPath, err)
	}
	sizeStr := strings.TrimSpace(string(output))
	sizeBytes, err := strconv.ParseUint(sizeStr, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse disk size %q: %w", sizeStr, err)
	}
	if sizeBytes < MinDiskSizeBytes {
		return fmt.Errorf("disk %s is too small: %s (minimum 8 GB required)", diskPath, FormatSize(sizeStr))
	}
	return nil
}

// ValidateDiskSelection checks that a disk is selected, meets minimum size,
// and has a valid filesystem type.
// Layout is auto-determined based on filesystem (ext4→ext4, btrfs→btrfs).
// Shared by GUI and console installer.
func ValidateDiskSelection(data DiskSelectionData) error {
	if data.DiskID == "" {
		return fmt.Errorf("please select a disk to install to")
	}
	var sizeBytes uint64
	fmt.Sscanf(data.Size, "%d", &sizeBytes) //nolint:errcheck // format is validated before calling
	if sizeBytes < MinDiskSizeBytes {
		return fmt.Errorf("selected disk is too small. minimum 8 GiB required")
	}
	if data.FileSystem == "" {
		return fmt.Errorf("please select a filesystem")
	}
	if data.FileSystem != FileSystemExt4 && data.FileSystem != FileSystemBtrfs {
		return fmt.Errorf("unsupported filesystem: %s (supported: ext4, btrfs)", data.FileSystem)
	}
	return nil
}
