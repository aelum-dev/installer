package install

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"aestheticos-installer/backend/shared"
)

// CleanupMounts forcefully unmounts everything under /mnt.
//
// It handles the case where a process (e.g. gpg-agent) holds a reference to
// /mnt and prevents umount. The steps are:
//
//  1. Find PIDs of processes accessing the /mnt filesystem via fuser -m.
//  2. Kill all such processes with SIGKILL.
//  3. Recursively unmount /mnt with umount -R.
//
// This is safe to call even if /mnt is not mounted — it will simply no-op.
func CleanupMounts(logger Logger, diskID string) error {
	// Step 1: Find PIDs of processes holding /mnt.
	pids, err := findHoldingPIDs(shared.MountRoot)
	if err != nil {
		if logger != nil {
			logger.Error("CleanupMounts: failed to find holding processes: %v", err)
		}
		return fmt.Errorf("find holding PIDs: %w", err)
	}

	// Step 2: Kill all holding processes and wait for them to exit.
	if len(pids) > 0 {
		if logger != nil {
			logger.Info("CleanupMounts: killing %d process(es) holding %s: %s",
				len(pids), shared.MountRoot, strings.Join(pids, ", "))
		}
		if err := killAndVerifyPIDs(pids); err != nil {
			if logger != nil {
				logger.Error("CleanupMounts: failed to kill processes: %v", err)
			}
			return fmt.Errorf("kill PIDs %s: %w", strings.Join(pids, ", "), err)
		}
	} else if logger != nil {
		logger.Info("CleanupMounts: no processes holding %s", shared.MountRoot)
	}

	// Step 3: Recursively unmount /mnt.
	if err := unmountRecursive(shared.MountRoot); err != nil {
		if logger != nil {
			logger.Error("CleanupMounts: umount -R failed: %v", err)
		}
		return fmt.Errorf("umount -R %s: %w", shared.MountRoot, err)
	}

	if logger != nil {
		logger.Info("CleanupMounts: unmounted %s", shared.MountRoot)
	}
	return nil
}

// findHoldingPIDs returns PIDs of processes accessing the filesystem at mountPoint.
// Uses fuser -m which treats the path as a mount point and lists all processes
// accessing files on that filesystem. Returns empty slice if no processes found.
func findHoldingPIDs(mountPoint string) ([]string, error) {
	// -M ensures we only match if mountPoint is an actual mount point.
	// Without it, fuser -m /mnt on an unmounted directory would scan the
	// host filesystem that /mnt sits on, potentially matching live ISO processes.
	cmd := exec.Command("fuser", "-m", "-M", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		// fuser returns non-zero when no processes are found or the path is
		// not a mount point — both are not errors for cleanup.
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

	// fuser outputs space-separated PIDs (one line).
	fields := strings.Fields(trimmed)
	return fields, nil
}

// killAndVerifyPIDs sends SIGKILL to each PID and waits until each process
// has actually exited. Uses kill -0 to poll for process existence.
func killAndVerifyPIDs(pids []string) error {
	for _, pid := range pids {
		// Send SIGKILL.
		killCmd := exec.Command("kill", "-KILL", pid)
		if err := killCmd.Run(); err != nil {
			// If kill itself fails with "No such process", it's already gone.
			if strings.Contains(err.Error(), "No such process") {
				continue
			}
			return fmt.Errorf("kill -KILL %s: %w", pid, err)
		}

		// Poll until the process is gone. Max 50 attempts × 20ms = 1 second.
		for i := 0; i < 50; i++ {
			checkCmd := exec.Command("kill", "-0", pid)
			if err := checkCmd.Run(); err != nil {
				// kill -0 returns non-zero if process doesn't exist — that's success.
				break
			}
			// Process still alive, wait 20ms and retry.
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

// unmountRecursive recursively unmounts everything under mountPoint.
// Uses umount -R which handles nested mounts (e.g. /mnt/boot unmounts before /mnt).
func unmountRecursive(mountPoint string) error {
	cmd := exec.Command("umount", "-R", mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If already unmounted, that's fine.
		errMsg := strings.ToLower(string(output))
		if strings.Contains(errMsg, "not mounted") || strings.Contains(errMsg, "no such device") {
			return nil
		}
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
