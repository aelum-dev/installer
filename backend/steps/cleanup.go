package steps

import (
	"fmt"
	"os/exec"
)

// CleanupMounts unmounts common installation mountpoints.
// Used to clean up leftover mounts from previous failed installation attempts.
// Safe to call even if mounts don't exist (errors are ignored).
func CleanupMounts(helper string) error {
	for _, mp := range []string{"/mnt", "/mnt/boot/efi"} {
		cmd := exec.Command("sudo", helper, "umount", mp)
		if out, err := cmd.CombinedOutput(); err != nil {
			// Ignore errors: mountpoint may not exist
			_ = out
			_ = err
			continue
		}
	}
	return nil
}

// FormatDiskErrorCleanup unmounts all partitions on a disk.
// Used before formatting to ensure no partitions are mounted.
func FormatDiskErrorCleanup(disk, helper string) error {
	// Query all mountpoints on this disk
	cmd := exec.Command("sudo", helper, "lsblk-partitions", disk)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to query partitions: %w", err)
	}

	// Unmount each one
	for _, mp := range splitLines(string(output)) {
		if mp == "" {
			continue
		}
		umountCmd := exec.Command("sudo", helper, "umount", mp)
		if out, err := umountCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to unmount %s: %w: %s", mp, err, string(out))
		}
	}

	return nil
}

// splitLines splits a string by newlines and trims each line.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
