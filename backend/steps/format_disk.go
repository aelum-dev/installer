package steps

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"aesthetic-os-installer/backend/shared"
)

// FormatDisk creates partitions and formats them according to the selected layout.
// Before formatting, any mounted partitions on the disk are unmounted first.
// For btrfs, creates subvolumes (@, @home) and sets @ as the default subvolume.
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
	// Mirrors archinstall's device_handler.umount_all_existing() which:
	// 1. Iterates all existing partitions on the device
	// 2. Locks any LUKS encrypted partitions
	// 3. Recursively unmounts all others
	if ctx.Logger != nil {
		ctx.Logger.Info("Unmounting existing partitions on %s", diskID)
	}
	if err := unmountAllPartitions(diskID, ctx.Helper, ctx.Logger); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: failed to unmount all partitions: %v (continuing)", err)
		}
		// Best effort — continue, FormatDiskErrorCleanup below handles /mnt
	}
	// Also unmount /mnt and /mnt/boot (may have leftover mounts)
	FormatDiskErrorCleanup(diskID, ctx.Helper)

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

	// Force kernel to re-read partition table (more reliable than partprobe)
	refreshPartitionTable(ctx, diskID)

	// GPT partition type GUIDs (per archinstall PartitionGUID enum)
	// ESP: C12A7328-F81F-11D2-BA4B-00A0C93EC93B
	// Linux filesystem (generic): 8300
	// Linux root (x86-64): 4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709
	// Linux home (x86-64): 4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709 + linux-home flag
	const (
		gptESP              = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
		gptLinuxRootX86_64  = "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709"
	)

	// Create partitions according to layout
	for i, part := range layout.Partitions {
		partNum := i + 1
		if ctx.Logger != nil {
			ctx.Logger.Info("Creating partition %d: %s (%d MB, %s)", partNum, part.Name, part.SizeMB, part.Fstype)
		}

		// Calculate sgdisk parameters
		var sgdiskArgs []string
		if part.SizeMB > 0 {
			// Fixed size: start at 1MiB (2048 sectors), end at +SizeMB
			sgdiskArgs = []string{"-n", fmt.Sprintf("%d:2048:+%dm", partNum, part.SizeMB)}
		} else {
			// Use rest of disk
			sgdiskArgs = []string{"-n", fmt.Sprintf("%d:0:0", partNum)}
		}

		// Set partition type GUID (per archinstall PartitionGUID values)
		switch part.Fstype {
		case "vfat":
			sgdiskArgs = append(sgdiskArgs, "-t", fmt.Sprintf("%d:%s", partNum, gptESP))
		case "ext4", "btrfs":
			// Use Linux root GUID for root partitions (more specific than generic 8300)
			sgdiskArgs = append(sgdiskArgs, "-t", fmt.Sprintf("%d:%s", partNum, gptLinuxRootX86_64))
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

	// Force kernel to re-read partition table after creating partitions
	// Mirrors archinstall's udev_sync() — ensures kernel has registered all new
	// partition device nodes before we try to format them.
	refreshPartitionTable(ctx, diskID)

	// Wipe filesystem signatures from newly created partitions
	// Mirrors archinstall's partition() → wipefs --all on filtered_part
	// This prevents "signature detected" errors during mkfs
	if ctx.Logger != nil {
		ctx.Logger.Info("Wiping filesystem signatures from new partitions")
	}
	for i := range layout.Partitions {
		partNum := i + 1
		partPath := derivePartitionPath(diskID, partNum)
		cmd = exec.Command("sudo", ctx.Helper, "run", "wipefs", "--all", "-f", partPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: wipefs failed for %s: %v", partPath, err)
			}
		} else if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}
	}

	// Sync to ensure wipefs writes are flushed
	exec.Command("sudo", ctx.Helper, "run", "sync").Run()

	// Format each partition according to layout
	// Mirrors archinstall's FilesystemHandler._format_partitions():
	//   - ext4: mkfs.ext4 -F (force create)
	//   - btrfs: mkfs.btrfs -f (force overwrite)
	//   - vfat: mkfs.fat -F 32 (force, 32-bit FAT)
	for i, part := range layout.Partitions {
		partNum := i + 1
		partPath := derivePartitionPath(diskID, partNum)

		if ctx.Logger != nil {
			ctx.Logger.Info("Formatting partition %d (%s): %s → %s", partNum, part.Name, partPath, part.Fstype)
		}

		// Build format command: format <partition> <fstype> [extra-options...]
		formatArgs := []string{partPath, part.Fstype}
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

	// Sync filesystem writes
	if ctx.Logger != nil {
		ctx.Logger.Info("Syncing filesystem writes")
	}
	exec.Command("sudo", ctx.Helper, "run", "sync").Run()

	// Wait for udev to finish processing all events after formatting
	// Mirrors archinstall's udev_sync() call after formatting
	// This ensures device nodes and UUID information are fully registered
	if ctx.Logger != nil {
		ctx.Logger.Info("Waiting for udev to settle after formatting")
	}
	cmd = exec.Command("sudo", ctx.Helper, "udev-settle")
	cmd.Run()

	// Force kernel to re-read partition table after formatting
	refreshPartitionTable(ctx, diskID)

	// For btrfs subvolume layout, create subvolumes and configure defaults
	// Mirrors archinstall's FilesystemHandler.create_btrfs_volumes():
	//   1. Mount the btrfs partition
	//   2. Create subvolumes with -p (parent reference) for efficiency
	//   3. Set @ as default subvolume
	//   4. Apply nodatacow to @home (for performance on user data)
	//   5. Apply compress=zstd to @ and @home (for space savings)
	//   6. Unmount
	if layoutName == "btrfs-subvolumes" {
		if ctx.Logger != nil {
			ctx.Logger.Info("Setting up btrfs subvolumes (@, @home with compression)")
		}

		// Find root partition (the btrfs one)
		rootPartPath := derivePartitionPath(diskID, 2)

		// Step 1: Mount top-level subvolume (subvolid=5) to a temp location
		// We use /tmp/btrfs-setup to avoid conflicting with /mnt
		tmpMount := "/tmp/btrfs-setup"
		cmd = exec.Command("sudo", ctx.Helper, "run", "mkdir", "-p", tmpMount)
		cmd.Run()

		cmd = exec.Command("sudo", ctx.Helper, "mount", "-o", "subvolid=5", rootPartPath, tmpMount)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to mount btrfs top-level for subvolume creation: %w", err)
		}

		// Step 2: Create subvolumes: @ (root), @home
		// Using -p (parent reference) mirrors archinstall's approach for efficiency
		cmd = exec.Command("sudo", ctx.Helper, "btrfs-subvol-create", tmpMount, "@", "@home")
		output, err = cmd.CombinedOutput()
		if err != nil {
			exec.Command("sudo", ctx.Helper, "umount", "-l", tmpMount).Run()
			return fmt.Errorf("failed to create btrfs subvolumes: %w: %s", err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}

		// Step 3: Set @ as the default subvolume
		// This means mounting the partition without options will use @
		cmd = exec.Command("sudo", ctx.Helper, "btrfs-subvol-set-default", tmpMount, "@")
		output, err = cmd.CombinedOutput()
		if err != nil {
			exec.Command("sudo", ctx.Helper, "umount", "-l", tmpMount).Run()
			return fmt.Errorf("failed to set default btrfs subvolume: %w: %s", err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}

		// Step 4: Set persistent compression on subvolumes using btrfs property
		// Mirrors archinstall's chattr approach but uses the proper btrfs property API
		// This applies regardless of mount options
		for _, subvol := range []string{"@", "@home"} {
			cmd = exec.Command("sudo", ctx.Helper, "btrfs-set-compression", tmpMount, subvol, "zstd")
			output, err = cmd.CombinedOutput()
			if err != nil {
				if ctx.Logger != nil {
					ctx.Logger.Error("Warning: failed to set compression on %s: %v", subvol, err)
				}
			} else if ctx.Logger != nil && len(output) > 0 {
				ctx.Logger.Command(string(output))
			}
		}

		// Step 5: Set nodatacow on @home for performance
		// Mirrors archinstall's BtrfsMountOption.nodatacow
		// This disables copy-on-write for new data in @home, improving write performance
		// for user data (documents, downloads, etc.)
		if ctx.Logger != nil {
			ctx.Logger.Info("Setting nodatacow on @home subvolume")
		}
		cmd = exec.Command("sudo", ctx.Helper, "run", "chattr", "+C", tmpMount+"/@home")
		output, err = cmd.CombinedOutput()
		if err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: failed to set nodatacow on @home: %v", err)
			}
		} else if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}

		// Step 6: Unmount temp location
		exec.Command("sudo", ctx.Helper, "umount", "-l", tmpMount).Run()
		exec.Command("sudo", ctx.Helper, "run", "rmdir", tmpMount).Run()

		if ctx.Logger != nil {
			ctx.Logger.Info("btrfs subvolumes configured: @ (default, compressed), @home (nodatacow + compressed)")
		}
	}

	// Discover partition paths and UUIDs via lsblk + blkid
	if ctx.Logger != nil {
		ctx.Logger.Info("Discovering partition paths and UUIDs")
	}

	// Force kernel to re-read partition table before discovery
	refreshPartitionTable(ctx, diskID)

	partitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("failed to discover partitions: %w", err)
	}

	// Read UUIDs via blkid (directly from superblock, more reliable than lsblk sysfs)
	for i := range partitions {
		uuid, err := readPartitionUUID(ctx.Helper, partitions[i].Path)
		if err != nil {
			return fmt.Errorf("failed to read UUID for %s: %w", partitions[i].Path, err)
		}
		partitions[i].UUID = uuid
		if ctx.Logger != nil {
			ctx.Logger.Info("%s: UUID=%s, FSTYPE=%s", partitions[i].Path, uuid, partitions[i].Fstype)
		}
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
		Disk:        diskID,
		LayoutName:  layoutName,
		EFIPart:     efiPart.Path,
		EFIUUID:     efiPart.UUID,
		RootPart:    rootPart.Path,
		RootUUID:    rootPart.UUID,
		MountPoints: make(map[string]string),
	}

	// For btrfs subvolumes, root partition is used for all subvolume mounts
	if layoutName == "btrfs-subvolumes" {
		partitionInfo.MountPoints["/home"] = rootPart.Path
	}

	ctx.Partitions = partitionInfo
	return nil
}

