// Package luks implements the LUKS2 operations the installer needs to encrypt
// the root partition: format, open, close, and read the header UUID.
//
// Only the root partition is encrypted; the EFI partition stays plaintext
// (mandatory for UEFI boot). Every command feeds the passphrase via a stdin
// pipe (never a flag) so it never appears in /proc, ps, or shell history.
package luks

import (
	"fmt"
	"os/exec"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// cryptoArgs returns the shared cryptsetup flags pinned to current best
// practices (see luks.md §11): LUKS2, aes-xts-plain64, 512-bit key (AES-256
// under XTS), argon2id KDF, sha256 workfactor hash, 2000ms iter time.
func cryptoArgs() []string {
	return []string{
		"--batch-mode",
		"--type", shared.LuksType,
		"--cipher", shared.LuksCipher,
		"--key-size", fmt.Sprintf("%d", shared.LuksKeySizeBits),
		"--hash", shared.LuksHash,
		"--pbkdf", shared.LuksPBKDF,
		"--iter-time", fmt.Sprintf("%d", shared.LuksIterTimeMs),
	}
}

// isLuksArgs builds the `cryptsetup isLuks <path>` arguments.
func isLuksArgs(path string) []string {
	return []string{"cryptsetup", "isLuks", path}
}

// formatArgs builds the `cryptsetup luksFormat --key-file=- <path>` arguments.
// The passphrase is supplied on stdin (never as an argument), so --key-file=-
// reads it from the pipe set up by feedPassphrase.
func formatArgs(path string) []string {
	args := []string{"cryptsetup", "luksFormat", "--key-file", "-"}
	args = append(args, cryptoArgs()...)
	args = append(args, path)
	return args
}

// openArgs builds the `cryptsetup open --type luks --key-file=- <path> <mapper>`
// arguments. The passphrase is supplied on stdin, never as an argument.
func openArgs(path, mapperName string) []string {
	return append([]string{"cryptsetup", "open", "--type", "luks", "--key-file", "-"}, path, mapperName)
}

// closeArgs builds the `cryptsetup close <mapper>` arguments.
func closeArgs(mapperName string) []string {
	return []string{"cryptsetup", "close", mapperName}
}

// uuidArgs builds the `cryptsetup luksUUID <path>` arguments.
func uuidArgs(path string) []string {
	return []string{"cryptsetup", "luksUUID", path}
}

// feedPassphrase supplies the passphrase to cmd on stdin (never as an
// argument), so it never appears in /proc, ps, or shell history.
//
// A strings.Reader is used instead of an io.Pipe: os/exec wraps any io.Reader
// stdin with its own io.Copy goroutine, and pairing that with a goroutine
// writing to an io.Pipe deadlocks — PipeWriter.Write blocks waiting for a
// reader while the copy's PipeReader.Read blocks waiting for a writer, so
// neither sends and cryptsetup's stdin is starved. That hung the luksFormat
// step forever.
func feedPassphrase(cmd *exec.Cmd, passphrase string) {
	cmd.Stdin = strings.NewReader(passphrase)
}

// IsLuks reports whether path already carries a LUKS2 header.
func IsLuks(path string) bool {
	return primitives.Run(isLuksArgs(path)...).Run() == nil
}

// LuksFormat initializes a LUKS2 header on path with the given passphrase.
func LuksFormat(logger shared.Logger, path, passphrase string) error {
	cmd := primitives.Run(formatArgs(path)...)
	feedPassphrase(cmd, passphrase)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("luksFormat %s failed: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	if logger != nil {
		logger.Info("Formatted LUKS2 header on %s", path)
	}
	return nil
}

// LuksOpen opens the LUKS device at path, creating the decrypted mapper
// device at /dev/mapper/<mapperName>.
func LuksOpen(path, mapperName, passphrase string) error {
	cmd := primitives.Run(openArgs(path, mapperName)...)
	feedPassphrase(cmd, passphrase)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("luksOpen %s -> %s failed: %w: %s", path, mapperName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// LuksClose closes the decrypted mapper device.
func LuksClose(mapperName string) error {
	out, err := primitives.Run(closeArgs(mapperName)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("luksClose %s failed: %w: %s", mapperName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// LuksUUID returns the LUKS header UUID of path (used for the boot cmdline).
func LuksUUID(path string) (string, error) {
	out, err := primitives.Run(uuidArgs(path)...).Output()
	if err != nil {
		return "", fmt.Errorf("luksUUID %s: %w", path, err)
	}
	return strings.TrimSpace(string(out)), nil
}
