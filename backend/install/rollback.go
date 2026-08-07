package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/shared"
)

// Rollback cleans up a partial or failed installation:
// unmounts everything under /mnt, wipes filesystem signatures from all
// partitions and the disk itself, then removes the /mnt mount point.
// After rollback the disk is ready for a fresh installation.
//
// This is a standalone function — it does not require an Installer instance.
func Rollback(logger Logger, diskID string) error {
	if diskID == "" {
		return fmt.Errorf("disk ID is required")
	}

	if logger != nil {
		logger.Info("Rolling back: cleaning up %s", diskID)
	}

	// 1. Unmount everything under /mnt (lazy recursive), kill processes,
	//    partprobe, dmsetup remove_all.
	//    Fail fast if cleanup fails — proceeding to wipe mounted filesystems
	//    can cause kernel panics or silent corruption.
	if err := CleanupMounts(logger, diskID); err != nil {
		if logger != nil {
			logger.Error("Rollback: cleanup failed: %v", err)
		}
		return fmt.Errorf("rollback cleanup failed: %w", err)
	}

	// 2. Wipe filesystem signatures from all partitions.
	//    Use lsblk to discover partitions (handles any number, not just 2).
	//    If the disk has no partitions (FormatDisk never completed), skip.
	partitions := discoverPartitions(diskID)
	if len(partitions) == 0 {
		if logger != nil {
			logger.Info("Rollback: no partitions found on %s, skipping wipe", diskID)
		}
	} else {
		for _, part := range partitions {
			if logger != nil {
				logger.Info("Wiping signatures on %s", part)
			}
			cmd := Run("wipefs", "--all", "-f", part)
			if out, err := cmd.CombinedOutput(); err != nil {
				if logger != nil {
					logger.Error("Failed to wipe %s: %v (%s)", part, err, string(out))
				}
				// Don't fail — some partitions may not exist yet if FormatDisk
				// failed early. Continue wiping the rest.
			}
		}
	}

	// 3. Wipe signatures on the disk itself (GPT backup, etc.).
	if logger != nil {
		logger.Info("Wiping signatures on %s", diskID)
	}
	cmd := Run("wipefs", "--all", "-f", diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Failed to wipe disk %s: %v (%s)", diskID, err, string(out))
		}
		// Non-fatal — disk may not have been partitioned yet.
	}

	// 4. Clear the GPT partition table so no stale partition entries remain.
	//    wipefs removes signatures but leaves partition entries intact;
	//    sgdisk -Z removes everything for a completely clean slate.
	if logger != nil {
		logger.Info("Clearing GPT partition table on %s", diskID)
	}
	cmd = Run("sgdisk", "-Z", diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Failed to clear GPT table on %s: %v (%s)", diskID, err, string(out))
		}
		// Non-fatal — disk may not have a GPT table yet.
	}

	// 5. Remove the /mnt mount point directory.
	cmd = Run("rmdir", shared.MountRoot)
	_ = cmd.Run()

	if logger != nil {
		logger.Info("Rollback complete: %s is ready for a fresh install", diskID)
	}
	return nil
}

// discoverPartitions uses lsblk to find all partition devices for a given disk.
// Returns device paths like /dev/vda1, /dev/vda2, etc.
func discoverPartitions(diskID string) []string {
	cmd := exec.Command("lsblk", "-lnbo", "KNAME,TYPE", diskID)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil
	}

	var partitions []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == "part" {
			name := fields[0]
			// Avoid double-prefixing if lsblk already returns an absolute path.
			if !strings.HasPrefix(name, "/dev/") {
				name = "/dev/" + name
			}
			partitions = append(partitions, name)
		}
	}
	return partitions
}
