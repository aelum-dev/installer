package steps

import (
	"fmt"
	"os/exec"

	"aesthetic-os-installer/backend/shared"
)

// MountFilesystem mounts the root and EFI partitions using sudo.
// Mount points (standard Arch procedure):
//   - /mnt: root partition
//   - /mnt/boot: EFI partition (where kernel/initrd live, and $BOOT for systemd-boot)
//
// If ctx.Partitions is nil (standalone invocation), it derives partition paths
// from the disk ID: /dev/sda → /dev/sda1 (EFI) + /dev/sda2 (root).
// This matches the layout created by FormatDisk.
func MountFilesystem(ctx *InstallContext) error {
	// Validate that partitions exist (from FormatDisk) or can be derived
	if ctx.Partitions == nil {
		if ctx.DiskID == "" {
			return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --mount-filesystem /dev/vda)")
		}
		// Derive partition layout from disk ID (matches FormatDisk output)
		disk := ctx.DiskID
		ctx.Partitions = &shared.PartitionInfo{
			Disk:     disk,
			EFIPart:  disk + "1",
			RootPart: disk + "2",
		}
	}
	p := ctx.Partitions

	if ctx.Logger != nil {
		ctx.Logger.Info("Mounting root partition (%s) → /mnt", p.RootPart)
	}

	// Mount root partition
	cmd := exec.Command("sudo", ctx.Helper, "mount", p.RootPart, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount root failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Mount EFI partition at /mnt/boot (standard Arch: kernel files live here, on the ESP)
	// This makes /mnt/boot the $BOOT partition for systemd-boot.
	bootMount := "/mnt/boot"
	if ctx.Logger != nil {
		ctx.Logger.Info("Mounting EFI partition (%s) → %s", p.EFIPart, bootMount)
	}

	cmd = exec.Command("sudo", ctx.Helper, "run", "mkdir", "-p", bootMount)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create boot mount point failed: %w: %s", err, string(output))
	}

	cmd = exec.Command("sudo", ctx.Helper, "mount", p.EFIPart, bootMount)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount EFI failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	return nil
}
