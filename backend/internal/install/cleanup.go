package steps

import (
	"os/exec"
)

// CleanupMounts unmounts common installation mountpoints.
// Used to clean up leftover mounts from previous failed installation attempts.
// Safe to call even if mounts don't exist (errors are ignored).
func CleanupMounts(helper string) error {
	for _, mp := range []string{"/mnt", "/mnt/boot"} {
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
	// Use recursive lazy unmount to handle nested mounts (e.g., /mnt/boot inside /mnt)
	cmd := exec.Command("sudo", helper, "run", "umount", "-lR", "/mnt")
	if out, err := cmd.CombinedOutput(); err != nil {
		// Ignore errors: nothing may be mounted
		_ = out
		_ = err
	}
	return nil
}


