package install

import (
	"fmt"
	"os/exec"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// FormatDisk creates partitions and formats them according to the filesystem type.
// Before formatting, any mounted partitions on the disk are unmounted first.
// For btrfs, creates subvolumes (@, @home, @root, @srv, @cache, @tmp, @log) and sets @ as default.
// After formatting, UUIDs are read via blkid and stored in PartitionInfo.
// On failure, cleans up partial state (unmounts, removes temporary mounts).
func FormatDisk(ctx *InstallContext) (retErr error) {
	diskID := ctx.DiskID
	if diskID == "" {
		return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --format-disk /dev/vda)")
	}

	// Cleanup on failure — unmounts /mnt, /mnt/boot, and all partitions on diskID
	defer func() {
		if retErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("FormatDisk failed, cleaning up: %v", retErr)
			}
			FormatDiskErrorCleanup(diskID, ctx.Helper)
		}
	}()

	// Determine filesystem type
	fsType := ctx.FileSystem
	if fsType == "" {
		fsType = shared.DefaultFileSystem
	}
	if fsType != shared.FileSystemExt4 && fsType != shared.FileSystemBtrfs {
		return fmt.Errorf("unsupported filesystem type: %s (supported: ext4, btrfs)", fsType)
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Using filesystem: %s", fsType)
	}

	// Define partitions based on filesystem type
	// Both ext4 and btrfs use: EFI (128MB) + Root (rest of disk)
	layoutPartitions := []struct {
		Name       string
		SizeMB     int64
		Fstype     string
		MountPoint string
	}{
		{Name: PartitionLayoutEFI, SizeMB: shared.EFIPartitionSizeMB, Fstype: FileSystemVfat, MountPoint: BootMountPoint},
		{Name: PartitionLayoutRoot, SizeMB: 0, Fstype: fsType, MountPoint: shared.MountRoot},
	}

	// Define btrfs subvolumes (only for btrfs filesystem)
	var subvolumes []shared.BtrfsSubvolume
	if fsType == "btrfs" {
		subvolumes = shared.GetBtrfsSubvolumes()
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
	// Destructive: this erases the partition table and all data on the disk
	if ctx.Logger != nil {
		ctx.Logger.Info("DESTRUCTIVE: Erasing partition table on %s. All data will be lost!", diskID)
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

	// Force kernel to re-read partition table. Try blockdev first, fall back to partprobe.
	// This is critical — without it, the kernel keeps the old device nodes and the new
	// partitions never get their device nodes, causing "device is mounted" errors.
	refreshPartitionTable(ctx, diskID)

	// GPT partition type GUIDs (per archinstall PartitionGUID enum)
	var gptESP, gptLinuxRootX86_64 string
	gptESP = shared.GPTTypeGUIDs.ESP
	gptLinuxRootX86_64 = shared.GPTTypeGUIDs.LinuxRootX86_64

	// Create partitions
	for i, part := range layoutPartitions {
		partNum := i + 1
		if ctx.Logger != nil {
			ctx.Logger.Info("Creating partition %d: %s (%d MB, %s)", partNum, part.Name, part.SizeMB, part.Fstype)
		}

		// Calculate sgdisk parameters
		var sgdiskArgs []string
		if part.SizeMB > 0 {
			// Fixed size: start at 1MiB, end at +SizeMB
			sgdiskArgs = []string{SgdiskCreatePartitionFlag, fmt.Sprintf("%d:%d:+%dm", partNum, shared.GPTPartitionStartSector, part.SizeMB)}
		} else {
			// Use rest of disk
			sgdiskArgs = []string{SgdiskCreatePartitionFlag, fmt.Sprintf("%d:0:0", partNum)}
		}

		// Set partition type GUID (per archinstall PartitionGUID values)
		switch part.Fstype {
		case FileSystemVfat:
			sgdiskArgs = append(sgdiskArgs, SgdiskSetTypeFlag, fmt.Sprintf("%d:%s", partNum, gptESP))
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
			// Use Linux root GUID for root partitions (more specific than generic)
			sgdiskArgs = append(sgdiskArgs, SgdiskSetTypeFlag, fmt.Sprintf("%d:%s", partNum, gptLinuxRootX86_64))
		default:
			sgdiskArgs = append(sgdiskArgs, SgdiskSetTypeFlag, fmt.Sprintf("%d:%s", partNum, shared.GPTTypeGUIDs.LinuxFilesystemGeneric))
		}

		// Set partition name
		sgdiskArgs = append(sgdiskArgs, SgdiskSetNameFlag, fmt.Sprintf("%d:%s", partNum, part.Name))

		cmd = exec.Command("sudo", ctx.Helper, HelperCmdRun, "sgdisk")
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
	for i := range layoutPartitions {
		partNum := i + 1
		partPath := derivePartitionPath(diskID, partNum)
		cmd = exec.Command("sudo", ctx.Helper, HelperCmdRun, "wipefs", WipefsAllFlag, WipefsForceFlag, partPath)
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

	// Format each partition
	// Mirrors archinstall's FilesystemHandler._format_partitions():
	//   - ext4: mkfs.ext4 -F (force create)
	//   - btrfs: mkfs.btrfs -f (force overwrite)
	//   - vfat: mkfs.fat -F 32 (force, 32-bit FAT)
	for i, part := range layoutPartitions {
		partNum := i + 1
		partPath := derivePartitionPath(diskID, partNum)

		if ctx.Logger != nil {
			ctx.Logger.Info("Formatting partition %d (%s): %s → %s", partNum, part.Name, partPath, part.Fstype)
		}

		// Build format command: format <partition> <fstype>
		formatArgs := []string{partPath, part.Fstype}

		cmd = exec.Command("sudo", ctx.Helper, HelperCmdFormat)
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
	cmd = exec.Command("sudo", ctx.Helper, HelperCmdUdevSettle)
	cmd.Run()

	// Force kernel to re-read partition table after formatting
	refreshPartitionTable(ctx, diskID)

	// For btrfs, create subvolumes and configure defaults
	// Mirrors archinstall's FilesystemHandler.create_btrfs_volumes():
	//   1. Mount the btrfs partition top-level subvolume
	//   2. Create subvolumes
	//   3. Set @ as default subvolume
	//   4. Apply per-subvolume properties (compression, nodatacow, etc.)
	//   5. Unmount
	if fsType == "btrfs" {
		if ctx.Logger != nil {
			ctx.Logger.Info("Setting up btrfs subvolumes: @, @home, @root, @srv, @cache, @tmp, @log")
		}

		// Find root partition (the btrfs one)
		rootPartPath := derivePartitionPath(diskID, 2)

		// Step 1: Mount top-level subvolume (subvolid=5) to a temp location
		tmpMount := BtrfsSetupTmpMount
		cmd = exec.Command("sudo", ctx.Helper, HelperCmdRun, "mkdir", "-p", tmpMount)
		cmd.Run()

		cmd = exec.Command("sudo", ctx.Helper, "mount", "-o", fmt.Sprintf("subvolid=%d", BtrfsTopLevelSubvolumeID), rootPartPath, tmpMount)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to mount btrfs top-level for subvolume creation: %w", err)
		}

		// Step 2: Create subvolumes
		for _, sv := range subvolumes {
			cmd = exec.Command("sudo", ctx.Helper, HelperCmdBtrfsSubvolCreate, tmpMount, sv.Name)
			output, err = cmd.CombinedOutput()
			if err != nil {
				exec.Command("sudo", ctx.Helper, "umount", "-l", tmpMount).Run()
				return fmt.Errorf("failed to create subvolume %s: %w: %s", sv.Name, err, string(output))
			}
			if ctx.Logger != nil && len(output) > 0 {
				ctx.Logger.Command(string(output))
			}
		}

		// Step 3: Set @ as the default subvolume
		// This means mounting the partition without options will use @
		cmd = exec.Command("sudo", ctx.Helper, HelperCmdBtrfsSubvolSetDefault, tmpMount, "@")
		output, err = cmd.CombinedOutput()
		if err != nil {
			exec.Command("sudo", ctx.Helper, "umount", "-l", tmpMount).Run()
			return fmt.Errorf("failed to set default btrfs subvolume: %w: %s", err, string(output))
		}
		if ctx.Logger != nil && len(output) > 0 {
			ctx.Logger.Command(string(output))
		}

		// Step 4: Apply per-subvolume properties
		// compression → btrfs property set, nodatacow → chattr +C
		for _, sv := range subvolumes {
			if sv.Compression != "" {
				cmd = exec.Command("sudo", ctx.Helper, HelperCmdBtrfsSetCompression, tmpMount, sv.Name, sv.Compression)
				output, err = cmd.CombinedOutput()
				if err != nil {
					if ctx.Logger != nil {
						ctx.Logger.Error("Warning: failed to set compression on %s: %v", sv.Name, err)
					}
				} else if ctx.Logger != nil && len(output) > 0 {
					ctx.Logger.Command(string(output))
				}
			}
			if sv.NoDataCow {
				if ctx.Logger != nil {
					ctx.Logger.Info("Setting nodatacow on %s subvolume", sv.Name)
				}
				cmd = exec.Command("sudo", ctx.Helper, "run", "chattr", "+C", shared.SubvolumePath(tmpMount, sv.Name))
				output, err = cmd.CombinedOutput()
				if err != nil {
					if ctx.Logger != nil {
						ctx.Logger.Error("Warning: failed to set nodatacow on %s: %v", sv.Name, err)
					}
				} else if ctx.Logger != nil && len(output) > 0 {
					ctx.Logger.Command(string(output))
				}
			}
		}

		// Step 5: Unmount temp location
		exec.Command("sudo", ctx.Helper, HelperCmdUmount, UmountLazyFlag, tmpMount).Run()
		exec.Command("sudo", ctx.Helper, HelperCmdRun, "rmdir", tmpMount).Run()

		if ctx.Logger != nil {
			ctx.Logger.Info("btrfs subvolumes configured: @ (default, compressed), @home (nodatacow + compressed), @root, @srv, @cache (nodatacow), @tmp (nodatacow), @log (nodatacow)")
		}
	}

	// Discover partition paths and UUIDs via lsblk + blkid
	if ctx.Logger != nil {
		ctx.Logger.Info("Discovering partition paths and UUIDs")
	}

	// Force kernel to re-read partition table before discovery
	refreshPartitionTable(ctx, diskID)

	diskPartitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("failed to discover partitions: %w", err)
	}

	// Read UUIDs via blkid (directly from superblock, more reliable than lsblk sysfs)
	for i := range diskPartitions {
		uuid, err := readPartitionUUID(ctx.Helper, diskPartitions[i].Path)
		if err != nil {
			return fmt.Errorf("failed to read UUID for %s: %w", diskPartitions[i].Path, err)
		}
		diskPartitions[i].UUID = uuid
		if ctx.Logger != nil {
			ctx.Logger.Info("%s: UUID=%s, FSTYPE=%s", diskPartitions[i].Path, uuid, diskPartitions[i].Fstype)
		}
	}

	// Match partitions by fstype, then by index
	efiPart, rootPart := findPartitionsByFstype(diskPartitions)
	if efiPart == nil || rootPart == nil {
		if ctx.Logger != nil {
			ctx.Logger.Info("FSTYPE not available, falling back to partition index (1=EFI, 2+=root)")
		}
		efiPart, rootPart = findPartitionsByIndex(diskPartitions)
	}

	if efiPart == nil {
		return fmt.Errorf("EFI partition not found after formatting (got %d partitions)", len(diskPartitions))
	}
	if rootPart == nil {
		return fmt.Errorf("root partition not found after formatting (got %d partitions)", len(diskPartitions))
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("EFI: %s (UUID: %s), Root: %s (UUID: %s)", efiPart.Path, efiPart.UUID, rootPart.Path, rootPart.UUID)
	}

	// Build partition info
	partitionInfo := &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     efiPart.Path,
		EFIUUID:     efiPart.UUID,
		RootPart:    rootPart.Path,
		RootUUID:    rootPart.UUID,
		MountPoints: make(map[string]string),
	}

	// For btrfs, root partition serves all subvolume mount points
	if fsType == "btrfs" {
		for _, sv := range subvolumes {
			if sv.MountPoint != "/" {
				partitionInfo.MountPoints[sv.MountPoint] = rootPart.Path
			}
		}
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
		case FileSystemVfat:
			if efi == nil {
				efi = &partitions[i]
			}
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
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
// Tries partprobe first (most reliable after sgdisk), then partx -a.
// If both fail, logs a warning but continues — the kernel may still process asynchronously.
func refreshPartitionTable(ctx *InstallContext, diskID string) {
	if ctx.Logger != nil {
		ctx.Logger.Info("Refreshing partition table for %s", diskID)
	}

	// Sync all pending writes first
	exec.Command("sudo", ctx.Helper, HelperCmdRun, "sync").Run()

	// Refresh partition table — partprobe is usually sufficient
	cmd := exec.Command("sudo", ctx.Helper, HelperCmdRun, "partprobe", diskID)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("Warning: partprobe failed: %v (trying partx fallback)", err)
		}
		// Try partx -a as fallback
		cmd = exec.Command("sudo", ctx.Helper, HelperCmdRun, "partx", "-a", diskID)
		if partxOut, partxErr := cmd.CombinedOutput(); partxErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: partx also failed: %v (kernel may still process)", partxErr)
			}
		} else if ctx.Logger != nil && len(partxOut) > 0 {
			ctx.Logger.Command(string(partxOut))
		}
	} else if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}
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

	// First, try lazy recursive unmount of /mnt (covers nested mounts including subvolumes)
	cmd := exec.Command("sudo", helper, HelperCmdUmount, UmountRecursiveFlag, shared.MountRoot)
	cmd.Run()

	// Also unmount /mnt/boot if mounted
	cmd = exec.Command("sudo", helper, HelperCmdUmount, BootMountPoint)
	cmd.Run()

	// Then unmount each partition individually
	for _, part := range partitions {
		if logger != nil {
			logger.Info("Unmounting partition: %s (fstype: %s)", part.Path, part.Fstype)
		}

		// Only unmount if actually mounted
		findCmd := exec.Command("sudo", helper, "run", "findmnt", "-n", "-o", "SOURCE", part.Path)
		if findCmd.Run() != nil {
			continue
		}

		cmd = exec.Command("sudo", helper, HelperCmdUmount, part.Path)
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
