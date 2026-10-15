package install

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"aelum-installer/backend/install/lvm"
	"aelum-installer/backend/shared"
)

// ============================================================================
// Cleanup
// ============================================================================

// CleanupMounts forcefully unmounts everything under /mnt.
func CleanupMounts(logger Logger) error {
	pids, err := findHoldingPIDs(shared.MountRoot)
	if err != nil {
		return fmt.Errorf("find holding PIDs: %w", err)
	}

	if len(pids) > 0 {
		if err := killAndVerifyPIDs(pids); err != nil {
			return fmt.Errorf("kill PIDs %s: %w", strings.Join(pids, ", "), err)
		}
	}

	if err := unmountRecursive(shared.MountRoot); err != nil {
		return fmt.Errorf("umount -R %s: %w", shared.MountRoot, err)
	}

	return nil
}

func findHoldingPIDs(mountPoint string) ([]string, error) {
	cmd := exec.Command("fuser", "-m", "-M", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, nil
			}
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Fields(trimmed), nil
}

func killAndVerifyPIDs(pids []string) error {
	for _, pid := range pids {
		killCmd := exec.Command("kill", "-KILL", pid)
		if err := killCmd.Run(); err != nil {
			if strings.Contains(err.Error(), "No such process") {
				continue
			}
			return fmt.Errorf("kill -KILL %s: %w", pid, err)
		}
		for i := 0; i < 50; i++ {
			checkCmd := exec.Command("kill", "-0", pid)
			if err := checkCmd.Run(); err != nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

func unmountRecursive(mountPoint string) error {
	cmd := exec.Command("umount", "-R", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.ToLower(string(output))
		if strings.Contains(errMsg, "not mounted") || strings.Contains(errMsg, "no such device") {
			return nil
		}
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// ============================================================================
// Rollback
// ============================================================================

// Rollback cleans up a partial or failed installation.
func Rollback(logger Logger, diskID string) error {
	if diskID == "" {
		return fmt.Errorf("disk ID is required")
	}
	if logger != nil {
		logger.Info("Rolling back: cleaning up %s", diskID)
	}

	if err := CleanupMounts(logger); err != nil {
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
			cmd := Run("wipefs", "--all", "-f", part)
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
	cmd := Run("wipefs", "--all", "-f", diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Failed to wipe disk %s: %v (%s)", diskID, err, string(out))
		}
	}

	if logger != nil {
		logger.Info("Clearing GPT partition table on %s", diskID)
	}
	cmd = Run("sgdisk", "-Z", diskID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if logger != nil {
			logger.Error("Failed to clear GPT table on %s: %v (%s)", diskID, err, string(out))
		}
	}

	cmd = Run("rmdir", shared.MountRoot)
	_ = cmd.Run()

	if logger != nil {
		logger.Info("Rollback complete: %s is ready for a fresh install", diskID)
	}
	return nil
}

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
			if !strings.HasPrefix(name, "/dev/") {
				name = "/dev/" + name
			}
			partitions = append(partitions, name)
		}
	}
	return partitions
}
