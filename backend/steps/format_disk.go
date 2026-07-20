package steps

import (
	"fmt"
	"os/exec"
	"strings"

	"aesthetic-os-installer/backend/shared"
)

// FormatDisk creates 2 partitions (EFI + root) and formats them using sudo.
// Partition layout:
//   - /dev/sda1: 128MB EFI (FAT32)
//   - /dev/sda2: Rest of disk (ext4)
//
// Before formatting, any mounted partitions on the disk are unmounted first.
// After formatting, UUIDs are read via blkid and stored in PartitionInfo.
func FormatDisk(ctx *InstallContext) error {
	diskID := ctx.DiskID
	if diskID == "" {
		return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --format-disk /dev/vda)")
	}

	// Unmount ALL partitions on this disk (live ISO may have them mounted)
	if err := FormatDiskErrorCleanup(diskID, ctx.Helper); err != nil {
		return fmt.Errorf("failed to unmount disk partitions: %w", err)
	}

	// Create partition table and partitions
	cmd := exec.Command("sudo", ctx.Helper, "parted", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("partitioning failed: %w: %s", err, string(output))
	}

	efiPartition := diskID + "1"
	rootPartition := diskID + "2"

	// Format EFI partition (FAT32)
	cmd = exec.Command("sudo", ctx.Helper, "format", efiPartition)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format EFI partition failed: %w: %s", err, string(output))
	}

	// Format root partition (ext4)
	cmd = exec.Command("sudo", ctx.Helper, "format", rootPartition)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format root partition failed: %w: %s", err, string(output))
	}

	// Re-read partition table so UUIDs are available
	exec.Command("sudo", ctx.Helper, "run", "partprobe", diskID).Run()

	// Read UUIDs via blkid
	efiUUID, err := readPartitionUUID(ctx.Helper, efiPartition)
	if err != nil {
		return fmt.Errorf("failed to read EFI partition UUID: %w", err)
	}

	rootUUID, err := readPartitionUUID(ctx.Helper, rootPartition)
	if err != nil {
		return fmt.Errorf("failed to read root partition UUID: %w", err)
	}

	ctx.Partitions = &shared.PartitionInfo{
		Disk:     diskID,
		EFIPart:  efiPartition,
		EFIUUID:  efiUUID,
		RootPart: rootPartition,
		RootUUID: rootUUID,
	}

	return nil
}

// readPartitionUUID reads the UUID of a partition using blkid via the helper.
func readPartitionUUID(helper, partition string) (string, error) {
	cmd := exec.Command("sudo", helper, "run", "blkid", "-s", "UUID", "-o", "value", partition)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("blkid failed for %s: %w", partition, err)
	}
	return strings.TrimSpace(string(output)), nil
}
