package shared

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Username rules mirror shadow-utils 4.20's is_valid_name()/is_valid_user_name()
// (used by useradd when NAME_REGEX is unset, as on Arch). Linux accepts:
//   - 1..255 characters (LOGIN_NAME_MAX is 256 including the NUL terminator)
//   - first character: letter, digit, underscore, or dot
//   - remaining characters: letters, digits, underscore, dot, or hyphen
//   - an optional trailing '$' (Samba machine accounts)
//   - but not ".", "..", a leading '-', or a name made up only of digits.
//
// Anything meeting those rules is valid here too; the installer does not add
// extra character-case or punctuation policy.
var (
	UsernameFirstCharRegex    = regexp.MustCompile(`^[A-Za-z0-9_.]`)
	UsernameAllowedCharsRegex = regexp.MustCompile(`^[A-Za-z0-9_.-]*\$?$`)
	UsernameAllDigitsRegex    = regexp.MustCompile(`^[0-9]+$`)
)

// reservedAccountNames is the set of target users/groups that would collide
// with a new account (see shared.ReservedAccountNames).
var reservedAccountNames = func() map[string]bool {
	set := make(map[string]bool)
	for _, name := range strings.Split(ReservedAccountNames, ",") {
		if name != "" {
			set[name] = true
		}
	}
	return set
}()

// ValidateUserAccount checks user account fields using Linux's own rules.
// Password/confirm checks are skipped when password is empty, allowing callers
// to validate the username before the password is entered.
// Shared by GUI and console installer.
func ValidateUserAccount(data UserAccountData) error {
	if data.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(data.Username) > MaxUsernameLength {
		return fmt.Errorf("username must be at most %d characters", MaxUsernameLength)
	}
	if data.Username == "." || data.Username == ".." {
		return fmt.Errorf("username cannot be %q", data.Username)
	}
	if UsernameAllDigitsRegex.MatchString(data.Username) {
		return fmt.Errorf("username cannot consist only of digits")
	}
	if !UsernameFirstCharRegex.MatchString(data.Username) {
		return fmt.Errorf("username must start with a letter, number, underscore, or dot")
	}
	if !UsernameAllowedCharsRegex.MatchString(data.Username) {
		return fmt.Errorf("username may only contain letters, numbers, dots, hyphens, and underscores (with an optional trailing '$')")
	}
	if reservedAccountNames[data.Username] {
		return fmt.Errorf("username %q is already used by a system account or group", data.Username)
	}
	if data.Password == "" {
		return nil
	}
	if strings.ContainsAny(data.Password, "\n\r\x00") {
		return fmt.Errorf("password cannot contain newlines or NUL characters")
	}
	if len(data.Password) > MaxPasswordLength {
		return fmt.Errorf("password must be at most %d UTF-8 bytes", MaxPasswordLength)
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
		return fmt.Errorf("disk %s is too small: %s (minimum %s required)", diskPath, FormatSize(int64(sizeBytes)), FormatSize(MinDiskSizeBytes))
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
	if data.Size < MinDiskSizeBytes {
		return fmt.Errorf("selected disk is too small. minimum %s required", FormatSize(MinDiskSizeBytes))
	}
	if data.FileSystem == "" {
		return fmt.Errorf("please select a filesystem")
	}
	if data.FileSystem != FileSystemExt4 && data.FileSystem != FileSystemBtrfs {
		return fmt.Errorf("unsupported filesystem: %s (supported: ext4, btrfs)", data.FileSystem)
	}
	return nil
}
