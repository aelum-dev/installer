package luks

import (
	"reflect"
	"testing"

	"aelum-installer/backend/shared"
)

// TestCryptoArgs verifies the pinned LUKS2 parameters match current best
// practices (luks.md §11): LUKS2, aes-xts-plain64, 512-bit key (AES-256 under
// XTS), argon2id KDF, sha256 hash, 2000ms iter time.
func TestCryptoArgs(t *testing.T) {
	want := []string{
		"--batch-mode",
		"--type", shared.LuksType,
		"--cipher", shared.LuksCipher,
		"--key-size", "512",
		"--hash", shared.LuksHash,
		"--pbkdf", shared.LuksPBKDF,
		"--iter-time", "2000",
	}
	got := cryptoArgs()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cryptoArgs() = %v, want %v", got, want)
	}
}

// TestFormatArgs verifies luksFormat is invoked with --key-file=- (passphrase
// comes via stdin, never the args) and the target path is the final argument.
func TestFormatArgs(t *testing.T) {
	got := formatArgs("/dev/vda2")
	want := []string{"cryptsetup", "luksFormat", "--key-file", "-"}
	want = append(want, cryptoArgs()...)
	want = append(want, "/dev/vda2")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("formatArgs() = %v, want %v", got, want)
	}
	// The passphrase must never appear in the argument list.
	for _, a := range got {
		if a == "secret" {
			t.Errorf("passphrase leaked into format args: %v", got)
		}
	}
	// The passphrase must never appear in the argument list.
	for _, a := range got {
		if a == "secret" {
			t.Errorf("passphrase leaked into format args: %v", got)
		}
	}
}

// TestOpenArgs verifies open uses --key-file=- (stdin) and appends path + mapper.
func TestOpenArgs(t *testing.T) {
	got := openArgs("/dev/vda2", shared.LuksMapperName)
	want := []string{"cryptsetup", "open", "--type", "luks", "--key-file", "-", "/dev/vda2", shared.LuksMapperName}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("openArgs() = %v, want %v", got, want)
	}
}

// TestCloseArgs verifies close targets the mapper name only.
func TestCloseArgs(t *testing.T) {
	got := closeArgs(shared.LuksMapperName)
	want := []string{"cryptsetup", "close", shared.LuksMapperName}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("closeArgs() = %v, want %v", got, want)
	}
}

// TestUUIDArgs verifies uuid reads the header UUID from the raw partition.
func TestUUIDArgs(t *testing.T) {
	got := uuidArgs("/dev/vda2")
	want := []string{"cryptsetup", "luksUUID", "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("uuidArgs() = %v, want %v", got, want)
	}
}

// TestIsLuksArgs verifies the isLuks check targets the raw partition.
func TestIsLuksArgs(t *testing.T) {
	got := isLuksArgs("/dev/vda2")
	want := []string{"cryptsetup", "isLuks", "/dev/vda2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("isLuksArgs() = %v, want %v", got, want)
	}
}
