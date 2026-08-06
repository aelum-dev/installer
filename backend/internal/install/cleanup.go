package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// CleanupMounts performs a full teardown of mounts and disk references for the
// given disk. This is the single cleanup entry point used on install failure,
// Ctrl+C, and rollback.

func CleanupMounts(helper string, logger Logger, diskID string) error {
	if logger != nil {
		logger.Info("Cleanup: unmounting %s", diskID)
	}

	// 1. Sync — flush pending writes.
	if logger != nil {
		logger.Info("Cleanup: syncing")
	}
	if err := runHelper(helper, HelperCmdRun, "sync"); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// 2. Kill processes holding open FDs under /mnt (e.g. daemons started in chroot
	//    like gpg-agent that hold sockets inside the target system).
	//    -M is a safety belt: only act if /mnt is actually a mountpoint. Without it,
	//    fuser -m /mnt resolves to the live ISO rootfs when /mnt is unmounted, which
	//    would kill every process on the system (including Hyprland).
	if logger != nil {
		logger.Info("Cleanup: fuser -k -M -m /mnt")
	}
	_ = runHelper(helper, HelperCmdRun, "fuser", "-k", "-M", "-m", shared.MountRoot)

	// 3. Kill processes holding open FDs on the disk (and its partitions).
	if logger != nil {
		logger.Info("Cleanup: fuser -k %s", diskID)
	}
	_ = runHelper(helper, HelperCmdRun, "fuser", "-k", diskID)

	// 4. Recursive unmount of /mnt and all submounts.
	//    This is the most critical step — if it fails, downstream operations
	//    (wipefs, sgdisk) may operate on still-mounted filesystems.
	if logger != nil {
		logger.Info("Cleanup: umount -R /mnt")
	}
	if err := runHelper(helper, HelperCmdUmount, "-R", shared.MountRoot); err != nil {
		if logger != nil {
			logger.Error("Cleanup: umount failed: %v", err)
		}
		return fmt.Errorf("umount failed: %w", err)
	}
	if logger != nil {
		logger.Info("Cleanup: umount success")
	}

	// 5. Re-read partition table.
	if logger != nil {
		logger.Info("Cleanup: partprobe %s", diskID)
	}
	if err := runHelper(helper, HelperCmdRun, "partprobe", diskID); err != nil {
		if logger != nil {
			logger.Error("Cleanup: partprobe failed: %v", err)
		}
		return fmt.Errorf("partprobe failed: %w", err)
	}
	if logger != nil {
		logger.Info("Cleanup: partprobe success")
	}

	// 6. Remove leftover device-mapper entries (LVM, LUKS, dm-crypt).
	if logger != nil {
		logger.Info("Cleanup: dmsetup remove_all")
	}
	if err := runHelper(helper, HelperCmdRun, "dmsetup", "remove_all"); err != nil {
		if logger != nil {
			logger.Error("Cleanup: dmsetup failed: %v", err)
		}
		return fmt.Errorf("dmsetup remove_all failed: %w", err)
	}
	if logger != nil {
		logger.Info("Cleanup: dmsetup success")
	}

	if logger != nil {
		logger.Info("Cleanup: complete")
	}
	return nil
}

// runHelperPiped executes a helper command and returns its stdout as a string.
// Returns empty string if the command fails or produces no output.
func runHelperPiped(helper string, args ...string) string {
	cmd := exec.Command("sudo", append([]string{helper}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// runHelper executes a privileged command via the helper binary.
func runHelper(helper string, args ...string) error {
	cmd := exec.Command("sudo", append([]string{helper}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (%s)", err, string(output))
	}
	return nil
}

// FormatDiskErrorCleanup tears down mounts left over from a failed FormatDisk.
// Format may have partially partitioned the disk, so we need the same thorough
// teardown as a full cleanup.
func FormatDiskErrorCleanup(diskID string, helper string, logger Logger) error {
	return CleanupMounts(helper, logger, diskID)
}
