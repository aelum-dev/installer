package shared

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestValidateUserAccount_UsernameOnly(t *testing.T) {
	// The GUI validates the username while the password is still empty, and the
	// console installer does the same before prompting. Password checks must be
	// skipped when password is empty.
	tests := []struct {
		name      string
		data      UserAccountData
		expectErr bool
	}{
		{name: "valid username, no password yet", data: UserAccountData{Username: "john_doe"}},
		{name: "empty username with no password", data: UserAccountData{Username: ""}, expectErr: true},
		{name: "one character username", data: UserAccountData{Username: "a"}},
		{name: "uppercase username", data: UserAccountData{Username: "Alice"}},
		{name: "username starting with a digit", data: UserAccountData{Username: "1alice"}},
		{name: "username starting with a dot", data: UserAccountData{Username: ".alice"}},
		{name: "username with a dot", data: UserAccountData{Username: "alice.b"}},
		{name: "username with a trailing dollar", data: UserAccountData{Username: "alice$"}},
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

func TestValidateUserAccount_UsernameFormat(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		expectErr bool
		errorMsg  string
	}{
		// Valid: shadow-utils (useradd) accepts all of these.
		{name: "lowercase", username: "john"},
		{name: "uppercase", username: "John"},
		{name: "all uppercase", username: "JOHN"},
		{name: "mixed case", username: "jOhnDoe"},
		{name: "one character", username: "a"},
		{name: "underscore", username: "_user"},
		{name: "leading digit", username: "1user"},
		{name: "dot", username: "user.name"},
		{name: "leading dot", username: ".user"},
		{name: "hyphen", username: "user-name"},
		{name: "trailing hyphen", username: "user-"},
		{name: "digits and dot", username: "1.23"},
		{name: "trailing dollar", username: "user$"},
		{name: "digits with trailing dollar", username: "123$"},
		{name: "max length (255)", username: strings.Repeat("a", MaxUsernameLength)},

		// Invalid: shadow-utils rejects these.
		{name: "empty", username: "", expectErr: true, errorMsg: "required"},
		{name: "dot", username: ".", expectErr: true, errorMsg: "cannot be"},
		{name: "dot dot", username: "..", expectErr: true, errorMsg: "cannot be"},
		{name: "all digits", username: "123", expectErr: true, errorMsg: "only of digits"},
		{name: "leading hyphen", username: "-user", expectErr: true, errorMsg: "must start with"},
		{name: "leading dollar", username: "$user", expectErr: true, errorMsg: "must start with"},
		{name: "internal dollar", username: "user$name", expectErr: true, errorMsg: "may only contain"},
		{name: "at sign", username: "user@home", expectErr: true, errorMsg: "may only contain"},
		{name: "space", username: "user name", expectErr: true, errorMsg: "may only contain"},
		{name: "hash", username: "user#name", expectErr: true, errorMsg: "may only contain"},
		{name: "colon", username: "user:name", expectErr: true, errorMsg: "may only contain"},
		{name: "slash", username: "user/name", expectErr: true, errorMsg: "may only contain"},
		{name: "plus", username: "user+name", expectErr: true, errorMsg: "may only contain"},
		{name: "non-ASCII later", username: "aliceä", expectErr: true, errorMsg: "may only contain"},
		{name: "non-ASCII first", username: "älice", expectErr: true, errorMsg: "must start with"},
		{name: "too long (256)", username: strings.Repeat("a", MaxUsernameLength+1), expectErr: true, errorMsg: "at most 255"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAccount(UserAccountData{Username: tt.username, Password: "x", ConfirmPassword: "x"})
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
			if tt.expectErr && err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestValidateUserAccount_ExistingAccounts(t *testing.T) {
	// These users/groups exist on a stock install, so useradd would either reuse
	// a system account or fail because its private group already exists.
	for _, name := range []string{"root", "wheel", "alpm", "greeter", "pcscd", "uuidd"} {
		t.Run(name, func(t *testing.T) {
			err := ValidateUserAccount(UserAccountData{Username: name, Password: "x", ConfirmPassword: "x"})
			if err == nil {
				t.Fatalf("expected error for existing account %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "already used") {
				t.Errorf("expected an 'already used' error, got %q", err.Error())
			}
		})
	}

	// Case-sensitive, like Linux: "Root" does not collide with "root".
	if err := ValidateUserAccount(UserAccountData{Username: "Root", Password: "x", ConfirmPassword: "x"}); err != nil {
		t.Errorf("Root should be allowed (case-sensitive), got %v", err)
	}

	// A name that is not on the target must stay allowed even though it is a
	// common account on other distros.
	if err := ValidateUserAccount(UserAccountData{Username: "avahi", Password: "x", ConfirmPassword: "x"}); err != nil {
		t.Errorf("avahi is not on the target and should be allowed, got %v", err)
	}
}

func TestValidateUserAccount_PasswordRules(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		expectErr bool
		errorMsg  string
	}{
		// Linux/PAM accepts any non-empty password up to 511 UTF-8 bytes.
		{name: "one character", password: "x"},
		{name: "spaces", password: "      "},
		{name: "punctuation", password: "!@#$%^&*()"},
		{name: "511 ASCII bytes", password: strings.Repeat("a", MaxPasswordLength)},
		{name: "510 multi-byte bytes", password: strings.Repeat("é", MaxPasswordLength/2)},
		{name: "max length minus one", password: strings.Repeat("a", MaxPasswordLength-1)},

		// PAM (pam_unix/PAM_MAX_RESP_SIZE) rejects 512+ bytes.
		{name: "512 ASCII bytes", password: strings.Repeat("a", MaxPasswordLength+1), expectErr: true, errorMsg: "at most 511"},
		{name: "512 multi-byte bytes", password: strings.Repeat("é", 256), expectErr: true, errorMsg: "at most 511"},
		{name: "newline", password: "pass\nword", expectErr: true, errorMsg: "newlines"},
		{name: "carriage return", password: "pass\rword", expectErr: true, errorMsg: "newlines"},
		{name: "NUL", password: "pass\x00word", expectErr: true, errorMsg: "NUL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAccount(UserAccountData{Username: "alice", Password: tt.password, ConfirmPassword: tt.password})
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
			if tt.expectErr && err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestValidateUserAccount_Confirmation(t *testing.T) {
	tests := []struct {
		name      string
		data      UserAccountData
		expectErr bool
		errorMsg  string
	}{
		{
			name: "valid",
			data: UserAccountData{Username: "alice", Password: "x", ConfirmPassword: "x"},
		},
		{
			name:      "missing confirm",
			data:      UserAccountData{Username: "alice", Password: "x", ConfirmPassword: ""},
			expectErr: true,
			errorMsg:  "please confirm",
		},
		{
			name:      "mismatch",
			data:      UserAccountData{Username: "alice", Password: "x", ConfirmPassword: "y"},
			expectErr: true,
			errorMsg:  "do not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserAccount(tt.data)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error=%v, got err=%v", tt.expectErr, err)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
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
		{name: "valid disk ext4", data: DiskSelectionData{DiskID: "/dev/sda", Size: 50000000000, FileSystem: "ext4"}, expectErr: false},
		{name: "valid disk btrfs", data: DiskSelectionData{DiskID: "/dev/sda", Size: 50000000000, FileSystem: "btrfs"}, expectErr: false},
		{name: "empty disk", data: DiskSelectionData{DiskID: ""}, expectErr: true},
		{name: "disk too small", data: DiskSelectionData{DiskID: "/dev/sda", Size: 4000000000}, expectErr: true},
		{name: "missing filesystem", data: DiskSelectionData{DiskID: "/dev/sda", Size: 50000000000}, expectErr: true},
		{name: "invalid filesystem", data: DiskSelectionData{DiskID: "/dev/sda", Size: 50000000000, FileSystem: "xfs"}, expectErr: true},
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

// fakeDiskSize puts an lsblk shim on PATH that reports the given size, so the
// size validators can be tested without a real disk.
func fakeDiskSize(t *testing.T, sizeBytes uint64) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%d\\n' " + strconv.FormatUint(sizeBytes, 10) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "lsblk"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake lsblk: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestValidateDiskSizeForLayout(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)
	// ESP + plain System floor + GPT regions, rounded up to whole GiB.
	plainDisk := gib * 9
	lvmDisk := gib * 17
	lvmLuksDisk := gib * 17

	tests := []struct {
		name    string
		disk    uint64
		lvm     bool
		luks    bool
		wantErr bool
	}{
		{"plain root on a 9 GiB disk", plainDisk, false, false, false},
		{"LVM on a 9 GiB disk", plainDisk, true, false, true},
		{"LVM on a 17 GiB disk", lvmDisk, true, false, false},
		{"LUKS under LVM on a 17 GiB disk", lvmLuksDisk, true, true, false},
		// 16 GiB + 16 MiB metadata + 32 MiB header + ESP + GPT does not fit in
		// 16 GiB, so the same disk is rejected once both are enabled.
		{"LUKS under LVM on a 16 GiB disk", gib * 16, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeDiskSize(t, tt.disk)
			err := ValidateDiskSizeForLayout("/dev/vda", tt.lvm, tt.luks)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateDiskSizeForLayout(%v, %v) error = %v, wantErr %v", tt.lvm, tt.luks, err, tt.wantErr)
			}
		})
	}
}
