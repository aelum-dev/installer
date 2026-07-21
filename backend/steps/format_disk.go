package steps

import (
	"fmt"
	"os/exec"
	"strings"

	"aesthetic-os-installer/backend/shared"
)

// FormatDisk creates partitions and formats them according to the selected layout.
// Before formatting, any mounted partitions on the disk are unmounted first.
// After formatting, UUIDs are read via blkid and stored in PartitionInfo.
func FormatDisk(ctx *InstallContext) error {
	diskID := ctx.DiskID
	if diskID == "" {
		return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --format-disk /dev/vda)")
	}

	// Determine filesystem type
	fsType := ctx.FileSystem
	if fsType == "" {
		fsType = "ext4" // default fallback
	}
	if fsType != "ext4" && fsType != "btrfs" {
		return fmt.Errorf("unsupported filesystem type: %s (supported: ext4, btrfs)", fsType)
	}

	// Determine layout
	layoutName := ctx.LayoutName
	if layoutName == "" {
		layoutName = "default"
	}

	layout, err := shared.GetLayout(layoutName, fsType)
	if err != nil {
		return fmt.Errorf("failed to get layout: %w", err)
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Using layout: %s (%s)", layout.Name, layout.Description)
	}

	// Unmount ALL partitions on this disk (live ISO may have them mounted)
	if ctx.Logger != nil {
		ctx.Logger.Info("Unmounting existing partitions on %s", diskID)
	}
	if err := FormatDiskErrorCleanup(diskID, ctx.Helper); err != nil {
		return fmt.Errorf("failed to unmount disk partitions: %w", err)
	}

	// Clear disk and create partition table
	if ctx.Logger != nil {
		ctx.Logger.Info("Creating partition table on %s", diskID)
	}
	cmd := exec.Command("sudo", ctx.Helper, "parted", diskID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("partitioning failed: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	// Create partitions according to layout
	for i, part := range layout.Partitions {
		partNum := i + 1
		if ctx.Logger != nil {
			ctx.Logger.Info("Creating partition %d: %s (%d MB, %s)", partNum, part.Name, part.SizeMB, part.Fstype)
		}

		// Calculate sgdisk parameters
		var sgdiskArgs []string
		if part.SizeMB > 0 {
			// Fixed size: start at 1MiB, end at +SizeMB
			sgdiskArgs = []string{"-n", fmt.Sprintf("%d:2048:+%dm", partNum, part.SizeMB)}
		} else {
			// Use rest of disk
			sgdiskArgs = []string{"-n", fmt.Sprintf("%d:0:0", partNum)}
		}

		// Set partition type
		switch part.Fstype {
		case "vfat":
			sgdiskArgs = append(sgdiskArgs, "-t", fmt.Sprintf("%d:EF00", partNum))
		default:
			sgdiskArgs = append(sgdiskArgs, "-t", fmt.Sprintf("%d:8300", partNum))
		}

		// Set partition name
		sgdiskArgs = append(sgdiskArgs, "-c", fmt.Sprintf("%d:%s", partNum, part.Name))

		cmd = exec.Command("sudo", ctx.Helper, "run", "sgdisk")
		cmd.Args = append(cmd.Args, sgdiskArgs...)
		cmd.Args = append(cmd.Args, diskID)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create partition %d (%s): %w: %s", partNum, part.Name, err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
	}

	// Re-read partition table
	exec.Command("sudo", ctx.Helper, "run", "partprobe", diskID).Run()

	// Format each partition according to layout
	for i, part := range layout.Partitions {
		partNum := i + 1
		partPath := diskID + fmt.Sprintf("%d", partNum)

		// Handle NVMe naming: /dev/nvme0n1 → /dev/nvme0n1p1
		if strings.Contains(diskID, "nvme") {
			partPath = diskID + "p" + fmt.Sprintf("%d", partNum)
		}

		if ctx.Logger != nil {
			ctx.Logger.Info("Formatting partition %d (%s): %s → %s", partNum, part.Name, partPath, part.Fstype)
		}

		// Build format command
		var formatArgs []string
		formatArgs = append(formatArgs, partPath)
		formatArgs = append(formatArgs, part.Fstype)
		if part.Options != "" {
			formatArgs = append(formatArgs, strings.Split(part.Options, " ")...)
		}

		cmd = exec.Command("sudo", ctx.Helper, "format")
		cmd.Args = append(cmd.Args, formatArgs...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("format partition %d (%s) failed: %w: %s", partNum, part.Name, err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
	}

	// For btrfs subvolume layout, create subvolumes
	if layoutName == "btrfs-subvolumes" {
		if ctx.Logger != nil {
			ctx.Logger.Info("Creating btrfs subvolumes (@ and @home)")
		}

		// Find root partition
		partitions, err := shared.GetPartitions(diskID)
		if err != nil {
			return fmt.Errorf("failed to discover partitions: %w", err)
		}

		var rootPart *shared.PartitionEntry
		for i := range partitions {
			if partitions[i].Fstype == "btrfs" {
				rootPart = &partitions[i]
				break
			}
		}

		if rootPart == nil {
			return fmt.Errorf("btrfs root partition not found after formatting")
		}

		// Temporarily mount root to create subvolumes
		cmd = exec.Command("sudo", ctx.Helper, "mount", rootPart.Path, "/mnt")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to mount root for subvolume creation: %w", err)
		}

		// Create subvolumes
		cmd = exec.Command("sudo", ctx.Helper, "btrfs-subvol-create", "/mnt", "@", "@home")
		output, err = cmd.CombinedOutput()
		if err != nil {
			// Unmount on error
			exec.Command("sudo", ctx.Helper, "umount", "/mnt").Run()
			return fmt.Errorf("failed to create btrfs subvolumes: %w: %s", err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}

		// Unmount root
		exec.Command("sudo", ctx.Helper, "umount", "/mnt").Run()
	}

	// Discover partition paths and UUIDs via lsblk
	if ctx.Logger != nil {
		ctx.Logger.Info("Discovering partition paths and UUIDs via lsblk")
	}

	partitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("failed to discover partitions: %w", err)
	}

	// Match partitions by fstype, then by index
	efiPart, rootPart := findPartitionsByFstype(partitions)
	if efiPart == nil || rootPart == nil {
		if ctx.Logger != nil {
			ctx.Logger.Info("FSTYPE not available, falling back to partition index (1=EFI, 2+=root)")
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

	// Build partition info
	partitionInfo := &shared.PartitionInfo{
		Disk:       diskID,
		LayoutName: layoutName,
		EFIPart:    efiPart.Path,
		EFIUUID:    efiPart.UUID,
		RootPart:   rootPart.Path,
		RootUUID:   rootPart.UUID,
		MountPoints: make(map[string]string),
	}

	// For btrfs subvolumes, root partition is used for all subvolume mounts
	if layoutName == "btrfs-subvolumes" {
		partitionInfo.MountPoints["/home"] = rootPart.Path
	}

	ctx.Partitions = partitionInfo
	return nil
}

// findPartitionsByFstype matches EFI (vfat) and root (ext4/btrfs) partitions by filesystem type.
// Returns nil for any partition not found.
func findPartitionsByFstype(partitions []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range partitions {
		switch partitions[i].Fstype {
		case "vfat":
			if efi == nil {
				efi = &partitions[i]
			}
		case "ext4", "btrfs":
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
