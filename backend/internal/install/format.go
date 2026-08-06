package install

import (
	"fmt"
	"strings"

	"aestheticos-installer/backend/internal/shared"
)

// Error wrapping convention: use %w when wrapping an existing error so the
// caller can inspect it with errors.Is/As. Use plain %s/%d for new errors
// that do not wrap an underlying error (validation, missing config, etc.).

// FormatDisk partitions and formats the target disk, then stores partition info
// in ctx.Partitions. On failure, tears down any mounts left behind.
func FormatDisk(ctx *InstallContext) (retErr error) {
	diskID := ctx.DiskID
	if diskID == "" {
		return fmt.Errorf("disk ID not set. Provide a disk path (e.g., --format-disk /dev/vda)")
	}

	defer func() {
		if retErr != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("FormatDisk failed, cleaning up: %v", retErr)
			}
			_ = FormatDiskErrorCleanup(diskID, ctx.Helper, ctx.Logger)
		}
	}()

	fsType := ctx.FileSystem
	if fsType == "" {
		fsType = shared.DefaultFileSystem
	}
	if fsType != shared.FileSystemExt4 && fsType != shared.FileSystemBtrfs {
		return fmt.Errorf("unsupported filesystem type: %s (supported: ext4, btrfs)", fsType)
	}

	partitions := []struct {
		Name       string
		SizeMB     int64
		Fstype     string
		MountPoint string
	}{
		{Name: PartitionLayoutEFI, SizeMB: shared.EFIPartitionSizeMB, Fstype: FileSystemVfat, MountPoint: BootMountPoint},
		{Name: PartitionLayoutRoot, SizeMB: 0, Fstype: fsType, MountPoint: shared.MountRoot},
	}

	// Unmount anything previously mounted
	_ = ctx.run(HelperCmdUmount, "-R", shared.MountRoot).Run()

	// Clear partition table
	cmd := ctx.run("parted", diskID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("parted failed: %w: %s", err, string(out))
	}
	if ctx.Logger != nil && len(out) > 0 {
		ctx.Logger.Command(string(out))
	}

	// Create partitions
	var currentSector int64 = shared.GPTPartitionStartSector
	for i, p := range partitions {
		num := i + 1

		var args []string
		if p.SizeMB > 0 {
			args = []string{"-n", fmt.Sprintf("%d:%d:+%dm", num, currentSector, p.SizeMB)}
			currentSector += (p.SizeMB * 1024 * 1024) / 512
		} else {
			args = []string{"-n", fmt.Sprintf("%d:%d:0", num, currentSector)}
		}

		switch p.Fstype {
		case FileSystemVfat:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.ESP))
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.LinuxRootX86_64))
		default:
			args = append(args, "-t", fmt.Sprintf("%d:%s", num, shared.GPTTypeGUIDs.LinuxFilesystemGeneric))
		}
		args = append(args, "-c", fmt.Sprintf("%d:%s", num, p.Name))

		cmd = ctx.run(HelperCmdRun, "sgdisk")
		cmd.Args = append(cmd.Args, args...)
		cmd.Args = append(cmd.Args, diskID)
		out, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("sgdisk partition %d failed: %w: %s", num, err, string(out))
		}
	}

	// Format EFI
	if err := formatPartition(ctx, diskID, 1, partitions[0]); err != nil {
		return err
	}

	// Format root
	if err := formatPartition(ctx, diskID, 2, partitions[1]); err != nil {
		return err
	}

	// Refresh kernel partition table
	cmd = ctx.run(HelperCmdRun, HelperCmdPartprobe, diskID)
	probeOut, probeErr := cmd.CombinedOutput()
	if probeErr != nil {
		if ctx.Logger != nil {
			ctx.Logger.Error("partprobe failed: %v (%s)", probeErr, string(probeOut))
		}
		return fmt.Errorf("partprobe: %w", probeErr)
	}

	var subvolumes []shared.BtrfsSubvolume
	if fsType == "btrfs" {
		subvolumes = shared.GetBtrfsSubvolumes()
		if err := setupBtrfsSubvolumes(ctx, diskID, subvolumes); err != nil {
			return err
		}
	}

	// Discover UUIDs
	diskPartitions, err := shared.GetPartitions(diskID)
	if err != nil {
		return fmt.Errorf("get partitions: %w", err)
	}
	for i := range diskPartitions {
		uuid, err := readPartitionUUID(ctx, diskPartitions[i].Path)
		if err != nil {
			return fmt.Errorf("blkid %s: %w", diskPartitions[i].Path, err)
		}
		diskPartitions[i].UUID = uuid
	}

	efi, root := findPartitionsByFstype(diskPartitions)
	if efi == nil || root == nil {
		efi, root = findPartitionsByIndex(diskPartitions)
	}
	if efi == nil {
		return fmt.Errorf("EFI partition not found (got %d partitions)", len(diskPartitions))
	}
	if root == nil {
		return fmt.Errorf("root partition not found (got %d partitions)", len(diskPartitions))
	}

	ctx.Partitions = &shared.PartitionInfo{
		Disk:        diskID,
		EFIPart:     efi.Path,
		EFIUUID:     efi.UUID,
		RootPart:    root.Path,
		RootUUID:    root.UUID,
		MountPoints: make(map[string]string),
	}
	if fsType == "btrfs" {
		for _, sv := range subvolumes {
			if sv.MountPoint != "/" {
				ctx.Partitions.MountPoints[sv.MountPoint] = root.Path
			}
		}
	}

	return nil
}

