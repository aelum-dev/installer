package shared

import (
	"strings"
	"testing"
)

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
		{
			name:      "valid 3-char username with no password",
			data:      UserAccountData{Username: "abc"},
			expectErr: false,
		},
		{
			name:      "username starting with underscore",
			data:      UserAccountData{Username: "_user"},
			expectErr: false,
		},
		{
			name:      "username with hyphen",
			data:      UserAccountData{Username: "user-name"},
			expectErr: false,
		},
		{
			name:      "username with numbers",
			data:      UserAccountData{Username: "user123"},
			expectErr: false,
		},
		{
			name:      "username at max length",
			data:      UserAccountData{Username: "abcdefghijklmnopqrstuvwxyz123456"},
			expectErr: false,
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

func TestValidateUserAccount_UsernameFormat(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		expectErr bool
		errorMsg  string
	}{
		// Valid usernames
		{name: "lowercase letters only", username: "john", expectErr: false},
		{name: "lowercase with underscore", username: "john_doe", expectErr: false},
		{name: "lowercase with hyphen", username: "john-doe", expectErr: false},
		{name: "lowercase with numbers", username: "user123", expectErr: false},
		{name: "starts with underscore", username: "_user", expectErr: false},
		{name: "minimum length (3 chars)", username: "abc", expectErr: false},
		{name: "maximum length (32 chars)", username: "abcdefghijklmnopqrstuvwxyz123456", expectErr: false},

		// Invalid: uppercase
		{name: "uppercase first letter", username: "John", expectErr: true, errorMsg: "must start with a lowercase letter or underscore"},
		{name: "all uppercase", username: "JOHN", expectErr: true, errorMsg: "must start with a lowercase letter or underscore"},
		{name: "mixed case (lower first, upper later)", username: "jOhnDoe", expectErr: true, errorMsg: "must contain only lowercase letters"},

		// Invalid: special characters (pass first char check, fail allowed chars)
		{name: "contains dot", username: "user.name", expectErr: true, errorMsg: "must contain only lowercase letters"},
		{name: "contains at sign", username: "user@home", expectErr: true, errorMsg: "must contain only lowercase letters"},
		{name: "contains hash", username: "user#name", expectErr: true, errorMsg: "must contain only lowercase letters"},
		{name: "contains space", username: "user name", expectErr: true, errorMsg: "must contain only lowercase letters"},

		// Invalid: starts with wrong character
		{name: "starts with hyphen", username: "-user", expectErr: true, errorMsg: "must start with a lowercase letter or underscore"},
		{name: "starts with number", username: "1user", expectErr: true, errorMsg: "must start with a lowercase letter or underscore"},
		{name: "starts with dot", username: ".user", expectErr: true, errorMsg: "must start with a lowercase letter or underscore"},

		// Invalid: length
		{name: "too short (2 chars)", username: "ab", expectErr: true, errorMsg: "at least 3"},
		{name: "too short (1 char)", username: "a", expectErr: true, errorMsg: "at least 3"},
		{name: "too long (33 chars)", username: "abcdefghijklmnopqrstuvwxyz1234567", expectErr: true, errorMsg: "at most 32"},
		{name: "too long (50 chars)", username: "a", expectErr: true}, // Will be set dynamically
	}

	// Set dynamic test case
	tests[17].errorMsg = "at most 32"
	tests[17].username = "a" + strings.Repeat("a", 49)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAccount(UserAccountData{Username: tt.username, Password: "test123", ConfirmPassword: "test123"})
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
			if tt.expectErr && err != nil {
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func TestValidateUserAccount_ReservedNames(t *testing.T) {
	// Test a sample of reserved usernames to ensure they're all blocked
	reservedNames := []string{"root", "bin", "daemon", "nobody", "sshd", "gdm", "pulse"}

	for _, name := range reservedNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateUserAccount(UserAccountData{Username: name, Password: "test123", ConfirmPassword: "test123"})
			if err == nil {
				t.Errorf("expected error for reserved username %q, got nil", name)
				return
			}
			if !strings.Contains(err.Error(), "reserved") {
				t.Errorf("expected error to contain 'reserved', got %q", err.Error())
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
