package steps

import (
	"fmt"
	"os/exec"
)

// FormatDisk creates 2 partitions (EFI + root) and formats them using sudo.
// Partition layout:
//   - /dev/sda1: 128MB EFI (FAT32)
//   - /dev/sda2: Rest of disk (ext4)
// Before formatting, any mounted partitions on the disk are unmounted first.
func FormatDisk(diskID, helper, username, password, language string) error {
	if diskID == "" {
		diskID = "/dev/sda"
	}

	// Unmount ALL partitions on this disk (live ISO may have them mounted)
	if err := FormatDiskErrorCleanup(diskID, helper); err != nil {
		return fmt.Errorf("failed to unmount disk partitions: %w", err)
	}

	// Create partition table and partitions
	cmd := exec.Command("sudo", helper, "parted", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("partitioning failed: %w: %s", err, string(output))
	}

	// Format EFI partition (FAT32)
	efiPartition := diskID + "1"
	cmd = exec.Command("sudo", helper, "format", efiPartition)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format EFI partition failed: %w: %s", err, string(output))
	}

	// Format root partition (ext4)
	rootPartition := diskID + "2"
	cmd = exec.Command("sudo", helper, "format", rootPartition)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format root partition failed: %w: %s", err, string(output))
	}

	return nil
}