// derivePartitionPath constructs a partition path from a disk ID and partition number.
// Handles all naming conventions: /dev/sda → /dev/sda1, /dev/nvme0n1 → /dev/nvme0n1p1
func derivePartitionPath(diskID string, partNum int) string {
	if strings.Contains(diskID, "nvme") {
		return fmt.Sprintf("%sp%d", diskID, partNum)
	}
	return fmt.Sprintf("%s%d", diskID, partNum)
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
		switch i {
		case 0:
			efi = &partitions[i]
		case 1:
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

// refreshPartitionTable forces the kernel to re-read the partition table for a disk.
// Uses blockdev --rereadpt which is more reliable than partprobe for ensuring
// the kernel recognizes new partitions immediately. Includes a small delay
// to allow the kernel to fully process the change.
func refreshPartitionTable(ctx *InstallContext, diskID string) {
	if ctx.Logger != nil {
		ctx.Logger.Info("Refreshing partition table for %s", diskID)
	}

	// First sync all pending writes
	exec.Command("sudo", ctx.Helper, "run", "sync").Run()

	// Use blockdev --rereadpt to force kernel re-read (more reliable than partprobe)
	cmd := exec.Command("sudo", ctx.Helper, "run", "blockdev", "--rereadpt", diskID)
	if err := cmd.Run(); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: blockdev --rereadpt failed: %v (continuing)", err)
		}
		// Fallback to partprobe
		cmd = exec.Command("sudo", ctx.Helper, "run", "partprobe", diskID)
		cmd.CombinedOutput()
	}

	// Small delay to let the kernel fully process the partition table change
	// This is critical — without it, subsequent operations may see "Device or resource busy"
	time.Sleep(500 * time.Millisecond)
}

