package install

import (
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// CleanupMounts performs a full teardown after install failure or Ctrl+C:
//  1. sync — flushes all pending kernel writes to disk
//  2. umount -l -R /mnt — lazy recursive unmount of all mounts under /mnt
//  3. fuser -k — kills any process still holding an open file descriptor on
//     the target disk partitions (prevents kernel from retaining the device)
//  4. dmsetup remove_all — removes any leftover device-mapper entries (LVM,
//     LUKS, dm-crypt) that may have been created during installation
//  5. partprobe — re-reads the partition table so the kernel releases all
//     internal references to the disk
//
// Safe to call when nothing is mounted — every step tolerates no-op cases.
func CleanupMounts(helper string, logger Logger) {
	// 1. Find disks — try to find mounted disks first, fallback to all disks.
	//    After unmount lsblk shows "-" for mountpoints, so we can't identify
	//    which disk was used. If /mnt wasn't mounted (e.g., FormatDisk error),
	//    we need to scan all block devices.
	disks := findMountedDisks()
	if len(disks) == 0 {
		disks = findAllDisks()
	}
	if logger != nil {
		logger.Info("Cleanup: found %d disk(s) to clean up", len(disks))
	}

	// Run the cleanup sequence.
	runCleanupSequence(helper, logger, disks)
}

// FormatDiskErrorCleanup tears down mounts left over from a failed FormatDisk.
// Same full cleanup as CleanupMounts — format may have partially partitioned
// the disk, so we need the same thorough teardown.
// diskID is used to target the specific disk even if /mnt wasn't mounted
// (e.g., format failed before mount step).
func FormatDiskErrorCleanup(diskID string, helper string, logger Logger) {
	// If we have a specific disk ID, prefer it over auto-detection.
	if diskID != "" {
		CleanupMountsWithDisk(helper, logger, diskID)
	} else {
		CleanupMounts(helper, logger)
	}
}

// CleanupMountsWithDisk performs cleanup targeting a specific disk device.
// Used when the disk ID is known (e.g., FormatDisk error) but /mnt wasn't mounted.
func CleanupMountsWithDisk(helper string, logger Logger, diskID string) {
	// Normalize disk ID: /dev/vda → /dev/vda
	disks := []string{diskID}
	if logger != nil {
		logger.Info("Cleanup: targeting specific disk %s", diskID)
	}

	// Run the same cleanup sequence with the specific disk.
	runCleanupSequence(helper, logger, disks)
}

// runCleanupSequence performs the actual cleanup steps for the given disks.
func runCleanupSequence(helper string, logger Logger, disks []string) {
	// 1. Sync — flush pending writes.
	if logger != nil {
		logger.Info("Cleanup: syncing kernel buffers")
	}
	syncCmd := exec.Command("sync")
	_ = syncCmd.Run()

	// 2. Lazy recursive unmount.
	if logger != nil {
		logger.Info("Cleanup: unmounting /mnt recursively")
	}
	umountCmd := exec.Command("sudo", helper, HelperCmdUmount, UmountLazyFlag, UmountRecursiveFlag, shared.MountRoot)
	if err := umountCmd.Run(); err != nil {
		if logger != nil {
			logger.Error("Cleanup: unmount failed: %v", err)
		}
	}

	// 3. Remove device-mapper entries.
	if logger != nil {
		logger.Info("Cleanup: removing device-mapper entries")
	}
	dmsetupCmd := exec.Command("sudo", helper, HelperCmdRun, "dmsetup", "remove_all")
	if err := dmsetupCmd.Run(); err != nil {
		if logger != nil {
			logger.Error("Cleanup: dmsetup remove_all failed: %v", err)
		}
	}

	// 4. Kill processes with open FDs on the target disk.
	for _, disk := range disks {
		if logger != nil {
			logger.Info("Cleanup: killing processes with open FDs on %s", disk)
		}
		fuserCmd := exec.Command("sudo", helper, HelperCmdRun, "fuser", "-k", disk)
		if err := fuserCmd.Run(); err != nil {
			if logger != nil {
				logger.Error("Cleanup: fuser -k failed for %s: %v", disk, err)
			}
		}
	}

	// 5. Re-read partition table.
	for _, disk := range disks {
		if logger != nil {
			logger.Info("Cleanup: re-reading partition table for %s", disk)
		}
		partprobeCmd := exec.Command("sudo", helper, HelperCmdRun, "partprobe", disk)
		if err := partprobeCmd.Run(); err != nil {
			if logger != nil {
				logger.Error("Cleanup: partprobe failed for %s: %v", disk, err)
			}
		}
	}

	if logger != nil {
		logger.Info("Cleanup: complete")
	}
}

// findAllDisks queries lsblk to find all top-level block devices (disks).
// Returns device paths like /dev/vda, /dev/sda, etc.
func findAllDisks() []string {
	cmd := exec.Command("lsblk", "-lnbo", "KNAME,TYPE", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil
	}

	var disks []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kname := fields[0]
		devType := fields[1]

		// Only include top-level disk devices (not partitions)
		if devType == "disk" {
			disks = append(disks, "/dev/"+kname)
		}
	}
	return disks
}

// findMountedDisks queries lsblk to find block devices that had mountpoints
// under /mnt before unmount. Returns a list of device paths like /dev/vda.
// If lsblk fails (nothing mounted, or no /mnt), returns an empty slice.
func findMountedDisks() []string {
	// lsblk -lnbo KNAME,MOUNTPOINT,TYPE lists all block devices with their mountpoints.
	// We look for partitions that have a mountpoint under /mnt (i.e., the target
	// disk's partitions mounted during installation).
	cmd := exec.Command("lsblk", "-lnbo", "KNAME,MOUNTPOINT,TYPE", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil
	}

	// Collect disk devices that had children with mountpoints under /mnt.
	diskDevices := make(map[string]bool)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		kname := fields[0]
		mountpoint := fields[1]
		devType := fields[2]

		// Match partitions mounted under /mnt (root, boot, subvolumes, etc.)
		if devType == "part" && strings.HasPrefix(mountpoint, "/mnt/") {
			// kname is like "vda1" — parent is "vda".
			var disk string
			if idx := strings.LastIndex(kname, "p"); idx > 0 {
				disk = kname[:idx]
			} else {
				disk = strings.TrimRight(kname, "0123456789")
			}
			if disk != "" {
				diskDevices[disk] = true
			}
		}
	}

	result := make([]string, 0, len(diskDevices))
	for disk := range diskDevices {
		result = append(result, "/dev/"+disk)
	}
	return result
}
