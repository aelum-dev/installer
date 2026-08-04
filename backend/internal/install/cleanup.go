package install

import (
	"fmt"
	"os"
	"os/exec"

	"aestheticos-installer/backend/internal/shared"
)

// CleanupMounts unmounts everything after install: all mounts under /mnt
// (including /mnt/boot, btrfs subvolumes, etc.). Uses lazy recursive unmount so
// it works even with nested mounts or lingering references. Safe to call when
// nothing is mounted.
func CleanupMounts(helper string) {
	cmd := exec.Command("sudo", helper, HelperCmdUmount, UmountRecursiveFlag, shared.MountRoot)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cleanup unmount failed: %v\n", err)
	}
}

// FormatDiskErrorCleanup tears down mounts left over from a failed FormatDisk.
func FormatDiskErrorCleanup(_ string, helper string) {
	cmd := exec.Command("sudo", helper, HelperCmdUmount, UmountRecursiveFlag, shared.MountRoot)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cleanup unmount failed: %v\n", err)
	}
}