func formatPartition(ctx *InstallContext, diskID string, num int, p struct {
	Name       string
	SizeMB     int64
	Fstype     string
	MountPoint string
}) error {
	path := derivePartitionPath(diskID, num)
	cmd := ctx.run(HelperCmdFormat, path, p.Fstype)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format %s failed: %w: %s", path, err, string(out))
	}
	if ctx.Logger != nil && len(out) > 0 {
		ctx.Logger.Command(string(out))
	}
	return nil
}

func setupBtrfsSubvolumes(ctx *InstallContext, diskID string, subvolumes []shared.BtrfsSubvolume) error {
	rootPath := derivePartitionPath(diskID, 2)
	tmp := BtrfsSetupTmpMount

	_ = ctx.run(HelperCmdRun, "mkdir", "-p", tmp).Run()
	cmd := ctx.run("mount", "-o", fmt.Sprintf("subvolid=%d", BtrfsTopLevelSubvolumeID), rootPath, tmp)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mount btrfs top-level: %w", err)
	}
	defer func() { _ = ctx.run(HelperCmdUmount, tmp).Run() }()

	for _, sv := range subvolumes {
		cmd = ctx.run(HelperCmdBtrfsSubvolCreate, tmp, sv.Name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("create subvolume %s: %w: %s", sv.Name, err, string(out))
		}
	}

	cmd = ctx.run(HelperCmdBtrfsSubvolSetDefault, tmp, "@")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set default subvolume: %w: %s", err, string(out))
	}
	if ctx.Logger != nil && len(out) > 0 {
		ctx.Logger.Command(string(out))
	}

	for _, sv := range subvolumes {
		if sv.Compression == "" {
			continue
		}
		cmd = ctx.run(HelperCmdBtrfsSetCompression, tmp, sv.Name, sv.Compression)
		if _, err := cmd.CombinedOutput(); err != nil {
			if ctx.Logger != nil {
				ctx.Logger.Error("Warning: compression on %s: %v", sv.Name, err)
			}
		}
	}

	_ = ctx.run(HelperCmdRun, "rmdir", tmp).Run()
	return nil
}

func derivePartitionPath(diskID string, num int) string {
	return shared.DerivePartitionPath(diskID, num)
}

func findPartitionsByFstype(parts []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range parts {
		switch parts[i].Fstype {
		case FileSystemVfat:
			if efi == nil {
				efi = &parts[i]
			}
		case shared.FileSystemExt4, shared.FileSystemBtrfs:
			if root == nil {
				root = &parts[i]
			}
		}
	}
	return efi, root
}

func findPartitionsByIndex(parts []shared.PartitionEntry) (*shared.PartitionEntry, *shared.PartitionEntry) {
	var efi, root *shared.PartitionEntry
	for i := range parts {
		switch i {
		case 0:
			efi = &parts[i]
		case 1:
			root = &parts[i]
			return efi, root
		}
	}
	return efi, root
}

func readPartitionUUID(ctx *InstallContext, partition string) (string, error) {
	cmd := ctx.run(HelperCmdRun, "blkid", "-s", "UUID", "-o", "value", partition)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("blkid %s: %w", partition, err)
	}
	return strings.TrimSpace(string(out)), nil
}