// unmountAllPartitions unmounts all partitions on a given disk device.
// Mirrors archinstall's device_handler.umount_all_existing():
//   1. Discovers all existing partitions via lsblk
//   2. For each partition, either locks LUKS (if encrypted) or unmounts
//   3. Uses recursive lazy unmount to handle nested mounts
//
// This is critical before formatting to ensure no partitions are in use.
func unmountAllPartitions(diskID, helper string, logger Logger) error {
	// Get all partitions on this disk
	partitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("failed to get partitions for unmount: %w", err)
	}

	if logger != nil {
		logger.Info("Found %d partitions on %s to unmount", len(partitions), diskID)
	}

	// First, try lazy recursive unmount of /mnt (covers nested mounts)
	cmd := exec.Command("sudo", helper, "run", "umount", "-lR", "/mnt")
	cmd.Run()

	// Then unmount each partition individually
	for _, part := range partitions {
		if logger != nil {
			logger.Info("Unmounting partition: %s (fstype: %s)", part.Path, part.Fstype)
		}

		// Skip if no fstype (partition may not have a filesystem yet)
		if part.Fstype == "" {
			continue
		}

		// For LUKS encrypted partitions, we'd need to lock them
		// For now, use lazy unmount which handles all cases
		cmd = exec.Command("sudo", helper, "umount", "-l", part.Path)
		if out, err := cmd.CombinedOutput(); err != nil {
			// Ignore errors — partition may not be mounted
			if logger != nil {
				logger.Error("Warning: failed to unmount %s: %v", part.Path, err)
			}
		} else if logger != nil && len(out) > 0 {
			logger.Command(string(out))
		}
	}

	return nil
}
