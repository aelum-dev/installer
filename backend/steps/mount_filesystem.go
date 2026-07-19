package steps

import (
	"fmt"
	"os/exec"
)

// MountFilesystem mounts the root partition and EFI partition using sudo.
// Mount points:
//   - /mnt: root partition (diskID + "2")
//   - /mnt/boot/efi: EFI partition (diskID + "1")
func MountFilesystem(diskID, helper, username, password, language string) error {
	if diskID == "" {
		diskID = "/dev/vda"  // Try virtio first (QEMU/VirtualBox), fallback to sda
	}

	rootPartition := diskID + "2"
	efiPartition := diskID + "1"

	// Force unmount if already mounted (ignore errors)
	exec.Command("sudo", helper, "umount", "/mnt").Run()
	exec.Command("sudo", helper, "umount", "/mnt/boot/efi").Run()
	exec.Command("sudo", helper, "umount", "-f", "/mnt").Run()
	exec.Command("sudo", helper, "umount", "-f", "/mnt/boot/efi").Run()

	// Mount root partition
	cmd := exec.Command("sudo", helper, "mount", rootPartition, "/mnt")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount root failed: %w: %s", err, string(output))
	}

	// Create EFI mount point directory
	efiMount := "/mnt/boot/efi"
	cmd = exec.Command("sudo", helper, "run", "mkdir", "-p", "/mnt/boot/efi")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create EFI mount point failed: %w: %s", err, string(output))
	}

	// Mount EFI partition
	cmd = exec.Command("sudo", helper, "mount", efiPartition, efiMount)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount EFI failed: %w: %s", err, string(output))
	}

	return nil
}
