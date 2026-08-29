package primitives

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aelum-installer/backend/shared"
)

// ============================================================================
// Command execution primitives
// ============================================================================

// Run executes a command directly (no helper needed when running as root).
func Run(args ...string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...)
}

// RunChroot executes a command inside the installed system via arch-chroot.
func RunChroot(args ...string) ([]byte, error) {
	cmdArgs := append([]string{shared.MountRoot}, args...)
	cmd := exec.Command("arch-chroot", cmdArgs...)
	return cmd.CombinedOutput()
}

// WriteFile writes content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/fstab").
// If the path already starts with MountRoot (e.g., ESP paths), it is used as-is.
func WriteFile(targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave host paths (like ESP) as-is
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	if err := os.WriteFile(hostPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

// AppendFile appends content to a file inside the installed system.
// targetPath should be absolute path inside the chroot (e.g., "/etc/vconsole.conf").
// MountRoot is the installed-system mount point, so chroot-relative paths get
// it prepended. Paths already under MountRoot (e.g. ESP files at /mnt/boot/...)
// are used as-is.
func AppendFile(targetPath string, content string) error {
	// Prepend mount root for chroot-relative paths, but leave paths already
	// under MountRoot (e.g. ESP files at /mnt/boot/...) as-is.
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(hostPath), err)
	}
	f, err := os.OpenFile(hostPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open %s for appending: %w", targetPath, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to %s: %w", targetPath, err)
	}
	return nil
}

// ============================================================================
// Validation helpers
// ============================================================================

// ValidateMountedSystem checks that /mnt is properly mounted with a root partition.
// This is a prerequisite for steps that operate on the installed system.
func ValidateMountedSystem() error {
	// Check if /mnt is mounted
	cmd := Run("findmnt", "-n", "-o", "SOURCE", shared.MountRoot)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return fmt.Errorf("/mnt is not mounted. Run --mount-filesystem <disk> first to mount partitions")
	}

	return nil
}

// ============================================================================
// Cleanup
// ============================================================================

// BuildMountOpts constructs mount options string from a BtrfsSubvolume's properties.
// Returns "subvol=<name>,compress=<val>,nodatacow" or similar based on properties.
func BuildMountOpts(sv *shared.BtrfsSubvolume) string {
	opts := shared.BtrfsSubvolumeMountPrefix + sv.Name
	if sv.Compression != "" {
		opts += ",compress=" + sv.Compression
	}
	if sv.NoDataCow {
		opts += ",nodatacow"
	}
	return opts
}

// CleanupMounts forcefully unmounts everything under /mnt.
func CleanupMounts(logger shared.Logger) error {
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
