package app

import (
	"encoding/json"
	"testing"
)

func TestJsonSize_UnmarshalJSON_String(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"quoted string", `"500.0 GB"`, "500.0 GB"},
		{"zero bytes", `"0"`, "0"},
		{"empty string", `""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var js jsonSize
			err := json.Unmarshal([]byte(tt.input), &js)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if js.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, js.Value)
			}
		})
	}
}

func TestJsonSize_UnmarshalJSON_Number(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"bytes value", `1024`, "1024"},
		{"large value", `500107862016`, "500107862016"},
		{"zero", `0`, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var js jsonSize
			err := json.Unmarshal([]byte(tt.input), &js)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if js.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, js.Value)
			}
		})
	}
}

func TestJsonBool_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"true boolean", `true`, true},
		{"false boolean", `false`, false},
		{"string 1", `"1"`, true},
		{"string 0", `"0"`, false},
		{"null", `null`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jb jsonBool
			err := json.Unmarshal([]byte(tt.input), &jb)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bool(jb) != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, bool(jb))
			}
		})
	}
}

func TestLsblkEntry_UnmarshalJSON(t *testing.T) {
	input := `{
		"name": "sda",
		"type": "disk",
		"size": 1024209543168,
		"model": "TestDisk",
		"vendor": null,
		"rota": false,
		"ro": false,
		"fstype": null,
		"mountpoint": null,
		"serial": "ABC123",
		"tran": "sata",
		"children": []
	}`

	var entry lsblkEntry
	err := json.Unmarshal([]byte(input), &entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.Name != "sda" {
		t.Errorf("expected name 'sda', got %q", entry.Name)
	}
	if entry.Size.Value != "1024209543168" {
		t.Errorf("expected size '1024209543168', got %q", entry.Size.Value)
	}
	if entry.Model != "TestDisk" {
		t.Errorf("expected model 'TestDisk', got %q", entry.Model)
	}
	if bool(entry.Rota) != false {
		t.Errorf("expected rota false, got %v", bool(entry.Rota))
	}
	if bool(entry.Ro) != false {
		t.Errorf("expected ro false, got %v", bool(entry.Ro))
	}
}

func TestLsblkEntry_UnmarshalJSON_StringSize(t *testing.T) {
	input := `{
		"name": "sda",
		"type": "disk",
		"size": "465.8 GiB",
		"model": "TestDisk",
		"vendor": null,
		"rota": "1",
		"ro": "0",
		"fstype": null,
		"mountpoint": null,
		"serial": "ABC123",
		"tran": "usb",
		"children": []
	}`

	var entry lsblkEntry
	err := json.Unmarshal([]byte(input), &entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.Size.Value != "465.8 GiB" {
		t.Errorf("expected size '465.8 GiB', got %q", entry.Size.Value)
	}
	if bool(entry.Rota) != true {
		t.Errorf("expected rota true, got %v", bool(entry.Rota))
	}
	if bool(entry.Ro) != false {
		t.Errorf("expected ro false, got %v", bool(entry.Ro))
	}
}

func TestValidateDiskSelection(t *testing.T) {
	app := NewApp(false)

	tests := []struct {
		name        string
		data        DiskSelectionData
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid disk selection",
			data:        DiskSelectionData{DiskID: "/dev/sda"},
			expectError: false,
		},
		{
			name:        "valid nvme disk",
			data:        DiskSelectionData{DiskID: "/dev/nvme0n1"},
			expectError: false,
		},
		{
			name:        "empty disk id",
			data:        DiskSelectionData{DiskID: ""},
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
	data := DiskSelectionData{DiskID: "/dev/sda"}

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
	validData := DiskSelectionData{DiskID: "/dev/sda"}
	invalidData := DiskSelectionData{DiskID: ""}

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
