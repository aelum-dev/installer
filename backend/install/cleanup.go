package install

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Rollback
// ============================================================================

// Rollback cleans up a partial or failed installation.
func Rollback(logger shared.Logger, diskID string) error {
	if diskID == "" {
		return fmt.Errorf("disk ID is required")
	}
	if logger != nil {
		logger.Info("Rolling back: cleaning up %s", diskID)
	}

	if err := primitives.CleanupMounts(logger); err != nil {
		if logger != nil {
			logger.Error("Rollback: cleanup failed: %v", err)
		}
		return fmt.Errorf("rollback cleanup failed: %w", err)
	}

	partitions := discoverPartitions(diskID)

	// Remove any `aelum` volume group left behind by a partial install before
	// wiping signatures: vgchange -an → vgremove -ff → pvremove. Keeps a retry
	// from failing with "VG already exists".
	if logger != nil {
		logger.Info("Rolling back LVM volume group %s (if present)", shared.LvmVolumeGroup)
	}
	lvm.Teardown(shared.LvmVolumeGroup, partitions)

	if len(partitions) == 0 {
		if logger != nil {
			logger.Info("Rollback: no partitions found on %s, skipping wipe", diskID)
		}
	} else {
		for _, part := range partitions {
			if logger != nil {
				logger.Info("Wiping signatures on %s", part)
			}
			cmd := primitives.Run("wipefs", "--all", "-f", part)
			if out, err := cmd.CombinedOutput(); err != nil {
				if logger != nil {
					logger.Error("Failed to wipe %s: %v (%s)", part, err, string(out))
				}
			}
		}
	}

	if logger != nil {
		logger.Info("Wiping signatures on %s", diskID)
	}
	cmd := primitives.Run("wipefs", "--all", "-f", diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Failed to wipe disk %s: %v (%s)", diskID, err, string(out))
		}
	}

	if logger != nil {
		logger.Info("Clearing GPT partition table on %s", diskID)
	}
	cmd = primitives.Run("sgdisk", "-Z", diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Failed to clear GPT table on %s: %v (%s)", diskID, err, string(out))
		}
	}

	cmd = primitives.Run("rmdir", shared.MountRoot)
	_ = cmd.Run()

	if logger != nil {
		logger.Info("Rollback complete: %s is ready for a fresh install", diskID)
	}
	return nil
}

func discoverPartitions(diskID string) []string {
	cmd := primitives.Run("lsblk", "-lnbo", "KNAME,TYPE", diskID)
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
			if !strings.HasPrefix(name, "/dev/") {
				name = "/dev/" + name
			}
			partitions = append(partitions, name)
		}
	}
	return partitions
}
