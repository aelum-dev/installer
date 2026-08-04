package install

import (
	"os/exec"

	"aestheticos-installer/backend/internal/shared"
)

// CleanupMounts unmounts everything after install: all mounts under /mnt
// (including /mnt/boot, btrfs subvolumes, etc.). Uses lazy recursive unmount so
// it works even with nested mounts or lingering references. Safe to call when
// nothing is mounted — errors are ignored.
func CleanupMounts(helper string) {
	cmd := exec.Command("sudo", helper, HelperCmdUmount, UmountRecursiveFlag, shared.MountRoot)
	cmd.Run()
}

// FormatDiskErrorCleanup tears down mounts left over from a failed FormatDisk.
func FormatDiskErrorCleanup(disk, helper string) {
	cmd := exec.Command("sudo", helper, HelperCmdUmount, UmountRecursiveFlag, shared.MountRoot)
	cmd.Run()
}
