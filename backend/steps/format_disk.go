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

	if ctx.Logger != nil {
		ctx.Logger.Info("Unmounting existing partitions on %s", diskID)
	}

	// Unmount ALL partitions on this disk (live ISO may have them mounted)
	if err := FormatDiskErrorCleanup(diskID, ctx.Helper); err != nil {
		return fmt.Errorf("failed to unmount disk partitions: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Creating partition table on %s", diskID)
	}

	// Create partition table and partitions
	cmd := exec.Command("sudo", ctx.Helper, "parted", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("partitioning failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	efiPartition := diskID + "1"
	rootPartition := diskID + "2"

	if ctx.Logger != nil {
		ctx.Logger.Info("Formatting EFI partition (%s) as FAT32", efiPartition)
	}

	// Format EFI partition (FAT32)
	cmd = exec.Command("sudo", ctx.Helper, "format", efiPartition)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format EFI partition failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Formatting root partition (%s) as ext4", rootPartition)
	}

	// Format root partition (ext4)
	cmd = exec.Command("sudo", ctx.Helper, "format", rootPartition)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format root partition failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Re-read partition table so UUIDs are available
	exec.Command("sudo", ctx.Helper, "run", "partprobe", diskID).Run()

	if ctx.Logger != nil {
		ctx.Logger.Info("Reading partition UUIDs")
	}

	// Read UUIDs via blkid
	efiUUID, err := readPartitionUUID(ctx.Helper, efiPartition)
	if err != nil {
		return fmt.Errorf("failed to read EFI partition UUID: %w", err)
	}

	rootUUID, err := readPartitionUUID(ctx.Helper, rootPartition)
	if err != nil {
		return fmt.Errorf("failed to read root partition UUID: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("UUID: EFI=%s, Root=%s", efiUUID, rootUUID)
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
