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

	if ctx.Logger != nil {
		ctx.Logger.Info("Formatting EFI partition (128MB FAT32) and root partition (rest, ext4)")
	}

	// Format EFI partition (FAT32)
	cmd = exec.Command("sudo", ctx.Helper, "format", diskID+"1")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format EFI partition failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Format root partition (ext4)
	cmd = exec.Command("sudo", ctx.Helper, "format", diskID+"2")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format root partition failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Re-read partition table
	exec.Command("sudo", ctx.Helper, "run", "partprobe", diskID).Run()

	if ctx.Logger != nil {
		ctx.Logger.Info("Discovering partition paths and UUIDs via lsblk")
	}

	// Use lsblk to get actual partition paths (handles NVMe: nvme0n1p1, not nvme0n11)
	partitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("failed to discover partitions: %w", err)
	}

	// Find EFI and root partitions — try fstype first, fall back to partition index.
	// FSTYPE may be empty if the filesystem superblock hasn't been fully synced yet.
	efiPart, rootPart := findPartitionsByFstype(partitions)
	if efiPart == nil || rootPart == nil {
		if ctx.Logger != nil {
			ctx.Logger.Info("FSTYPE not available, falling back to partition index (1=EFI, 2=root)")
		}
		efiPart, rootPart = findPartitionsByIndex(partitions)
	}

	if efiPart == nil {
		return fmt.Errorf("EFI partition not found after formatting (got %d partitions)", len(partitions))
	}
	if rootPart == nil {
		return fmt.Errorf("root partition not found after formatting (got %d partitions)", len(partitions))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("EFI: %s (UUID: %s), Root: %s (UUID: %s)", efiPart.Path, efiPart.UUID, rootPart.Path, rootPart.UUID)
	}

	ctx.Partitions = &shared.PartitionInfo{
		Disk:     diskID,
		EFIPart:  efiPart.Path,
		EFIUUID:  efiPart.UUID,
		RootPart: rootPart.Path,
		RootUUID: rootPart.UUID,
	}

	return nil
}

// findPartitionsByFstype matches EFI (vfat) and root (ext4) partitions by filesystem type.
// Returns nil for any partition not found.
func findPartitionsByFstype(partitions []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range partitions {
		switch partitions[i].Fstype {
		case "vfat":
			if efi == nil {
				efi = &partitions[i]
			}
		case "ext4":
			if root == nil {
				root = &partitions[i]
			}
		}
	}
	return efi, root
}

// findPartitionsByIndex matches partitions by their position.
// Partition index 0 = EFI, index 1 = root. Used as fallback when FSTYPE is unavailable.
func findPartitionsByIndex(partitions []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range partitions {
		if i == 0 {
			efi = &partitions[i]
		} else if i == 1 {
			root = &partitions[i]
			return efi, root
		}
	}
	return efi, root
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
