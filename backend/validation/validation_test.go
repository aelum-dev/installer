package validation

import (
	"testing"

	"aelum-installer/backend/shared"
)

// The validation package is a thin, centralized façade over shared. These tests
// assert the delegation contract (same accept/reject behavior) so GUI/CLI call
// sites keep a single entry point.

func TestValidateUserAccount_DelegatesToShared(t *testing.T) {
	valid := shared.UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"}
	if err := ValidateUserAccount(valid); err != nil {
		t.Errorf("valid data returned error: %v", err)
	}
	if err := ValidateUserAccount(shared.UserAccountData{Username: ""}); err == nil {
		t.Error("expected an error for an empty username")
	}
}

func TestValidateDiskSelection_DelegatesToShared(t *testing.T) {
	valid := shared.DiskSelectionData{DiskID: "/dev/sda", Size: shared.MinDiskSizeBytes, FileSystem: shared.FileSystemExt4}
	if err := ValidateDiskSelection(valid); err != nil {
		t.Errorf("valid data returned error: %v", err)
	}
	if err := ValidateDiskSelection(shared.DiskSelectionData{}); err == nil {
		t.Error("expected an error for an empty selection")
	}
}

func TestValidateDiskPath_DelegatesToShared(t *testing.T) {
	if err := ValidateDiskPath("/dev/sda"); err != nil {
		t.Errorf("valid path returned error: %v", err)
	}
	if err := ValidateDiskPath("sda"); err == nil {
		t.Error("expected an error for a relative path")
	}
}

// ValidateDiskSize shells out to lsblk; a nonexistent device must surface an
// error rather than panic.
func TestValidateDiskSize_UnknownDevice(t *testing.T) {
	if err := ValidateDiskSize("/dev/aelum-nonexistent-device"); err == nil {
		t.Error("expected an error for a nonexistent device")
	}
}
