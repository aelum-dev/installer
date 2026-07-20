package steps

import (
	"fmt"
	"os/exec"
)

// MountFilesystem mounts the root and EFI partitions using sudo.
// Mount points (standard Arch procedure):
//   - /mnt: root partition
//   - /mnt/boot: EFI partition (where kernel/initrd live, and $BOOT for systemd-boot)
func MountFilesystem(ctx *InstallContext) error {
	p := ctx.Partitions
	if p == nil {
		return fmt.Errorf("partition info not available (FormatDisk must run first)")
	}

	// Mount root partition
	cmd := exec.Command("sudo", ctx.Helper, "mount", p.RootPart, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount root failed: %w: %s", err, string(output))
	}

	// Mount EFI partition at /mnt/boot (standard Arch: kernel files live here, on the ESP)
	// This makes /mnt/boot the $BOOT partition for systemd-boot.
	bootMount := "/mnt/boot"
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

	return nil
}
