package gui

import (
	"testing"

	"aelum-installer/backend/shared"
)

func TestValidateUserAccount(t *testing.T) {
	app := NewApp(false)

	tests := []struct {
		name        string
		data        shared.UserAccountData
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid account data",
			data:        shared.UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"},
			expectError: false,
		},
		{
			name:        "valid minimum length username and password",
			data:        shared.UserAccountData{Username: "abc", Password: "abc123", ConfirmPassword: "abc123"},
			expectError: false,
		},
		{
			name:        "empty username",
			data:        shared.UserAccountData{Username: "", Password: "password123", ConfirmPassword: "password123"},
			expectError: true,
			errorMsg:    "username is required",
		},
		{
			name:        "username too short",
			data:        shared.UserAccountData{Username: "ab", Password: "password123", ConfirmPassword: "password123"},
			expectError: true,
			errorMsg:    "username must be at least 3 characters",
		},
		{
			name:        "empty password (skipped, waiting for input)",
			data:        shared.UserAccountData{Username: "john_doe", Password: "", ConfirmPassword: ""},
			expectError: false,
		},
		{
			name:        "password too short",
			data:        shared.UserAccountData{Username: "john_doe", Password: "abc12", ConfirmPassword: "abc12"},
			expectError: true,
			errorMsg:    "password must be at least 6 characters",
		},
		{
			name:        "empty confirm password",
			data:        shared.UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: ""},
			expectError: true,
			errorMsg:    "please confirm your password",
		},
		{
			name:        "passwords do not match",
			data:        shared.UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "different"},
			expectError: true,
			errorMsg:    "passwords do not match",
		},
		{
			name:        "all fields empty",
			data:        shared.UserAccountData{Username: "", Password: "", ConfirmPassword: ""},
			expectError: true,
			errorMsg:    "username is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.ValidateUserAccount(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tt.expectError && err != nil && err.Error() != tt.errorMsg {
				t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestValidateUserAccount_SavesOnSuccess(t *testing.T) {
	app := NewApp(false)
	data := shared.UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"}

	err := app.ValidateUserAccount(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Validate separates concerns — Save must be called explicitly
	app.SaveUserAccount(data)

	if app.userAccount.Username != "john_doe" {
		t.Errorf("expected username to be saved as 'john_doe', got %q", app.userAccount.Username)
	}
	if app.userAccount.Password != "password123" {
		t.Errorf("expected password to be saved, got %q", app.userAccount.Password)
	}
}

func TestValidateUserAccount_DoesNotSaveOnFailure(t *testing.T) {
	app := NewApp(false)
	validData := shared.UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"}
	invalidData := shared.UserAccountData{Username: "", Password: "", ConfirmPassword: ""}

	// First, validate and save valid data
	err := app.ValidateUserAccount(validData)
	if err != nil {
		t.Fatalf("unexpected error on valid data: %v", err)
	}
	app.SaveUserAccount(validData)

	// Then try invalid data — validation should fail
	err = app.ValidateUserAccount(invalidData)
	if err == nil {
		t.Fatal("expected error on invalid data")
	}

	// Saved data should still be intact (validation doesn't clear state)
	if app.userAccount.Username != "john_doe" {
		t.Errorf("expected saved username to still be 'john_doe', got %q", app.userAccount.Username)
	}
}

func TestValidateDiskSelection(t *testing.T) {
	app := NewApp(false)

	tests := []struct {
		name        string
		data        shared.DiskSelectionData
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid disk selection",
			data:        shared.DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000", FileSystem: "ext4"},
			expectError: false,
		},
		{
			name:        "valid nvme disk",
			data:        shared.DiskSelectionData{DiskID: "/dev/nvme0n1", Size: "50000000000", FileSystem: "btrfs"},
			expectError: false,
		},
		{
			name:        "empty disk id",
			data:        shared.DiskSelectionData{DiskID: ""},
			expectError: true,
			errorMsg:    "please select a disk to install to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.ValidateDiskSelection(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tt.expectError && err != nil && err.Error() != tt.errorMsg {
				t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestValidateDiskSelection_SavesOnSuccess(t *testing.T) {
	app := NewApp(false)
	data := shared.DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000", FileSystem: "ext4"}

	err := app.ValidateDiskSelection(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Validate separates concerns — Save must be called explicitly
	app.SaveDiskSelection(data)

	if app.diskSelection.DiskID != "/dev/sda" {
		t.Errorf("expected diskId to be saved as '/dev/sda', got %q", app.diskSelection.DiskID)
	}
	if app.diskSelection.FileSystem != "ext4" {
		t.Errorf("expected FileSystem to be saved as 'ext4', got %q", app.diskSelection.FileSystem)
	}
	if app.fileSystem != "ext4" {
		t.Errorf("expected fileSystem to be saved as 'ext4', got %q", app.fileSystem)
	}
}

func TestValidateDiskSelection_DoesNotSaveOnFailure(t *testing.T) {
	app := NewApp(false)
	validData := shared.DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000", FileSystem: "ext4"}
	invalidData := shared.DiskSelectionData{DiskID: ""}

	// First, validate and save valid data
	err := app.ValidateDiskSelection(validData)
	if err != nil {
		t.Fatalf("unexpected error on valid data: %v", err)
	}
	app.SaveDiskSelection(validData)

	// Then try invalid data — validation should fail
	err = app.ValidateDiskSelection(invalidData)
	if err == nil {
		t.Fatal("expected error on invalid data")
	}

	// Saved data should still be intact (validation doesn't clear state)
	if app.diskSelection.DiskID != "/dev/sda" {
		t.Errorf("expected saved diskId to still be '/dev/sda', got %q", app.diskSelection.DiskID)
	}
	if app.fileSystem != "ext4" {
		t.Errorf("expected saved fileSystem to still be 'ext4', got %q", app.fileSystem)
	}
}
