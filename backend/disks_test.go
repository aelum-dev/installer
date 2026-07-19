package app

import (
	"testing"

	"aesthetic-os-installer/backend/shared"
)

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
			data:        shared.DiskSelectionData{DiskID: "/dev/sda"},
			expectError: false,
		},
		{
			name:        "valid nvme disk",
			data:        shared.DiskSelectionData{DiskID: "/dev/nvme0n1"},
			expectError: false,
		},
		{
			name:        "empty disk id",
			data:        shared.DiskSelectionData{DiskID: ""},
			expectError: true,
			errorMsg:    "Please select a disk to install to",
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
	data := shared.DiskSelectionData{DiskID: "/dev/sda"}

	err := app.ValidateDiskSelection(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.diskSelection.DiskID != "/dev/sda" {
		t.Errorf("expected diskId to be saved as '/dev/sda', got %q", app.diskSelection.DiskID)
	}
}

func TestValidateDiskSelection_DoesNotSaveOnFailure(t *testing.T) {
	app := NewApp(false)
	validData := shared.DiskSelectionData{DiskID: "/dev/sda"}
	invalidData := shared.DiskSelectionData{DiskID: ""}

	// First, save valid data
	err := app.ValidateDiskSelection(validData)
	if err != nil {
		t.Fatalf("unexpected error on valid data: %v", err)
	}

	// Then try invalid data
	err = app.ValidateDiskSelection(invalidData)
	if err == nil {
		t.Fatal("expected error on invalid data")
	}

	// Original data should still be saved
	if app.diskSelection.DiskID != "/dev/sda" {
		t.Errorf("expected saved diskId to still be '/dev/sda', got %q", app.diskSelection.DiskID)
	}
}
