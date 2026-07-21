package shared

import "testing"

func TestValidateUserAccount_UsernameOnly(t *testing.T) {
	// Console installer validates username before password is entered.
	// Password checks must be skipped when password is empty.
	tests := []struct {
		name      string
		data      UserAccountData
		expectErr bool
	}{
		{
			name:      "valid username, no password yet",
			data:      UserAccountData{Username: "john_doe"},
			expectErr: false,
		},
		{
			name:      "empty username with no password",
			data:      UserAccountData{Username: ""},
			expectErr: true,
		},
		{
			name:      "short username with no password",
			data:      UserAccountData{Username: "ab"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAccount(tt.data)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
		})
	}
}

func TestValidateUserAccount_FullData(t *testing.T) {
	tests := []struct {
		name      string
		data      UserAccountData
		expectErr bool
		errorMsg  string
	}{
		{
			name:      "valid full data",
			data:      UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"},
			expectErr: false,
		},
		{
			name:      "password too short",
			data:      UserAccountData{Username: "john_doe", Password: "abc12", ConfirmPassword: "abc12"},
			expectErr: true,
			errorMsg:  "password must be at least 6 characters",
		},
		{
			name:      "passwords do not match",
			data:      UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "different"},
			expectErr: true,
			errorMsg:  "passwords do not match",
		},
		{
			name:      "missing confirm password",
			data:      UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: ""},
			expectErr: true,
			errorMsg:  "please confirm your password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAccount(tt.data)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
			if tt.expectErr && err != nil && err.Error() != tt.errorMsg {
				t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestValidateDiskSelection(t *testing.T) {
	tests := []struct {
		name      string
		data      DiskSelectionData
		expectErr bool
	}{
		{name: "valid disk ext4", data: DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000", FileSystem: "ext4"}, expectErr: false},
		{name: "valid disk btrfs", data: DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000", FileSystem: "btrfs"}, expectErr: false},
		{name: "empty disk", data: DiskSelectionData{DiskID: ""}, expectErr: true},
		{name: "disk too small", data: DiskSelectionData{DiskID: "/dev/sda", Size: "4000000000"}, expectErr: true},
		{name: "missing filesystem", data: DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000"}, expectErr: true},
		{name: "invalid filesystem", data: DiskSelectionData{DiskID: "/dev/sda", Size: "50000000000", FileSystem: "xfs"}, expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDiskSelection(tt.data)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
		})
	}
}
