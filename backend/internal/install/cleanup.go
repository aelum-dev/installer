package install

import (
	"fmt"
	"os/exec"
)

// CleanupMounts performs a full teardown of mounts and disk references for the
// given disk. This is the single cleanup entry point used on install failure,
// Ctrl+C, and rollback.
//
// Steps (all delegated to the helper for safety):
//   1. sync — flushes all pending kernel writes to disk
//   2. umount -l -R /mnt — lazy recursive unmount of all mounts under /mnt
//   3. fuser -k <disk> — kills any process still holding an open file descriptor
//      on the target disk and its partitions
//   4. partprobe <disk> — re-reads the partition table so the kernel releases
//      internal references
//   5. dmsetup remove_all — removes leftover device-mapper entries (LVM, LUKS,
//      dm-crypt) that may have been created during installation
//
// Every step tolerates no-op cases (nothing mounted, no processes, etc.).
// Returns an error if the helper binary fails to execute or returns a non-zero
// exit code. Callers should treat a non-nil return as a warning — mounts may
// still be active and manual intervention might be needed.
func CleanupMounts(helper string, logger Logger, diskID string) error {
	if logger != nil {
		logger.Info("Cleanup: tearing down %s", diskID)
	}

	cmd := exec.Command("sudo", helper, "cleanup", diskID)
	if output, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Cleanup: %v (%s)", err, string(output))
		}
		return fmt.Errorf("cleanup failed: %w", err)
	}
	if logger != nil {
		logger.Info("Cleanup: complete")
	}
	return nil
}

// FormatDiskErrorCleanup tears down mounts left over from a failed FormatDisk.
// Format may have partially partitioned the disk, so we need the same thorough
// teardown as a full cleanup.
func FormatDiskErrorCleanup(diskID string, helper string, logger Logger) error {
	return CleanupMounts(helper, logger, diskID)
}
